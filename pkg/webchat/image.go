package webchat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"localagent/pkg/config"
	"localagent/pkg/logger"
	"localagent/pkg/utils"

	"github.com/labstack/echo/v5"
)

type ImageJob struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Model          string    `json:"model"`
	Prompt         string    `json:"prompt"`
	NegativePrompt string    `json:"negative_prompt,omitempty"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	Seed           *int      `json:"seed,omitempty"`
	Steps          *int      `json:"steps,omitempty"`
	GuidanceScale  *float64  `json:"guidance_scale,omitempty"`
	Count          int       `json:"count"`
	SourceImages   int       `json:"source_images,omitempty"`
	Status         string    `json:"status"`
	ImageCount     int       `json:"image_count"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type imageJobEntry struct {
	job *ImageJob
	cfg config.ImageConfig
}

type ImageJobStore struct {
	mu      sync.RWMutex
	jobs    map[string]*ImageJob
	order   []string
	baseDir string
	queue   chan imageJobEntry
	done    chan struct{}
}

func NewImageJobStore(baseDir string) *ImageJobStore {
	s := &ImageJobStore{
		jobs:    make(map[string]*ImageJob),
		baseDir: baseDir,
		queue:   make(chan imageJobEntry, 16),
		done:    make(chan struct{}),
	}
	s.load()
	go s.worker()
	return s
}

func (s *ImageJobStore) worker() {
	defer close(s.done)
	for entry := range s.queue {
		s.processJob(entry.job, entry.cfg)
	}
}

func (s *ImageJobStore) Stop() {
	close(s.queue)
	<-s.done
}

func (s *ImageJobStore) Enqueue(job *ImageJob, cfg config.ImageConfig) {
	s.queue <- imageJobEntry{job: job, cfg: cfg}
}

func (s *ImageJobStore) processJob(job *ImageJob, cfg config.ImageConfig) {
	if s.Get(job.ID) == nil {
		return
	}

	job.Status = "generating"
	s.Update(job)

	var endpoint string
	switch job.Type {
	case "edit":
		endpoint = cfg.URL + "/edit"
	case "upscale":
		endpoint = cfg.URL + "/upscale"
	default:
		endpoint = cfg.URL + "/generate"
	}

	var resp *http.Response
	var err error

	switch job.Type {
	case "edit":
		resp, err = s.doEditRequest(job, cfg, endpoint)
	case "upscale":
		resp, err = s.doUpscaleRequest(job, cfg, endpoint)
	default:
		resp, err = s.doGenerateRequest(job, cfg, endpoint)
	}

	if err != nil {
		job.Status = "error"
		job.Error = fmt.Sprintf("request failed: %v", err)
		s.Update(job)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		job.Status = "error"
		job.Error = fmt.Sprintf("remote returned %d: %s", resp.StatusCode, string(respBody))
		s.Update(job)
		return
	}

	var genResp remoteGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		job.Status = "error"
		job.Error = fmt.Sprintf("invalid response: %v", err)
		s.Update(job)
		return
	}

	imageCount := 0
	for i, b64 := range genResp.Images {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			logger.Warn("image decode error for job %s: %v", job.ID, err)
			continue
		}
		s.saveImage(job.ID, i, data)
		imageCount++
	}

	job.ImageCount = imageCount
	if genResp.Width > 0 && genResp.Height > 0 {
		job.Width = genResp.Width
		job.Height = genResp.Height
	}
	job.Status = "done"
	s.Update(job)
}

func (s *ImageJobStore) doGenerateRequest(job *ImageJob, cfg config.ImageConfig, url string) (*http.Response, error) {
	remoteReq := remoteGenerateRequest{
		Model:          job.Model,
		Prompt:         job.Prompt,
		NegativePrompt: job.NegativePrompt,
		Width:          job.Width,
		Height:         job.Height,
		Seed:           job.Seed,
		Steps:          job.Steps,
		GuidanceScale:  job.GuidanceScale,
		Count:          job.Count,
	}
	body, err := json.Marshal(remoteReq)
	if err != nil {
		return nil, err
	}
	return imageHTTPRequest("POST", url, cfg, "application/json", bytes.NewReader(body))
}

func (s *ImageJobStore) doEditRequest(job *ImageJob, cfg config.ImageConfig, url string) (*http.Response, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for i := 0; i < job.SourceImages; i++ {
		srcPath := s.sourcePath(job.ID, i)
		f, err := os.Open(srcPath)
		if err != nil {
			return nil, fmt.Errorf("open source image %d: %w", i, err)
		}
		part, err := w.CreateFormFile("images", fmt.Sprintf("source_%d.png", i))
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			f.Close()
			return nil, fmt.Errorf("copy source image: %w", err)
		}
		f.Close()
	}

	w.WriteField("model", job.Model)
	w.WriteField("prompt", job.Prompt)
	if job.NegativePrompt != "" {
		w.WriteField("negative_prompt", job.NegativePrompt)
	}
	if job.Seed != nil {
		w.WriteField("seed", strconv.Itoa(*job.Seed))
	}
	if job.Steps != nil {
		w.WriteField("steps", strconv.Itoa(*job.Steps))
	}
	if job.GuidanceScale != nil {
		w.WriteField("guidance_scale", strconv.FormatFloat(*job.GuidanceScale, 'f', -1, 64))
	}
	if job.Count > 0 {
		w.WriteField("count", strconv.Itoa(job.Count))
	}
	w.Close()

	return imageHTTPRequest("POST", url, cfg, w.FormDataContentType(), &buf)
}

func (s *ImageJobStore) doUpscaleRequest(job *ImageJob, cfg config.ImageConfig, url string) (*http.Response, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for i := 0; i < job.SourceImages; i++ {
		srcPath := s.sourcePath(job.ID, i)
		f, err := os.Open(srcPath)
		if err != nil {
			return nil, fmt.Errorf("open source image %d: %w", i, err)
		}
		part, err := w.CreateFormFile("images", fmt.Sprintf("source_%d.png", i))
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			f.Close()
			return nil, fmt.Errorf("copy source image: %w", err)
		}
		f.Close()
	}

	w.WriteField("model", job.Model)
	w.Close()

	return imageHTTPRequest("POST", url, cfg, w.FormDataContentType(), &buf)
}

func (s *ImageJobStore) load() {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return
	}

	type jobWithTime struct {
		job *ImageJob
		t   time.Time
	}
	var loaded []jobWithTime

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.baseDir, entry.Name(), "job.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var job ImageJob
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}
		loaded = append(loaded, jobWithTime{job: &job, t: job.CreatedAt})
	}

	sort.Slice(loaded, func(i, j int) bool {
		return loaded[i].t.Before(loaded[j].t)
	})

	for _, l := range loaded {
		if l.job.Status == "generating" || l.job.Status == "pending" {
			l.job.Status = "error"
			l.job.Error = "interrupted by restart"
			s.saveJob(l.job)
		}
		s.jobs[l.job.ID] = l.job
		s.order = append(s.order, l.job.ID)
	}

	if len(loaded) > 0 {
		logger.Info("loaded %d image jobs from disk", len(loaded))
	}
}

func (s *ImageJobStore) jobDir(id string) string {
	return filepath.Join(s.baseDir, id)
}

func (s *ImageJobStore) saveJob(job *ImageJob) {
	dir := s.jobDir(job.ID)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		logger.Error("failed to marshal job %s: %v", job.ID, err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "job.json"), data, 0644); err != nil {
		logger.Error("failed to save job %s: %v", job.ID, err)
	}
}

func (s *ImageJobStore) saveImage(jobID string, index int, data []byte) {
	dir := s.jobDir(jobID)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, fmt.Sprintf("%d.png", index))
	if err := os.WriteFile(path, data, 0644); err != nil {
		logger.Error("failed to save image %s/%d: %v", jobID, index, err)
	}
}

func (s *ImageJobStore) imagePath(jobID string, index int) string {
	return filepath.Join(s.jobDir(jobID), fmt.Sprintf("%d.png", index))
}

func (s *ImageJobStore) sourcePath(jobID string, index int) string {
	return filepath.Join(s.jobDir(jobID), fmt.Sprintf("source_%d.png", index))
}

func (s *ImageJobStore) saveSource(jobID string, index int, data []byte) {
	dir := s.jobDir(jobID)
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, fmt.Sprintf("source_%d.png", index))
	if err := os.WriteFile(path, data, 0644); err != nil {
		logger.Error("failed to save source image %s/%d: %v", jobID, index, err)
	}
}

func (s *ImageJobStore) Create(job *ImageJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	s.order = append(s.order, job.ID)
	s.saveJob(job)
}

func (s *ImageJobStore) Update(job *ImageJob) {
	s.saveJob(job)
}

func (s *ImageJobStore) Get(id string) *ImageJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

func (s *ImageJobStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[id]; !ok {
		return false
	}
	delete(s.jobs, id)
	for i, oid := range s.order {
		if oid == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	os.RemoveAll(s.jobDir(id))
	return true
}

func (s *ImageJobStore) All() []*ImageJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*ImageJob, 0, len(s.order))
	for _, id := range s.order {
		result = append(result, s.jobs[id])
	}
	return result
}

type generateRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	NegativePrompt string   `json:"negative_prompt"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Seed           *int     `json:"seed"`
	Steps          *int     `json:"steps"`
	GuidanceScale  *float64 `json:"guidance_scale"`
	Count          int      `json:"count"`
}

type remoteGenerateRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	Width          int      `json:"width,omitempty"`
	Height         int      `json:"height,omitempty"`
	Seed           *int     `json:"seed,omitempty"`
	Steps          *int     `json:"steps,omitempty"`
	GuidanceScale  *float64 `json:"guidance_scale,omitempty"`
	Count          int      `json:"count,omitempty"`
}

type remoteHealthResponse struct {
	Status         string   `json:"status"`
	GenerateModels []string `json:"generate_models"`
	EditModels     []string `json:"edit_models"`
	UpscaleModels  []string `json:"upscale_models"`
}

type remoteGenerateResponse struct {
	Images []string `json:"images"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
}

func imageHTTPRequest(method, url string, cfg config.ImageConfig, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if apiKey := cfg.ResolveAPIKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return http.DefaultClient.Do(req)
}

func (s *Server) handleImageModels(c *echo.Context) error {
	cfg := s.channel.image
	if cfg.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "image service not configured"})
	}

	resp, err := imageHTTPRequest("GET", cfg.URL+"/health", cfg, "", nil)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "image service unreachable"})
	}
	defer resp.Body.Close()

	var health remoteHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "invalid response from image service"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"generate": health.GenerateModels,
		"edit":     health.EditModels,
		"upscale":  health.UpscaleModels,
	})
}

func (s *Server) handleImageGenerate(c *echo.Context) error {
	imageConfig := s.channel.image
	if imageConfig.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "image service not configured"})
	}

	var req generateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}
	if req.Model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
	}
	if req.Count < 1 {
		req.Count = 1
	}
	if req.Count > 4 {
		req.Count = 4
	}

	job := &ImageJob{
		ID:             utils.RandHex(8),
		Type:           "generate",
		Model:          req.Model,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		Width:          req.Width,
		Height:         req.Height,
		Seed:           req.Seed,
		Steps:          req.Steps,
		GuidanceScale:  req.GuidanceScale,
		Count:          req.Count,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	s.imageJobs.Create(job)
	s.imageJobs.Enqueue(job, imageConfig)

	return c.JSON(http.StatusOK, map[string]string{"id": job.ID})
}

func (s *Server) handleImageJobs(c *echo.Context) error {
	jobs := s.imageJobs.All()
	return c.JSON(http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) handleImageJob(c *echo.Context) error {
	id := c.Param("id")
	job := s.imageJobs.Get(id)
	if job == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}
	return c.JSON(http.StatusOK, job)
}

func (s *Server) handleImageDelete(c *echo.Context) error {
	id := c.Param("id")
	if !s.imageJobs.Delete(id) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleImageResultDelete(c *echo.Context) error {
	id := c.Param("id")
	indexStr := c.Param("index")

	job := s.imageJobs.Get(id)
	if job == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 || index >= job.ImageCount {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "image not found"})
	}

	path := s.imageJobs.imagePath(id, index)
	os.Remove(path)

	// Shift remaining images down
	for i := index + 1; i < job.ImageCount; i++ {
		oldPath := s.imageJobs.imagePath(id, i)
		newPath := s.imageJobs.imagePath(id, i-1)
		os.Rename(oldPath, newPath)
	}

	job.ImageCount--
	if job.ImageCount == 0 {
		s.imageJobs.Delete(id)
	} else {
		s.imageJobs.Update(job)
	}

	return c.JSON(http.StatusOK, map[string]any{"ok": true, "image_count": job.ImageCount})
}

func (s *Server) handleImageResult(c *echo.Context) error {
	id := c.Param("id")
	indexStr := c.Param("index")

	job := s.imageJobs.Get(id)
	if job == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 || index >= job.ImageCount {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "image not found"})
	}

	path := s.imageJobs.imagePath(id, index)
	return c.File(path)
}

func (s *Server) handleImageEdit(c *echo.Context) error {
	imageConfig := s.channel.image
	if imageConfig.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "image service not configured"})
	}

	prompt := c.FormValue("prompt")
	if prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}
	model := c.FormValue("model")
	if model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
	}

	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "multipart form required"})
	}

	files := c.Request().MultipartForm.File["images[]"]
	if len(files) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "at least one source image is required"})
	}

	countVal := 1
	if v := c.FormValue("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 4 {
			countVal = n
		}
	}

	var seedVal *int
	if v := c.FormValue("seed"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			seedVal = &n
		}
	}

	var stepsVal *int
	if v := c.FormValue("steps"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			stepsVal = &n
		}
	}

	var guidanceVal *float64
	if v := c.FormValue("guidance_scale"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			guidanceVal = &n
		}
	}

	job := &ImageJob{
		ID:             utils.RandHex(8),
		Type:           "edit",
		Model:          model,
		Prompt:         prompt,
		NegativePrompt: c.FormValue("negative_prompt"),
		Seed:           seedVal,
		Steps:          stepsVal,
		GuidanceScale:  guidanceVal,
		Count:          countVal,
		SourceImages:   len(files),
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	s.imageJobs.Create(job)

	for i, fh := range files {
		src, err := fh.Open()
		if err != nil {
			s.imageJobs.Delete(job.ID)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read uploaded file"})
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			s.imageJobs.Delete(job.ID)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read uploaded file"})
		}
		s.imageJobs.saveSource(job.ID, i, data)
	}

	s.imageJobs.Enqueue(job, imageConfig)
	return c.JSON(http.StatusOK, map[string]string{"id": job.ID})
}

func (s *Server) handleImageSource(c *echo.Context) error {
	id := c.Param("id")
	indexStr := c.Param("index")

	job := s.imageJobs.Get(id)
	if job == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 || index >= job.SourceImages {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "source image not found"})
	}

	path := s.imageJobs.sourcePath(id, index)
	return c.File(path)
}

func (s *Server) handleImageUpscale(c *echo.Context) error {
	imageConfig := s.channel.image
	if imageConfig.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "image service not configured"})
	}

	model := c.FormValue("model")
	if model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
	}

	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "multipart form required"})
	}

	files := c.Request().MultipartForm.File["images[]"]
	if len(files) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "at least one source image is required"})
	}

	job := &ImageJob{
		ID:           utils.RandHex(8),
		Type:         "upscale",
		Model:        model,
		Count:        len(files),
		SourceImages: len(files),
		Status:       "pending",
		CreatedAt:    time.Now(),
	}

	s.imageJobs.Create(job)

	for i, fh := range files {
		src, err := fh.Open()
		if err != nil {
			s.imageJobs.Delete(job.ID)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read uploaded file"})
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			s.imageJobs.Delete(job.ID)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read uploaded file"})
		}
		s.imageJobs.saveSource(job.ID, i, data)
	}

	s.imageJobs.Enqueue(job, imageConfig)
	return c.JSON(http.StatusOK, map[string]string{"id": job.ID})
}
