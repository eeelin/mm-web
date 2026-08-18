package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPushServicePersistsKeysAndSubscriptions(t *testing.T) {
	dir := t.TempDir()
	first, err := newPushService(dir, "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if first.subject != "test@example.com" {
		t.Fatalf("normalized VAPID subject = %q", first.subject)
	}
	var subscription pushSubscription
	subscription.Endpoint = "https://web.push.apple.com/subscription"
	subscription.Keys.Auth = "auth"
	subscription.Keys.P256dh = "p256dh"
	if err := first.subscribe(subscription); err != nil {
		t.Fatal(err)
	}

	second, err := newPushService(dir, "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if second.keys != first.keys {
		t.Fatal("VAPID keys changed across restarts")
	}
	if len(second.subscriptions) != 1 || second.subscriptions[0].Endpoint != subscription.Endpoint {
		t.Fatalf("subscriptions = %#v", second.subscriptions)
	}
	info, err := os.Stat(filepath.Join(dir, "vapid.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("VAPID key permissions = %v", info.Mode().Perm())
	}
}

func TestPushServiceRejectsUntrustedEndpoint(t *testing.T) {
	service, err := newPushService(t.TempDir(), "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	var subscription pushSubscription
	subscription.Endpoint = "https://127.0.0.1/internal"
	subscription.Keys.Auth = "auth"
	subscription.Keys.P256dh = "p256dh"
	if err := service.subscribe(subscription); err == nil {
		t.Fatal("untrusted push endpoint was accepted")
	}
}

func TestPushSettingsPersistAndControlMessagePreview(t *testing.T) {
	dir := t.TempDir()
	service, err := newPushService(dir, "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.updateSettings(pushSettings{ShowMessageContent: true}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newPushService(dir, "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.settings.ShowMessageContent {
		t.Fatal("message preview setting was not persisted")
	}

	message := message{ID: "7", Number: "13800138000", Text: "验证码 1234"}
	private := pushContent(message, false)
	if private["title"] != "新信息" || private["body"] != "收到一条新信息" {
		t.Fatalf("private push = %#v", private)
	}
	preview := pushContent(message, true)
	if preview["title"] != message.Number || preview["body"] != message.Text {
		t.Fatalf("preview push = %#v", preview)
	}
	message.SenderName = "Alice"
	if named := pushContent(message, true); named["title"] != "Alice" {
		t.Fatalf("named preview = %#v", named)
	}
}

func TestDebugPushReportsMissingSubscription(t *testing.T) {
	service, err := newPushService(t.TempDir(), "mailto:test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	a := &api{push: service, debugPushToken: "secret"}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/debug/push", nil)
	request.Header.Set("X-Debug-Token", "secret")
	a.debugPush(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("debugPush status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestPushSubscriptionJSONMatchesBrowserShape(t *testing.T) {
	var subscription pushSubscription
	if err := json.Unmarshal([]byte(`{"endpoint":"https://push.example/id","keys":{"auth":"a","p256dh":"p"}}`), &subscription); err != nil {
		t.Fatal(err)
	}
	if subscription.Endpoint == "" || subscription.Keys.Auth != "a" || subscription.Keys.P256dh != "p" {
		t.Fatalf("subscription = %#v", subscription)
	}
}
