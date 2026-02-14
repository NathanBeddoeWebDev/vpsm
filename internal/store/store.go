// Package store provides persistent storage for in-flight provider actions.
//
// When a user starts or stops a server, the CLI tracks the action locally
// so that if the process is interrupted (Ctrl+C, crash, etc.) the action
// can be resumed on the next invocation.
//
// Storage is backed by a SQLite database at ~/.config/vpsm/actions.db
// (or the platform-equivalent path returned by os.UserConfigDir).
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	appDir = "vpsm"
	dbFile = "actions.db"
)

// pathOverride, when non-empty, replaces the default database path.
// Intended for testing. Use SetPath / ResetPath to manage.
var pathOverride string

// SetPath overrides the database path. Intended for testing.
func SetPath(p string) { pathOverride = p }

// ResetPath clears the path override. Intended for testing.
func ResetPath() { pathOverride = "" }

// ActionRecord represents a persisted action. It extends
// domain.ActionStatus with metadata needed to resume polling
// after a CLI restart.
type ActionRecord struct {
	// ID is the auto-increment primary key (assigned on insert).
	ID int64

	// ActionID is the provider-specific action identifier used for polling.
	ActionID string

	// Provider is the name of the cloud provider (e.g. "hetzner").
	Provider string

	// ServerID is the ID of the server being acted upon.
	ServerID string

	// ServerName is the human-readable server name (for display).
	ServerName string

	// Command describes the operation, e.g. "start_server", "stop_server".
	Command string

	// TargetStatus is the expected server status when the action completes
	// (e.g. "running", "off").
	TargetStatus string

	// Status is the current state: "running", "success", or "error".
	Status string

	// Progress is a percentage (0–100).
	Progress int

	// ErrorMessage contains a human-readable explanation when Status is "error".
	ErrorMessage string

	// CreatedAt is when the action was first recorded.
	CreatedAt time.Time

	// UpdatedAt is the last time the record was modified.
	UpdatedAt time.Time
}

// ActionStore defines the persistence interface for action records.
type ActionStore interface {
	// Save inserts or updates an action record. On insert (ID == 0), an
	// ID is assigned to the record.
	Save(record *ActionRecord) error

	// Get retrieves a single action record by ID.
	Get(id int64) (*ActionRecord, error)

	// ListPending returns all action records with status "running",
	// ordered by creation time (newest first).
	ListPending() ([]ActionRecord, error)

	// ListRecent returns the most recent n action records regardless of
	// status, ordered by creation time (newest first).
	ListRecent(n int) ([]ActionRecord, error)

	// DeleteOlderThan removes completed/errored records older than d.
	// Returns the number of records removed.
	DeleteOlderThan(d time.Duration) (int64, error)

	// Close releases database resources.
	Close() error
}

// SQLiteStore implements ActionStore backed by a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// DefaultPath returns the default database path.
func DefaultPath() (string, error) {
	if pathOverride != "" {
		return pathOverride, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("store: unable to determine config directory: %w", err)
	}
	return filepath.Join(base, appDir, dbFile), nil
}

// Open creates or opens the action store at the default path.
func Open() (*SQLiteStore, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return OpenAt(path)
}

// OpenAt creates or opens a SQLite database at the given path.
// The parent directory is created if it does not exist.
func OpenAt(path string) (*SQLiteStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: failed to create directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("store: failed to open database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// migrate creates the actions table if it doesn't exist.
func (s *SQLiteStore) migrate() error {
	const ddl = `
		CREATE TABLE IF NOT EXISTS actions (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			action_id     TEXT    NOT NULL DEFAULT '',
			provider      TEXT    NOT NULL,
			server_id     TEXT    NOT NULL,
			server_name   TEXT    NOT NULL DEFAULT '',
			command       TEXT    NOT NULL DEFAULT '',
			target_status TEXT    NOT NULL DEFAULT '',
			status        TEXT    NOT NULL DEFAULT 'running',
			progress      INTEGER NOT NULL DEFAULT 0,
			error_message TEXT    NOT NULL DEFAULT '',
			created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_actions_status ON actions(status);
	`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("store: migration failed: %w", err)
	}
	return nil
}

// Save inserts a new record (ID == 0) or updates an existing one.
func (s *SQLiteStore) Save(r *ActionRecord) error {
	r.UpdatedAt = time.Now().UTC()

	if r.ID == 0 {
		// Insert
		if r.CreatedAt.IsZero() {
			r.CreatedAt = r.UpdatedAt
		}
		result, err := s.db.Exec(`
			INSERT INTO actions (action_id, provider, server_id, server_name, command, target_status, status, progress, error_message, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ActionID, r.Provider, r.ServerID, r.ServerName, r.Command,
			r.TargetStatus, r.Status, r.Progress, r.ErrorMessage,
			r.CreatedAt.Format(time.RFC3339Nano), r.UpdatedAt.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("store: insert failed: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("store: failed to get last insert ID: %w", err)
		}
		r.ID = id
		return nil
	}

	// Update
	result, err := s.db.Exec(`
		UPDATE actions SET action_id=?, provider=?, server_id=?, server_name=?,
		       command=?, target_status=?, status=?, progress=?, error_message=?,
		       updated_at=?
		WHERE id=?`,
		r.ActionID, r.Provider, r.ServerID, r.ServerName, r.Command,
		r.TargetStatus, r.Status, r.Progress, r.ErrorMessage,
		r.UpdatedAt.Format(time.RFC3339Nano), r.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update failed: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("store: action with ID %d not found", r.ID)
	}
	return nil
}

// Get retrieves a single action record by ID.
func (s *SQLiteStore) Get(id int64) (*ActionRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, action_id, provider, server_id, server_name, command,
		       target_status, status, progress, error_message, created_at, updated_at
		FROM actions WHERE id = ?`, id)

	r, err := scanRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: query failed: %w", err)
	}
	return r, nil
}

// ListPending returns all action records with status "running".
func (s *SQLiteStore) ListPending() ([]ActionRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, action_id, provider, server_id, server_name, command,
		       target_status, status, progress, error_message, created_at, updated_at
		FROM actions WHERE status = 'running' ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: query failed: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// ListRecent returns the most recent n action records regardless of status.
func (s *SQLiteStore) ListRecent(n int) ([]ActionRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, action_id, provider, server_id, server_name, command,
		       target_status, status, progress, error_message, created_at, updated_at
		FROM actions ORDER BY created_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("store: query failed: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// DeleteOlderThan removes completed/errored records older than d.
func (s *SQLiteStore) DeleteOlderThan(d time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-d).Format(time.RFC3339Nano)
	result, err := s.db.Exec(`
		DELETE FROM actions WHERE status != 'running' AND updated_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: delete failed: %w", err)
	}
	return result.RowsAffected()
}

// Close releases database resources.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// scanRow scans a single row into an ActionRecord.
func scanRow(row *sql.Row) (*ActionRecord, error) {
	var r ActionRecord
	var createdStr, updatedStr string
	err := row.Scan(
		&r.ID, &r.ActionID, &r.Provider, &r.ServerID, &r.ServerName,
		&r.Command, &r.TargetStatus, &r.Status, &r.Progress, &r.ErrorMessage,
		&createdStr, &updatedStr,
	)
	if err != nil {
		return nil, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedStr)
	return &r, nil
}

// scanRows scans multiple rows into ActionRecords.
func scanRows(rows *sql.Rows) ([]ActionRecord, error) {
	var records []ActionRecord
	for rows.Next() {
		var r ActionRecord
		var createdStr, updatedStr string
		err := rows.Scan(
			&r.ID, &r.ActionID, &r.Provider, &r.ServerID, &r.ServerName,
			&r.Command, &r.TargetStatus, &r.Status, &r.Progress, &r.ErrorMessage,
			&createdStr, &updatedStr,
		)
		if err != nil {
			return nil, fmt.Errorf("store: scan failed: %w", err)
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
		r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedStr)
		records = append(records, r)
	}
	return records, rows.Err()
}
