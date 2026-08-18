package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type callHistoryEntry struct {
	ID           string `json:"id"`
	SourceCallID string `json:"sourceCallId"`
	ModemID      string `json:"modemId"`
	Number       string `json:"number"`
	Direction    string `json:"direction"`
	Outcome      string `json:"outcome"`
	StartedAt    string `json:"startedAt"`
	ConnectedAt  string `json:"connectedAt,omitempty"`
	EndedAt      string `json:"endedAt,omitempty"`
}

type callHistoryStore struct {
	mu      sync.Mutex
	path    string
	entries []callHistoryEntry
	seen    map[string]int
	ignored map[string]bool
}

func newCallHistoryStore(dataDir string) (*callHistoryStore, error) {
	store := &callHistoryStore{seen: make(map[string]int), ignored: make(map[string]bool)}
	if dataDir == "" {
		return store, nil
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	store.path = filepath.Join(dataDir, "call-history.json")
	data, err := os.ReadFile(store.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &store.entries); err != nil {
			return nil, err
		}
	}
	for index, entry := range store.entries {
		store.seen[callHistoryKey(entry.ModemID, entry.SourceCallID)] = index
	}
	return store, nil
}

func (s *callHistoryStore) observe(call voiceCall, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := callHistoryKey(call.ModemID, call.ID)
	if s.ignored[key] {
		return nil
	}
	index, exists := s.seen[key]
	created := !exists
	if !exists {
		entry := callHistoryEntry{ID: now.UTC().Format("20060102T150405.000000000") + "-" + call.ModemID + "-" + call.ID, SourceCallID: call.ID, ModemID: call.ModemID, Number: call.Number, Direction: call.Direction, Outcome: "in-progress", StartedAt: now.UTC().Format(time.RFC3339Nano)}
		s.entries = append([]callHistoryEntry{entry}, s.entries...)
		for k, i := range s.seen {
			s.seen[k] = i + 1
		}
		s.seen[key] = 0
		index = 0
	}
	entry := &s.entries[index]
	before := *entry
	if call.Number != "" {
		entry.Number = call.Number
	}
	if call.Direction != "unknown" {
		entry.Direction = call.Direction
	}
	if call.State == "active" && entry.ConnectedAt == "" {
		entry.ConnectedAt = now.UTC().Format(time.RFC3339Nano)
	}
	if call.State == "terminated" && entry.EndedAt == "" {
		entry.EndedAt = now.UTC().Format(time.RFC3339Nano)
		switch {
		case entry.ConnectedAt != "":
			entry.Outcome = "completed"
		case entry.Direction == "incoming":
			entry.Outcome = "missed"
		default:
			entry.Outcome = "failed"
		}
	}
	if len(s.entries) > 200 {
		s.entries = s.entries[:200]
		s.reindex()
	}
	if !created && *entry == before {
		return nil
	}
	return s.save()
}

func (s *callHistoryStore) list() []callHistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]callHistoryEntry, len(s.entries))
	copy(result, s.entries)
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartedAt > result[j].StartedAt })
	return result
}
func (s *callHistoryStore) clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.seen {
		s.ignored[key] = true
	}
	s.entries = nil
	s.seen = make(map[string]int)
	return s.save()
}
func (s *callHistoryStore) reindex() {
	s.seen = make(map[string]int)
	for i, e := range s.entries {
		s.seen[callHistoryKey(e.ModemID, e.SourceCallID)] = i
	}
}
func (s *callHistoryStore) save() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
func callHistoryKey(modemID, callID string) string { return modemID + "/" + callID }

func (a *api) watchCallHistory(ctx context.Context) {
	if a.callHistory == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			objects, err := a.managedObjects(readCtx)
			if err == nil {
				var calls []voiceCall
				calls, err = a.collectCalls(readCtx, objects)
				if err == nil {
					for _, call := range calls {
						if saveErr := a.callHistory.observe(call, now); saveErr != nil {
							log.Printf("save call history: %v", saveErr)
						}
					}
				}
			}
			cancel()
		}
	}
}

func (a *api) getCallHistory(w http.ResponseWriter, _ *http.Request) {
	if a.callHistory == nil {
		writeJSON(w, http.StatusOK, map[string]any{"calls": []callHistoryEntry{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": a.callHistory.list()})
}
func (a *api) clearCallHistory(w http.ResponseWriter, _ *http.Request) {
	if a.callHistory == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := a.callHistory.clear(); err != nil {
		writeError(w, http.StatusInternalServerError, "清除通话记录失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
