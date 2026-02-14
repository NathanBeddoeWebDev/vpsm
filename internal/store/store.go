// Package store provides persistent storage for in-flight provider actions.
//
// When a user starts or stops a server, the CLI tracks the action locally
// so that if the process is interrupted (Ctrl+C, crash, etc.) the action
// can be resumed on the next invocation.
//
// Storage is backed by a JSON file at ~/.config/vpsm/actions.json (or the
// platform-equivalent path returned by os.UserConfigDir).
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	appDir   = "vpsm"
	fileName = "actions.json"
)

// pathOverride, when non-empty, replaces the default file path.
// Intended for testing. Use SetPath / ResetPath to manage.
var pathOverride string

// SetPath overrides the actions file path. Intended for testing.
func SetPath(p string) { pathOverride = p }

// ResetPath clears the path override. Intended for testing.
func ResetPath() { pathOverride = "" }

// ActionRecord represents a persisted action. It extends
// domain.ActionStatus with metadata needed to resume polling
// after a CLI restart.
type ActionRecord struct {
	// ID is a locally unique identifier (auto-assigned).
	ID int64 `json:"id"`

	// ActionID is the provider-specific action identifier used for polling.
	ActionID string `json:"action_id"`

	// Provider is the name of the cloud provider (e.g. "hetzner").
	Provider string `json:"provider"`

	// ServerID is the ID of the server being acted upon.
	ServerID string `json:"server_id"`

	// ServerName is the human-readable server name (for display).
	ServerName string `json:"server_name,omitempty"`

	// Command describes the operation, e.g. "start_server", "stop_server".
	Command string `json:"command"`

	// TargetStatus is the expected server status when the action completes
	// (e.g. "running", "off").
	TargetStatus string `json:"target_status"`

	// Status is the current state: "running", "success", or "error".
	Status string `json:"status"`

	// Progress is a percentage (0–100).
	Progress int `json:"progress"`

	// ErrorMessage contains a human-readable explanation when Status is "error".
	ErrorMessage string `json:"error_message,omitempty"`

	// CreatedAt is when the action was first recorded.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the last time the record was modified.
	UpdatedAt time.Time `json:"updated_at"`
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
}

// fileData is the on-disk JSON structure.
type fileData struct {
	NextID  int64          `json:"next_id"`
	Actions []ActionRecord `json:"actions"`
}

// FileStore implements ActionStore backed by a JSON file.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// DefaultPath returns the default actions file path.
func DefaultPath() (string, error) {
	if pathOverride != "" {
		return pathOverride, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("store: unable to determine config directory: %w", err)
	}
	return filepath.Join(base, appDir, fileName), nil
}

// Open creates or opens the action store at the default path.
func Open() (*FileStore, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return OpenAt(path), nil
}

// OpenAt creates or opens the action store at the given path.
func OpenAt(path string) *FileStore {
	return &FileStore{path: path}
}

// Save inserts a new record (ID == 0) or updates an existing one.
func (s *FileStore) Save(r *ActionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return err
	}

	r.UpdatedAt = time.Now().UTC()

	if r.ID == 0 {
		// Insert
		if r.CreatedAt.IsZero() {
			r.CreatedAt = r.UpdatedAt
		}
		data.NextID++
		r.ID = data.NextID
		data.Actions = append(data.Actions, *r)
	} else {
		// Update
		found := false
		for i := range data.Actions {
			if data.Actions[i].ID == r.ID {
				data.Actions[i] = *r
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("store: action with ID %d not found", r.ID)
		}
	}

	return s.save(data)
}

// Get retrieves a single action record by ID.
func (s *FileStore) Get(id int64) (*ActionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, err
	}

	for i := range data.Actions {
		if data.Actions[i].ID == id {
			r := data.Actions[i]
			return &r, nil
		}
	}
	return nil, nil
}

// ListPending returns all action records with status "running".
func (s *FileStore) ListPending() ([]ActionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, err
	}

	var result []ActionRecord
	for _, r := range data.Actions {
		if r.Status == "running" {
			result = append(result, r)
		}
	}
	sortByCreatedDesc(result)
	return result, nil
}

// ListRecent returns the most recent n action records.
func (s *FileStore) ListRecent(n int) ([]ActionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, err
	}

	result := make([]ActionRecord, len(data.Actions))
	copy(result, data.Actions)
	sortByCreatedDesc(result)

	if n > 0 && len(result) > n {
		result = result[:n]
	}
	return result, nil
}

// DeleteOlderThan removes completed/errored records older than d.
func (s *FileStore) DeleteOlderThan(d time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().UTC().Add(-d)
	var kept []ActionRecord
	var removed int64

	for _, r := range data.Actions {
		if r.Status != "running" && r.UpdatedAt.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, r)
	}

	if removed == 0 {
		return 0, nil
	}

	data.Actions = kept
	if err := s.save(data); err != nil {
		return 0, err
	}
	return removed, nil
}

// load reads the JSON file from disk. Returns empty data if the file
// does not exist.
func (s *FileStore) load() (*fileData, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &fileData{}, nil
		}
		return nil, fmt.Errorf("store: failed to read %s: %w", s.path, err)
	}

	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("store: failed to parse %s: %w", s.path, err)
	}
	return &data, nil
}

// save writes the JSON file atomically.
func (s *FileStore) save(data *fileData) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: failed to create directory %s: %w", dir, err)
	}

	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("store: failed to marshal data: %w", err)
	}
	payload = append(payload, '\n')

	// Atomic write: write to temp file then rename.
	tmp, err := os.CreateTemp(dir, "actions-*.tmp")
	if err != nil {
		return fmt.Errorf("store: failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("store: failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("store: failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("store: failed to rename temp file: %w", err)
	}

	return nil
}

func sortByCreatedDesc(records []ActionRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
}
