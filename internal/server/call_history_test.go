package server

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCallHistoryLifecycleAndPersistence(t *testing.T) {
	dir := t.TempDir()
	store, err := newCallHistoryStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	call := voiceCall{ID: "7", ModemID: "0", Number: "10086", Direction: "outgoing", State: "dialing"}
	if err := store.observe(call, started); err != nil {
		t.Fatal(err)
	}
	call.State = "active"
	if err := store.observe(call, started.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	call.State = "terminated"
	if err := store.observe(call, started.Add(65*time.Second)); err != nil {
		t.Fatal(err)
	}
	entries := store.list()
	if len(entries) != 1 || entries[0].Outcome != "completed" || entries[0].ConnectedAt == "" || entries[0].EndedAt == "" {
		t.Fatalf("entries = %#v", entries)
	}
	reloaded, err := newCallHistoryStore(filepath.Dir(store.path))
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.list(); len(got) != 1 || got[0].Number != "10086" {
		t.Fatalf("reloaded = %#v", got)
	}
}

func TestCallHistoryClassifiesMissedAndClearDoesNotReimport(t *testing.T) {
	store, err := newCallHistoryStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	call := voiceCall{ID: "8", ModemID: "0", Number: "123", Direction: "incoming", State: "terminated"}
	if err := store.observe(call, now); err != nil {
		t.Fatal(err)
	}
	if got := store.list(); len(got) != 1 || got[0].Outcome != "missed" {
		t.Fatalf("history = %#v", got)
	}
	if err := store.clear(); err != nil {
		t.Fatal(err)
	}
	if err := store.observe(call, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := store.list(); len(got) != 0 {
		t.Fatalf("cleared history reimported = %#v", got)
	}
	if store.list() == nil {
		t.Fatal("empty history must encode as [] instead of null")
	}
}
