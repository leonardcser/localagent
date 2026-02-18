package webchat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"localagent/pkg/logger"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type vapidKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

type PushManager struct {
	dir           string
	vapid         vapidKeys
	subscriptions []webpush.Subscription
	mu            sync.RWMutex
}

func NewPushManager(webchatDir string) (*PushManager, error) {
	dir := filepath.Join(webchatDir, "push")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create push dir: %w", err)
	}

	pm := &PushManager{dir: dir}

	if err := pm.loadVAPID(); err != nil {
		return nil, fmt.Errorf("load vapid keys: %w", err)
	}

	pm.loadSubscriptions()
	return pm, nil
}

func (pm *PushManager) VAPIDPublicKey() string {
	return pm.vapid.Public
}

func (pm *PushManager) AddSubscription(sub webpush.Subscription) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, existing := range pm.subscriptions {
		if existing.Endpoint == sub.Endpoint {
			return nil
		}
	}

	pm.subscriptions = append(pm.subscriptions, sub)
	return pm.saveSubscriptions()
}

func (pm *PushManager) SendPush(title, body, url string) {
	pm.mu.RLock()
	subs := make([]webpush.Subscription, len(pm.subscriptions))
	copy(subs, pm.subscriptions)
	pm.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   url,
	})

	var expired []int
	for i, sub := range subs {
		resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
			VAPIDPublicKey:  pm.vapid.Public,
			VAPIDPrivateKey: pm.vapid.Private,
			Subscriber:      "localagent@example.com",
			TTL:             60,
			Urgency:         webpush.UrgencyNormal,
		})
		if err != nil {
			logger.Warn("push: send failed for %s: %v", sub.Endpoint, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone {
			expired = append(expired, i)
		}
	}

	if len(expired) > 0 {
		pm.removeExpired(expired)
	}
}

func (pm *PushManager) removeExpired(indices []int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	removed := make(map[int]bool, len(indices))
	for _, i := range indices {
		removed[i] = true
	}

	filtered := pm.subscriptions[:0]
	for i, sub := range pm.subscriptions {
		if !removed[i] {
			filtered = append(filtered, sub)
		}
	}
	pm.subscriptions = filtered

	if err := pm.saveSubscriptions(); err != nil {
		logger.Error("push save subscriptions after cleanup: %v", err)
	}
}

func (pm *PushManager) loadVAPID() error {
	path := filepath.Join(pm.dir, "vapid.json")
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &pm.vapid); err == nil && pm.vapid.Public != "" && pm.vapid.Private != "" {
			return nil
		}
	}

	privKey, pubKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("generate VAPID keys: %w", err)
	}

	pm.vapid.Private = privKey
	pm.vapid.Public = pubKey

	out, err := json.MarshalIndent(pm.vapid, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, out)
}

func (pm *PushManager) loadSubscriptions() {
	path := filepath.Join(pm.dir, "subscriptions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &pm.subscriptions)
}

func (pm *PushManager) saveSubscriptions() error {
	path := filepath.Join(pm.dir, "subscriptions.json")
	data, err := json.MarshalIndent(pm.subscriptions, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
