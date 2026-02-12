package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "config.json")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultProvider != "" {
		t.Errorf("expected empty DefaultProvider, got %q", cfg.DefaultProvider)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vpsm", "config.json")

	want := &Config{DefaultProvider: "hetzner"}
	if err := want.SaveTo(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("config mismatch (-want +got):\n%s", diff)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	path := filepath.Join(dir, "config.json")

	cfg := &Config{DefaultProvider: "hetzner"}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file exists.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json}"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	first := &Config{DefaultProvider: "hetzner"}
	if err := first.SaveTo(path); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	second := &Config{DefaultProvider: "digitalocean"}
	if err := second.SaveTo(path); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got.DefaultProvider != "digitalocean" {
		t.Errorf("expected DefaultProvider %q, got %q", "digitalocean", got.DefaultProvider)
	}
}

func TestLoad_EmptyDefaultProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultProvider != "" {
		t.Errorf("expected empty DefaultProvider, got %q", cfg.DefaultProvider)
	}
}
