package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testMessage(id, state string) message {
	return message{
		ID: id, ModemMessageID: id, ModemID: "0", Number: "10086", Text: "余额充足",
		Direction: "received", State: state, Timestamp: "2026-09-20T08:00:00+08:00",
	}
}

func TestMessageStorePersistsMessagesAndKeepsDistinctModemIDs(t *testing.T) {
	dir := t.TempDir()
	store, err := newMessageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := testMessage("7", "receiving")
	second := testMessage("8", "received")
	if err := store.upsert(first); err != nil {
		t.Fatal(err)
	}
	if err := store.upsert(second); err != nil {
		t.Fatal(err)
	}
	first.State = "received"
	if err := store.upsert(first); err != nil {
		t.Fatal(err)
	}
	if err := store.close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := newMessageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.close()
	items, err := reloaded.list()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("archived messages = %d, want 2; database = %s", len(items), filepath.Join(dir, "messages.db"))
	}
	if items[1].State != "received" {
		t.Fatalf("updated message state = %q, want received", items[1].State)
	}
}

func TestMessageStoreFilesArePrivate(t *testing.T) {
	dir := t.TempDir()
	store, err := newMessageStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	if err := store.upsert(testMessage("7", "received")); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(filepath.Join(dir, "messages.db") + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s permissions = %o, want 600", info.Name(), info.Mode().Perm())
		}
	}
}

func TestArchiveCompletesBeforeTerminalModemMessagesAreDeleted(t *testing.T) {
	store, err := newMessageStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	deleted := make([]string, 0)
	a := &api{messageStore: store, deleteSMS: func(_ context.Context, item message) error {
		deleted = append(deleted, item.ModemMessageID)
		return nil
	}}
	items := []message{testMessage("8", "receiving"), testMessage("7", "received"), testMessage("6", "sent")}
	if err := a.archiveAndCleanMessages(context.Background(), items, false); err != nil {
		t.Fatal(err)
	}
	archived, err := store.list()
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 2 {
		t.Fatalf("archived messages = %d, want 2 terminal messages", len(archived))
	}
	if len(deleted) != 2 || deleted[0] != "7" || deleted[1] != "6" {
		t.Fatalf("deleted modem messages = %#v, want [7 6]", deleted)
	}
}

func TestModemDeleteFailureKeepsLocalArchive(t *testing.T) {
	store, err := newMessageStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	a := &api{messageStore: store, deleteSMS: func(_ context.Context, _ message) error {
		return errors.New("temporary D-Bus failure")
	}}
	if err := a.archiveAndCleanMessages(context.Background(), []message{testMessage("7", "received")}, false); err != nil {
		t.Fatal(err)
	}
	items, err := store.list()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("archived messages = %d, want 1", len(items))
	}
}

func TestArchiveFailureNeverDeletesModemMessages(t *testing.T) {
	store, err := newMessageStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.close(); err != nil {
		t.Fatal(err)
	}
	deleted := false
	a := &api{messageStore: store, deleteSMS: func(_ context.Context, _ message) error {
		deleted = true
		return nil
	}}
	if err := a.archiveAndCleanMessages(context.Background(), []message{testMessage("7", "received")}, false); err == nil {
		t.Fatal("archive unexpectedly succeeded after database close")
	}
	if deleted {
		t.Fatal("modem message was deleted after local archive failed")
	}
}

func TestExistingMessagesDoNotBecomeStartupNotifications(t *testing.T) {
	store, err := newMessageStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	if err := store.upsert(testMessage("7", "received")); err != nil {
		t.Fatal(err)
	}
	if err := store.markExistingReceivedNotified(); err != nil {
		t.Fatal(err)
	}
	if err := store.upsert(testMessage("8", "received")); err != nil {
		t.Fatal(err)
	}
	pending, err := store.pendingNotifications()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ModemMessageID != "8" {
		t.Fatalf("pending notifications = %#v, want only modem message 8", pending)
	}
}
