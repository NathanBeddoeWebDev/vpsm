package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vpsm.db")
	s, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSave_Insert(t *testing.T) {
	s := tempStore(t)

	r := &ActionRecord{
		ActionID:     "act-1",
		Provider:     "hetzner",
		ServerID:     "42",
		ServerName:   "web-1",
		Command:      "start_server",
		TargetStatus: "running",
		Status:       "running",
	}

	if err := s.Save(r); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if r.ID == 0 {
		t.Error("expected ID to be assigned after insert")
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if r.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSave_Update(t *testing.T) {
	s := tempStore(t)

	r := &ActionRecord{
		ActionID:     "act-1",
		Provider:     "hetzner",
		ServerID:     "42",
		Command:      "start_server",
		TargetStatus: "running",
		Status:       "running",
	}

	if err := s.Save(r); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	r.Status = "success"
	r.Progress = 100
	if err := s.Save(r); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err := s.Get(r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Status != "success" {
		t.Errorf("expected status 'success', got %q", got.Status)
	}
	if got.Progress != 100 {
		t.Errorf("expected progress 100, got %d", got.Progress)
	}
}

func TestSave_UpdateNotFound(t *testing.T) {
	s := tempStore(t)

	r := &ActionRecord{ID: 999, Status: "running"}
	err := s.Save(r)
	if err == nil {
		t.Fatal("expected error updating non-existent record")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := tempStore(t)

	got, err := s.Get(999)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent record, got %+v", got)
	}
}

func TestGet_Found(t *testing.T) {
	s := tempStore(t)

	r := &ActionRecord{
		ActionID: "act-1",
		Provider: "hetzner",
		ServerID: "42",
		Status:   "running",
	}
	s.Save(r)

	got, err := s.Get(r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected record, got nil")
	}
	if got.ActionID != "act-1" {
		t.Errorf("expected ActionID 'act-1', got %q", got.ActionID)
	}
}

func TestListPending(t *testing.T) {
	s := tempStore(t)

	// Insert mix of running and completed actions.
	for _, status := range []string{"running", "success", "running", "error"} {
		r := &ActionRecord{
			Provider: "hetzner",
			ServerID: "42",
			Status:   status,
		}
		s.Save(r)
	}

	pending, err := s.ListPending()
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending actions, got %d", len(pending))
	}
	for _, r := range pending {
		if r.Status != "running" {
			t.Errorf("expected status 'running', got %q", r.Status)
		}
	}
}

func TestListRecent(t *testing.T) {
	s := tempStore(t)

	for i := 0; i < 5; i++ {
		r := &ActionRecord{
			Provider:  "hetzner",
			ServerID:  "42",
			Status:    "success",
			CreatedAt: time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		s.Save(r)
	}

	recent, err := s.ListRecent(3)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent actions, got %d", len(recent))
	}
	// Should be sorted newest first.
	for i := 1; i < len(recent); i++ {
		if recent[i].CreatedAt.After(recent[i-1].CreatedAt) {
			t.Error("expected records sorted by created_at descending")
		}
	}
}

func TestListRecent_All(t *testing.T) {
	s := tempStore(t)

	for i := 0; i < 3; i++ {
		s.Save(&ActionRecord{Provider: "hetzner", ServerID: "42", Status: "success"})
	}

	// Request more than available.
	recent, err := s.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recent))
	}
}

func TestDeleteOlderThan(t *testing.T) {
	s := tempStore(t)

	recent := &ActionRecord{
		Provider: "hetzner",
		ServerID: "43",
		Status:   "running",
	}
	s.Save(recent)

	completed := &ActionRecord{
		Provider: "hetzner",
		ServerID: "44",
		Status:   "success",
	}
	s.Save(completed)

	// Nothing should be deleted since everything is recent.
	removed, err := s.DeleteOlderThan(24 * time.Hour)
	if err != nil {
		t.Fatalf("DeleteOlderThan failed: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}

	// Delete everything older than 0 (all completed).
	removed, err = s.DeleteOlderThan(0)
	if err != nil {
		t.Fatalf("DeleteOlderThan failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Running action should still be there.
	pending, _ := s.ListPending()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending action remaining, got %d", len(pending))
	}
}

func TestSQLiteStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vpsm.db")

	// Write with one store instance.
	s1, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	r := &ActionRecord{
		ActionID: "act-1",
		Provider: "hetzner",
		ServerID: "42",
		Status:   "running",
	}
	if err := s1.Save(r); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	s1.Close()

	// Read with a new store instance.
	s2, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	defer s2.Close()

	got, err := s2.Get(r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected record to be persisted, got nil")
	}
	if got.ActionID != "act-1" {
		t.Errorf("expected ActionID 'act-1', got %q", got.ActionID)
	}
}

func TestSQLiteStore_EmptyDB(t *testing.T) {
	s := tempStore(t)

	pending, err := s.ListPending()
	if err != nil {
		t.Fatalf("ListPending on empty store failed: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending on empty store, got %d", len(pending))
	}
}

func TestSQLiteStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "vpsm.db")
	s, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed to create nested directory: %v", err)
	}
	defer s.Close()

	r := &ActionRecord{Provider: "hetzner", ServerID: "42", Status: "running"}
	if err := s.Save(r); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist at %s, got error: %v", path, err)
	}
}
