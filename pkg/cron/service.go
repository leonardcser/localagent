package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adhocore/gronx"

	"localagent/pkg/logger"
	"localagent/pkg/utils"
)

var errorBackoffMS = []int64{30_000, 60_000, 300_000, 900_000, 3_600_000}

const maxScheduleErrors = 3

func assertSupportedJobSpec(job *CronJob) error {
	if job.SessionTarget == "main" && job.Payload.Kind != "systemEvent" {
		return fmt.Errorf("sessionTarget=\"main\" requires payload.kind=\"systemEvent\", got %q", job.Payload.Kind)
	}
	if job.SessionTarget == "isolated" && job.Payload.Kind != "agentTurn" {
		return fmt.Errorf("sessionTarget=\"isolated\" requires payload.kind=\"agentTurn\", got %q", job.Payload.Kind)
	}
	return nil
}

type CronSchedule struct {
	Kind      string `json:"kind"`
	At        string `json:"at,omitempty"`
	EveryMS   *int64 `json:"everyMs,omitempty"`
	AnchorMS  *int64 `json:"anchorMs,omitempty"`
	Expr      string `json:"expr,omitempty"`
	TZ        string `json:"tz,omitempty"`
	StaggerMS *int64 `json:"staggerMs,omitempty"`
}

type CronPayload struct {
	Kind           string `json:"kind"`
	Text           string `json:"text,omitempty"`
	Message        string `json:"message,omitempty"`
	Model          string `json:"model,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type CronDelivery struct {
	Mode       string `json:"mode"`
	Channel    string `json:"channel,omitempty"`
	To         string `json:"to,omitempty"`
	BestEffort bool   `json:"bestEffort,omitempty"`
}

type CronJobState struct {
	NextRunAtMS        *int64 `json:"nextRunAtMs,omitempty"`
	LastRunAtMS        *int64 `json:"lastRunAtMs,omitempty"`
	LastStatus         string `json:"lastStatus,omitempty"`
	LastError          string `json:"lastError,omitempty"`
	RunningAtMS        *int64 `json:"runningAtMs,omitempty"`
	LastDurationMS     *int64 `json:"lastDurationMs,omitempty"`
	ConsecutiveErrors  int    `json:"consecutiveErrors,omitempty"`
	ScheduleErrorCount int    `json:"scheduleErrorCount,omitempty"`
}

type CronJob struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Description    string        `json:"description,omitempty"`
	Enabled        bool          `json:"enabled"`
	Schedule       CronSchedule  `json:"schedule"`
	Payload        CronPayload   `json:"payload"`
	Delivery       *CronDelivery `json:"delivery,omitempty"`
	State          CronJobState  `json:"state"`
	SessionTarget  string        `json:"sessionTarget,omitempty"`
	WakeMode       string        `json:"wakeMode,omitempty"`
	CreatedAtMS    int64         `json:"createdAtMs"`
	UpdatedAtMS    int64         `json:"updatedAtMs"`
	DeleteAfterRun bool          `json:"deleteAfterRun"`
}

type CronStore struct {
	Version int       `json:"version"`
	Jobs    []CronJob `json:"jobs"`
}

type CronStatus struct {
	Running   bool   `json:"running"`
	JobCount  int    `json:"jobCount"`
	NextRunAt *int64 `json:"nextRunAt,omitempty"`
}

type JobHandler func(job *CronJob) (string, error)

type CronService struct {
	storePath string
	store     *CronStore
	onJob     JobHandler
	mu        sync.RWMutex
	running   bool
	stopChan  chan struct{}
	gronx     *gronx.Gronx
}

func NewCronService(storePath string, onJob JobHandler) *CronService {
	cs := &CronService{
		storePath: storePath,
		onJob:     onJob,
		gronx:     gronx.New(),
	}
	cs.loadStore()
	return cs
}

func (cs *CronService) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.running {
		return nil
	}

	if err := cs.loadStore(); err != nil {
		return fmt.Errorf("failed to load store: %w", err)
	}

	cs.recomputeNextRuns()
	if err := cs.saveStoreUnsafe(); err != nil {
		return fmt.Errorf("failed to save store: %w", err)
	}

	cs.stopChan = make(chan struct{})
	cs.running = true
	go cs.runLoop(cs.stopChan)

	return nil
}

func (cs *CronService) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.running {
		return
	}

	cs.running = false
	if cs.stopChan != nil {
		close(cs.stopChan)
		cs.stopChan = nil
	}
}

func (cs *CronService) runLoop(stopChan chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			cs.checkJobs()
		}
	}
}

func (cs *CronService) checkJobs() {
	cs.mu.Lock()

	if !cs.running {
		cs.mu.Unlock()
		return
	}

	now := time.Now().UnixMilli()
	var dueJobIDs []string

	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.Enabled && job.State.RunningAtMS == nil && job.State.NextRunAtMS != nil && *job.State.NextRunAtMS <= now {
			dueJobIDs = append(dueJobIDs, job.ID)
		}
	}

	dueMap := make(map[string]bool, len(dueJobIDs))
	for _, jobID := range dueJobIDs {
		dueMap[jobID] = true
	}
	for i := range cs.store.Jobs {
		if dueMap[cs.store.Jobs[i].ID] {
			cs.store.Jobs[i].State.NextRunAtMS = nil
			runningAt := now
			cs.store.Jobs[i].State.RunningAtMS = &runningAt
		}
	}

	if err := cs.saveStoreUnsafe(); err != nil {
		logger.Error("cron: failed to save store: %v", err)
	}

	cs.mu.Unlock()

	for _, jobID := range dueJobIDs {
		cs.executeJobByID(jobID)
	}
}

func (cs *CronService) executeJobByID(jobID string) {
	startTime := time.Now().UnixMilli()

	cs.mu.RLock()
	var callbackJob *CronJob
	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.ID == jobID {
			jobCopy := *job
			callbackJob = &jobCopy
			break
		}
	}
	cs.mu.RUnlock()

	if callbackJob == nil {
		return
	}

	var err error
	if cs.onJob != nil {
		_, err = cs.onJob(callbackJob)
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	var job *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			job = &cs.store.Jobs[i]
			break
		}
	}
	if job == nil {
		logger.Warn("cron: job %s disappeared before state update", jobID)
		return
	}

	endTime := time.Now().UnixMilli()
	duration := endTime - startTime
	job.State.LastRunAtMS = &startTime
	job.State.LastDurationMS = &duration
	job.State.RunningAtMS = nil
	job.UpdatedAtMS = endTime

	if err != nil {
		job.State.LastStatus = "error"
		job.State.LastError = err.Error()
		job.State.ConsecutiveErrors++

		backoffIdx := job.State.ConsecutiveErrors - 1
		if backoffIdx >= len(errorBackoffMS) {
			backoffIdx = len(errorBackoffMS) - 1
		}
		backoff := errorBackoffMS[backoffIdx]

		if job.Schedule.Kind != "at" {
			nextRun := endTime + backoff
			job.State.NextRunAtMS = &nextRun
		}
	} else {
		job.State.LastStatus = "ok"
		job.State.LastError = ""
		job.State.ConsecutiveErrors = 0
	}

	if job.Schedule.Kind == "at" {
		if job.DeleteAfterRun {
			cs.removeJobUnsafe(job.ID)
		} else {
			job.Enabled = false
			job.State.NextRunAtMS = nil
		}
	} else if err == nil {
		nextRun := cs.computeNextRun(&job.Schedule, endTime)
		job.State.NextRunAtMS = nextRun
		if nextRun == nil {
			job.State.ScheduleErrorCount++
			if job.State.ScheduleErrorCount >= maxScheduleErrors {
				job.Enabled = false
				logger.Warn("cron: job %s auto-disabled after %d schedule errors", job.ID, maxScheduleErrors)
			}
		}
	}

	if err := cs.saveStoreUnsafe(); err != nil {
		logger.Error("cron: failed to save store: %v", err)
	}
}

func (cs *CronService) computeNextRun(schedule *CronSchedule, nowMS int64) *int64 {
	if schedule.Kind == "at" {
		if schedule.At != "" {
			t, err := time.Parse(time.RFC3339, schedule.At)
			if err != nil {
				logger.Error("cron: failed to parse 'at' timestamp '%s': %v", schedule.At, err)
				return nil
			}
			ms := t.UnixMilli()
			if ms > nowMS {
				return &ms
			}
		}
		return nil
	}

	if schedule.Kind == "every" {
		if schedule.EveryMS == nil || *schedule.EveryMS <= 0 {
			return nil
		}
		var next int64
		if schedule.AnchorMS != nil {
			anchor := *schedule.AnchorMS
			interval := *schedule.EveryMS
			if anchor > nowMS {
				next = anchor
			} else {
				elapsed := nowMS - anchor
				periods := elapsed / interval
				next = anchor + (periods+1)*interval
			}
		} else {
			next = nowMS + *schedule.EveryMS
		}
		return &next
	}

	if schedule.Kind == "cron" {
		if schedule.Expr == "" {
			return nil
		}

		now := time.UnixMilli(nowMS)
		if schedule.TZ != "" {
			loc, err := time.LoadLocation(schedule.TZ)
			if err == nil {
				now = now.In(loc)
			}
		}

		nextTime, err := gronx.NextTickAfter(schedule.Expr, now, false)
		if err != nil {
			logger.Error("cron: failed to compute next run for expr '%s': %v", schedule.Expr, err)
			return nil
		}

		nextMS := nextTime.UnixMilli()
		if schedule.StaggerMS != nil && *schedule.StaggerMS > 0 {
			nextMS += *schedule.StaggerMS
		}
		return &nextMS
	}

	return nil
}

func (cs *CronService) recomputeNextRuns() {
	now := time.Now().UnixMilli()
	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.Enabled {
			job.State.RunningAtMS = nil
			job.State.NextRunAtMS = cs.computeNextRun(&job.Schedule, now)
		}
	}
}

func (cs *CronService) Load() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.loadStore()
}

func (cs *CronService) SetOnJob(handler JobHandler) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.onJob = handler
}

func (cs *CronService) loadStore() error {
	cs.store = &CronStore{
		Version: 1,
		Jobs:    []CronJob{},
	}

	data, err := os.ReadFile(cs.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, cs.store)
}

func (cs *CronService) saveStoreUnsafe() error {
	dir := filepath.Dir(cs.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cs.store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cs.storePath, data, 0644)
}

func (cs *CronService) AddJob(job CronJob) (*CronJob, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now().UnixMilli()

	if job.ID == "" {
		job.ID = utils.RandHex(8)
	}
	if err := assertSupportedJobSpec(&job); err != nil {
		return nil, err
	}
	if job.Schedule.Kind == "at" {
		job.DeleteAfterRun = true
	}
	job.Enabled = true
	job.CreatedAtMS = now
	job.UpdatedAtMS = now
	job.State.NextRunAtMS = cs.computeNextRun(&job.Schedule, now)

	cs.store.Jobs = append(cs.store.Jobs, job)
	if err := cs.saveStoreUnsafe(); err != nil {
		return nil, err
	}

	return &cs.store.Jobs[len(cs.store.Jobs)-1], nil
}

func (cs *CronService) PatchJob(jobID string, patch map[string]any) (*CronJob, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var job *CronJob
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			job = &cs.store.Jobs[i]
			break
		}
	}
	if job == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	if name, ok := patch["name"].(string); ok {
		job.Name = name
	}
	if desc, ok := patch["description"].(string); ok {
		job.Description = desc
	}
	if enabled, ok := patch["enabled"].(bool); ok {
		job.Enabled = enabled
		if enabled {
			job.State.NextRunAtMS = cs.computeNextRun(&job.Schedule, time.Now().UnixMilli())
			job.State.ConsecutiveErrors = 0
			job.State.ScheduleErrorCount = 0
		} else {
			job.State.NextRunAtMS = nil
		}
	}
	if sessionTarget, ok := patch["sessionTarget"].(string); ok {
		job.SessionTarget = sessionTarget
	}
	if wakeMode, ok := patch["wakeMode"].(string); ok {
		job.WakeMode = wakeMode
	}

	if scheduleRaw, ok := patch["schedule"]; ok {
		if schedMap, ok := scheduleRaw.(map[string]any); ok {
			data, _ := json.Marshal(schedMap)
			var sched CronSchedule
			if err := json.Unmarshal(data, &sched); err == nil {
				job.Schedule = sched
				job.State.NextRunAtMS = cs.computeNextRun(&sched, time.Now().UnixMilli())
			}
		}
	}
	if payloadRaw, ok := patch["payload"]; ok {
		if payMap, ok := payloadRaw.(map[string]any); ok {
			data, _ := json.Marshal(payMap)
			var payload CronPayload
			if err := json.Unmarshal(data, &payload); err == nil {
				job.Payload = payload
			}
		}
	}
	if deliveryRaw, ok := patch["delivery"]; ok {
		if delMap, ok := deliveryRaw.(map[string]any); ok {
			data, _ := json.Marshal(delMap)
			var delivery CronDelivery
			if err := json.Unmarshal(data, &delivery); err == nil {
				job.Delivery = &delivery
			}
		}
	}

	if err := assertSupportedJobSpec(job); err != nil {
		return nil, err
	}

	job.UpdatedAtMS = time.Now().UnixMilli()
	if err := cs.saveStoreUnsafe(); err != nil {
		return nil, err
	}

	return job, nil
}

func (cs *CronService) RemoveJob(jobID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.removeJobUnsafe(jobID)
}

func (cs *CronService) removeJobUnsafe(jobID string) bool {
	before := len(cs.store.Jobs)
	var jobs []CronJob
	for _, job := range cs.store.Jobs {
		if job.ID != jobID {
			jobs = append(jobs, job)
		}
	}
	cs.store.Jobs = jobs
	removed := len(cs.store.Jobs) < before

	if removed {
		if err := cs.saveStoreUnsafe(); err != nil {
			logger.Error("cron: failed to save store after remove: %v", err)
		}
	}

	return removed
}

func (cs *CronService) RunJob(jobID string, force bool) error {
	cs.mu.RLock()
	var found bool
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			found = true
			if !force && (cs.store.Jobs[i].State.NextRunAtMS == nil || *cs.store.Jobs[i].State.NextRunAtMS > time.Now().UnixMilli()) {
				cs.mu.RUnlock()
				// force=false means only run if due; trigger it anyway
			}
			break
		}
	}
	cs.mu.RUnlock()

	if !found {
		return fmt.Errorf("job not found: %s", jobID)
	}

	go cs.executeJobByID(jobID)
	return nil
}

func (cs *CronService) ListJobs(includeDisabled bool) []CronJob {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if includeDisabled {
		result := make([]CronJob, len(cs.store.Jobs))
		copy(result, cs.store.Jobs)
		return result
	}

	var enabled []CronJob
	for _, job := range cs.store.Jobs {
		if job.Enabled {
			enabled = append(enabled, job)
		}
	}

	return enabled
}

func (cs *CronService) Status() CronStatus {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	status := CronStatus{
		Running:  cs.running,
		JobCount: len(cs.store.Jobs),
	}

	var earliest *int64
	for _, job := range cs.store.Jobs {
		if job.Enabled && job.State.NextRunAtMS != nil {
			if earliest == nil || *job.State.NextRunAtMS < *earliest {
				ms := *job.State.NextRunAtMS
				earliest = &ms
			}
		}
	}
	status.NextRunAt = earliest

	return status
}
