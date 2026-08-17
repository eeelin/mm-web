package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		Auth   string `json:"auth"`
		P256dh string `json:"p256dh"`
	} `json:"keys"`
}

type vapidKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

type pushService struct {
	mu            sync.Mutex
	dir           string
	subject       string
	keys          vapidKeys
	subscriptions []pushSubscription
}

func newPushService(dir, subject string) (*pushService, error) {
	if dir == "" {
		return nil, errors.New("push data directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create push data directory: %w", err)
	}
	service := &pushService{dir: dir, subject: subject}
	if err := service.loadOrCreateKeys(); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "push-subscriptions.json"), &service.subscriptions); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load push subscriptions: %w", err)
	}
	return service, nil
}

func (p *pushService) loadOrCreateKeys() error {
	path := filepath.Join(p.dir, "vapid.json")
	if err := readJSON(path, &p.keys); err == nil {
		if p.keys.Public == "" || p.keys.Private == "" {
			return errors.New("stored VAPID keys are incomplete")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load VAPID keys: %w", err)
	}
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("generate VAPID keys: %w", err)
	}
	p.keys = vapidKeys{Public: publicKey, Private: privateKey}
	return writeJSONFile(path, p.keys)
}

func (p *pushService) saveSubscriptions() error {
	return writeJSONFile(filepath.Join(p.dir, "push-subscriptions.json"), p.subscriptions)
}

func (p *pushService) subscribe(subscription pushSubscription) error {
	if subscription.Endpoint == "" || subscription.Keys.Auth == "" || subscription.Keys.P256dh == "" {
		return errors.New("push subscription is incomplete")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for index := range p.subscriptions {
		if p.subscriptions[index].Endpoint == subscription.Endpoint {
			p.subscriptions[index] = subscription
			return p.saveSubscriptions()
		}
	}
	p.subscriptions = append(p.subscriptions, subscription)
	return p.saveSubscriptions()
}

func (p *pushService) unsubscribe(endpoint string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	filtered := p.subscriptions[:0]
	for _, subscription := range p.subscriptions {
		if subscription.Endpoint != endpoint {
			filtered = append(filtered, subscription)
		}
	}
	p.subscriptions = filtered
	return p.saveSubscriptions()
}

func (p *pushService) notify(message message) {
	payload, _ := json.Marshal(map[string]string{
		"title": "新信息",
		"body":  "收到一条新信息",
		"url":   "/?screen=messages",
		"tag":   "sms-" + message.ID,
	})

	p.mu.Lock()
	subscriptions := append([]pushSubscription(nil), p.subscriptions...)
	p.mu.Unlock()

	for _, subscription := range subscriptions {
		response, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: subscription.Endpoint,
			Keys:     webpush.Keys{Auth: subscription.Keys.Auth, P256dh: subscription.Keys.P256dh},
		}, &webpush.Options{
			Subscriber: p.subject, VAPIDPublicKey: p.keys.Public, VAPIDPrivateKey: p.keys.Private,
			TTL: 60, Urgency: webpush.UrgencyHigh,
		})
		if err != nil {
			log.Printf("send push notification: %v", err)
			continue
		}
		_ = response.Body.Close()
		if response.StatusCode == http.StatusGone || response.StatusCode == http.StatusNotFound {
			if err := p.unsubscribe(subscription.Endpoint); err != nil {
				log.Printf("remove expired push subscription: %v", err)
			}
		} else if response.StatusCode < 200 || response.StatusCode >= 300 {
			log.Printf("push service returned %s", response.Status)
		}
	}
}

func (a *api) pushConfig(w http.ResponseWriter, _ *http.Request) {
	if a.push == nil {
		writeError(w, http.StatusServiceUnavailable, "消息推送未启用", errors.New("push service unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": a.push.keys.Public, "supported": true})
}

func (a *api) subscribePush(w http.ResponseWriter, r *http.Request) {
	if a.push == nil {
		writeError(w, http.StatusServiceUnavailable, "消息推送未启用", errors.New("push service unavailable"))
		return
	}
	var subscription pushSubscription
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&subscription); err != nil {
		writeError(w, http.StatusBadRequest, "推送订阅格式无效", err)
		return
	}
	if err := a.push.subscribe(subscription); err != nil {
		writeError(w, http.StatusBadRequest, "保存推送订阅失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"subscribed": true})
}

func (a *api) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	if a.push == nil {
		writeError(w, http.StatusServiceUnavailable, "消息推送未启用", errors.New("push service unavailable"))
		return
	}
	var input struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil || input.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "推送订阅格式无效", err)
		return
	}
	if err := a.push.unsubscribe(input.Endpoint); err != nil {
		writeError(w, http.StatusInternalServerError, "删除推送订阅失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) watchIncomingMessages(ctx context.Context) {
	seen := make(map[string]string)
	initialized := false
	check := func() {
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		objects, err := a.managedObjects(requestCtx)
		if err != nil {
			log.Printf("watch incoming messages: %v", err)
			return
		}
		messages, err := a.collectMessages(requestCtx, objects)
		if err != nil {
			log.Printf("watch incoming message contents: %v", err)
			return
		}
		for _, current := range messages {
			key := current.ModemID + ":" + current.ID
			previous, exists := seen[key]
			if initialized && current.Direction == "received" && current.State == "received" && (!exists || previous != "received") {
				go a.push.notify(current)
			}
			seen[key] = current.State
		}
		initialized = true
	}
	check()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, destination)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
