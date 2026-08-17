package server

import (
	"encoding/json"
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
	var subscription pushSubscription
	subscription.Endpoint = "https://push.example/subscription"
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

func TestPushSubscriptionJSONMatchesBrowserShape(t *testing.T) {
	var subscription pushSubscription
	if err := json.Unmarshal([]byte(`{"endpoint":"https://push.example/id","keys":{"auth":"a","p256dh":"p"}}`), &subscription); err != nil {
		t.Fatal(err)
	}
	if subscription.Endpoint == "" || subscription.Keys.Auth != "a" || subscription.Keys.P256dh != "p" {
		t.Fatalf("subscription = %#v", subscription)
	}
}
