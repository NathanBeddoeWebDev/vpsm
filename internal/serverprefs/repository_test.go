package serverprefs

import (
	"os"
	"path/filepath"
	"testing"
)

func tempRepo(t *testing.T) *SQLiteRepository {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vpsm.db")
	r, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestGet_NotFound(t *testing.T) {
	r := tempRepo(t)

	got, err := r.Get("hetzner", "12345")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent prefs, got %+v", got)
	}
}

func TestSave_Insert(t *testing.T) {
	r := tempRepo(t)

	prefs := &ServerPrefs{
		Provider: "hetzner",
		ServerID: "12345",
		SSHUser:  "ubuntu",
	}

	if err := r.Save(prefs); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if prefs.ID == 0 {
		t.Error("expected ID to be assigned after insert")
	}
	if prefs.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSave_Upsert(t *testing.T) {
	r := tempRepo(t)

	// First insert.
	prefs := &ServerPrefs{
		Provider: "hetzner",
		ServerID: "12345",
		SSHUser:  "root",
	}
	if err := r.Save(prefs); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	firstID := prefs.ID

	// Upsert with same key.
	prefs2 := &ServerPrefs{
		Provider: "hetzner",
		ServerID: "12345",
		SSHUser:  "ubuntu",
	}
	if err := r.Save(prefs2); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	// Should still have only one row.
	got, err := r.Get("hetzner", "12345")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected prefs, got nil")
	}
	if got.SSHUser != "ubuntu" {
		t.Errorf("expected SSHUser 'ubuntu', got %q", got.SSHUser)
	}
	if got.ID != firstID {
		t.Errorf("expected same ID after upsert, got %d (was %d)", got.ID, firstID)
	}
}

func TestGet_Found(t *testing.T) {
	r := tempRepo(t)

	prefs := &ServerPrefs{
		Provider: "hetzner",
		ServerID: "12345",
		SSHUser:  "ubuntu",
	}
	r.Save(prefs)

	got, err := r.Get("hetzner", "12345")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected prefs, got nil")
	}
	if got.SSHUser != "ubuntu" {
		t.Errorf("expected SSHUser 'ubuntu', got %q", got.SSHUser)
	}
}

func TestGet_DifferentProviders(t *testing.T) {
	r := tempRepo(t)

	// Same server ID, different providers.
	r.Save(&ServerPrefs{Provider: "hetzner", ServerID: "1", SSHUser: "root"})
	r.Save(&ServerPrefs{Provider: "aws", ServerID: "1", SSHUser: "ec2-user"})

	hetzner, err := r.Get("hetzner", "1")
	if err != nil {
		t.Fatalf("Get hetzner failed: %v", err)
	}
	if hetzner.SSHUser != "root" {
		t.Errorf("expected hetzner SSHUser 'root', got %q", hetzner.SSHUser)
	}

	aws, err := r.Get("aws", "1")
	if err != nil {
		t.Fatalf("Get aws failed: %v", err)
	}
	if aws.SSHUser != "ec2-user" {
		t.Errorf("expected aws SSHUser 'ec2-user', got %q", aws.SSHUser)
	}
}

func TestSQLiteRepository_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vpsm.db")

	// Write with one repository instance.
	r1, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	r1.Save(&ServerPrefs{Provider: "hetzner", ServerID: "12345", SSHUser: "ubuntu"})
	r1.Close()

	// Read with a new repository instance.
	r2, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	defer r2.Close()

	got, err := r2.Get("hetzner", "12345")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected prefs to be persisted, got nil")
	}
	if got.SSHUser != "ubuntu" {
		t.Errorf("expected SSHUser 'ubuntu', got %q", got.SSHUser)
	}
}

func TestSQLiteRepository_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "vpsm.db")
	r, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed to create nested directory: %v", err)
	}
	defer r.Close()

	prefs := &ServerPrefs{Provider: "hetzner", ServerID: "1", SSHUser: "root"}
	if err := r.Save(prefs); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist at %s, got error: %v", path, err)
	}
}
