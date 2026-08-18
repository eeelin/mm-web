package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	settings      pushSettings
}

type pushSettings struct {
	ShowMessageContent bool `json:"showMessageContent"`
}

func newPushService(dir, subject string) (*pushService, error) {
	if dir == "" {
		return nil, errors.New("push data directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create push data directory: %w", err)
	}
	// webpush-go adds the mailto: scheme for non-HTTPS subjects itself.
	service := &pushService{dir: dir, subject: strings.TrimPrefix(subject, "mailto:")}
	if err := service.loadOrCreateKeys(); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "push-subscriptions.json"), &service.subscriptions); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load push subscriptions: %w", err)
	}
	if err := readJSON(filepath.Join(dir, "push-settings.json"), &service.settings); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load push settings: %w", err)
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
	endpoint, err := url.Parse(subscription.Endpoint)
	if err != nil || endpoint.Scheme != "https" || !allowedPushHost(endpoint.Hostname()) {
		return errors.New("push subscription endpoint is not trusted")
	}
	if len(subscription.Endpoint) > 2048 || len(subscription.Keys.Auth) > 256 || len(subscription.Keys.P256dh) > 256 {
		return errors.New("push subscription is too large")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for index := range p.subscriptions {
		if p.subscriptions[index].Endpoint == subscription.Endpoint {
			p.subscriptions[index] = subscription
			return p.saveSubscriptions()
		}
	}
	if len(p.subscriptions) >= 32 {
		return errors.New("push subscription limit reached")
	}
	p.subscriptions = append(p.subscriptions, subscription)
	return p.saveSubscriptions()
}

func allowedPushHost(host string) bool {
	for _, allowed := range []string{"web.push.apple.com", "fcm.googleapis.com", "updates.push.services.mozilla.com"} {
		if host == allowed {
			return true
		}
	}
	return false
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
	p.mu.Lock()
	showContent := p.settings.ShowMessageContent
	p.mu.Unlock()
	p.send(pushContent(message, showContent))
}

func (p *pushService) notifyIncomingCall(call voiceCall) {
	p.send(incomingCallPushContent(call))
}

func incomingCallPushContent(call voiceCall) map[string]string {
	return map[string]string{
		"title": "来电",
		"body":  "有新的来电",
		"url":   "/?screen=phone",
		"tag":   "call-" + call.ModemID + "-" + call.ID,
	}
}

func pushContent(message message, showContent bool) map[string]string {
	content := map[string]string{
		"title": "新信息",
		"body":  "收到一条新信息",
		"url":   "/?screen=messages",
		"tag":   "sms-" + message.ID,
	}
	if showContent {
		content["title"] = fallback(message.SenderName, fallback(message.Number, "未知号码"))
		content["body"] = message.Text
	}
	return content
}

func (p *pushService) updateSettings(settings pushSettings) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.settings = settings
	return writeJSONFile(filepath.Join(p.dir, "push-settings.json"), settings)
}

type pushReport struct {
	Subscriptions int `json:"subscriptions"`
	Delivered     int `json:"delivered"`
	Failed        int `json:"failed"`
}

func (p *pushService) send(content map[string]string) pushReport {
	payload, _ := json.Marshal(content)

	p.mu.Lock()
	subscriptions := append([]pushSubscription(nil), p.subscriptions...)
	p.mu.Unlock()
	report := pushReport{Subscriptions: len(subscriptions)}

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
			report.Failed++
			continue
		}
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		_ = response.Body.Close()
		if response.StatusCode == http.StatusGone || response.StatusCode == http.StatusNotFound {
			if err := p.unsubscribe(subscription.Endpoint); err != nil {
				log.Printf("remove expired push subscription: %v", err)
			}
		} else if response.StatusCode < 200 || response.StatusCode >= 300 {
			log.Printf("push service returned %s: %s", response.Status, responseBody)
			report.Failed++
		} else {
			report.Delivered++
		}
	}
	return report
}

func (a *api) pushConfig(w http.ResponseWriter, _ *http.Request) {
	if a.push == nil {
		writeError(w, http.StatusServiceUnavailable, "消息推送未启用", errors.New("push service unavailable"))
		return
	}
	a.push.mu.Lock()
	count := len(a.push.subscriptions)
	a.push.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": a.push.keys.Public, "supported": true, "subscriptions": count})
}

func (a *api) getPushSettings(w http.ResponseWriter, _ *http.Request) {
	if a.push == nil {
		writeError(w, http.StatusServiceUnavailable, "消息推送未启用", errors.New("push service unavailable"))
		return
	}
	a.push.mu.Lock()
	settings := a.push.settings
	a.push.mu.Unlock()
	writeJSON(w, http.StatusOK, settings)
}

func (a *api) updatePushSettings(w http.ResponseWriter, r *http.Request) {
	if a.push == nil {
		writeError(w, http.StatusServiceUnavailable, "消息推送未启用", errors.New("push service unavailable"))
		return
	}
	var settings pushSettings
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "推送设置格式无效", err)
		return
	}
	if err := a.push.updateSettings(settings); err != nil {
		writeError(w, http.StatusInternalServerError, "保存推送设置失败", err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *api) debugPush(w http.ResponseWriter, r *http.Request) {
	if a.debugPushToken == "" {
		writeError(w, http.StatusNotFound, "调试推送未启用", errors.New("MM_WEB_DEBUG_PUSH_TOKEN is not configured"))
		return
	}
	if r.Header.Get("X-Debug-Token") != a.debugPushToken {
		writeError(w, http.StatusUnauthorized, "调试令牌无效", errors.New("invalid debug token"))
		return
	}
	report := a.push.send(map[string]string{
		"title": "mmOS 推送测试",
		"body":  "如果你看到这条通知，Web Push 链路工作正常。",
		"url":   "/?screen=messages",
		"tag":   fmt.Sprintf("debug-%d", time.Now().Unix()),
	})
	if report.Subscriptions == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "没有已注册的推送设备", "report": report})
		return
	}
	writeJSON(w, http.StatusOK, report)
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
