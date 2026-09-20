package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

type messageStore struct {
	db   *sql.DB
	path string
}

func newMessageStore(dataDir string) (*messageStore, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("message data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create message data directory: %w", err)
	}
	path := filepath.Join(dataDir, "messages.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open message database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &messageStore{db: db, path: path}
	if err := store.initialize(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *messageStore) initialize() error {
	_, err := s.db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = FULL;
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fingerprint TEXT NOT NULL UNIQUE,
			modem_message_id TEXT NOT NULL,
			modem_id TEXT NOT NULL,
			number TEXT NOT NULL,
			sender_name TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL,
			direction TEXT NOT NULL,
			state TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			notified INTEGER NOT NULL DEFAULT 0,
			archived_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS messages_direction_notified ON messages(direction, notified);
	`)
	if err != nil {
		return fmt.Errorf("initialize message database: %w", err)
	}
	for _, path := range []string{s.path, s.path + "-wal", s.path + "-shm"} {
		if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("secure message database: %w", err)
		}
	}
	return nil
}

func (s *messageStore) close() error { return s.db.Close() }

func messageFingerprint(item message) string {
	sum := sha256.Sum256([]byte(item.ModemID + "\x00" + item.ModemMessageID + "\x00" + item.Number + "\x00" + item.Text + "\x00" + item.Direction + "\x00" + item.Timestamp))
	return hex.EncodeToString(sum[:])
}

const upsertMessageSQL = `
	INSERT INTO messages (fingerprint, modem_message_id, modem_id, number, sender_name, text, direction, state, timestamp, archived_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(fingerprint) DO UPDATE SET
		sender_name = excluded.sender_name,
		state = excluded.state
`

func (s *messageStore) upsert(item message) error {
	return s.archive([]message{item})
}

func (s *messageStore) archive(items []message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin message archive: %w", err)
	}
	defer tx.Rollback()
	for index := len(items) - 1; index >= 0; index-- {
		item := items[index]
		if _, err := tx.Exec(upsertMessageSQL, messageFingerprint(item), item.ModemMessageID, item.ModemID, item.Number, item.SenderName, item.Text, item.Direction, item.State, item.Timestamp, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("archive message: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit message archive: %w", err)
	}
	return nil
}

func (s *messageStore) list() ([]message, error) {
	rows, err := s.db.Query(`SELECT id, modem_message_id, modem_id, number, sender_name, text, direction, state, timestamp FROM messages ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list archived messages: %w", err)
	}
	defer rows.Close()
	items := make([]message, 0)
	for rows.Next() {
		var item message
		var id int64
		if err := rows.Scan(&id, &item.ModemMessageID, &item.ModemID, &item.Number, &item.SenderName, &item.Text, &item.Direction, &item.State, &item.Timestamp); err != nil {
			return nil, fmt.Errorf("read archived message: %w", err)
		}
		item.ID = strconv.FormatInt(id, 10)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list archived messages: %w", err)
	}
	return items, nil
}

func (s *messageStore) delete(id string) (bool, error) {
	result, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete archived message: %w", err)
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func (s *messageStore) get(id string) (message, bool, error) {
	var item message
	var storedID int64
	err := s.db.QueryRow(`SELECT id, modem_message_id, modem_id, number, sender_name, text, direction, state, timestamp FROM messages WHERE id = ?`, id).
		Scan(&storedID, &item.ModemMessageID, &item.ModemID, &item.Number, &item.SenderName, &item.Text, &item.Direction, &item.State, &item.Timestamp)
	if err == sql.ErrNoRows {
		return message{}, false, nil
	}
	if err != nil {
		return message{}, false, fmt.Errorf("get archived message: %w", err)
	}
	item.ID = strconv.FormatInt(storedID, 10)
	return item, true, nil
}

func (s *messageStore) pendingNotifications() ([]message, error) {
	rows, err := s.db.Query(`SELECT id, modem_message_id, modem_id, number, sender_name, text, direction, state, timestamp FROM messages WHERE direction = 'received' AND state = 'received' AND notified = 0 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list pending message notifications: %w", err)
	}
	defer rows.Close()
	items := make([]message, 0)
	for rows.Next() {
		var item message
		var id int64
		if err := rows.Scan(&id, &item.ModemMessageID, &item.ModemID, &item.Number, &item.SenderName, &item.Text, &item.Direction, &item.State, &item.Timestamp); err != nil {
			return nil, fmt.Errorf("read pending message notification: %w", err)
		}
		item.ID = strconv.FormatInt(id, 10)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *messageStore) markNotified(id string) error {
	_, err := s.db.Exec(`UPDATE messages SET notified = 1 WHERE id = ?`, id)
	return err
}

func (s *messageStore) markExistingReceivedNotified() error {
	_, err := s.db.Exec(`UPDATE messages SET notified = 1 WHERE direction = 'received' AND state = 'received'`)
	return err
}
