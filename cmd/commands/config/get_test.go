package config

import (
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/config"
)

func TestGet_DefaultProvider_NotSet(t *testing.T) {
	setupTestConfig(t)

	stdout, stderr := execConfig(t, "get", "default-provider")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "not set") {
		t.Errorf("expected 'not set', got: %s", stdout)
	}
}

func TestGet_DefaultProvider_Set(t *testing.T) {
	path := setupTestConfig(t)

	// Write a config value directly.
	cfg := &config.Config{DefaultProvider: "hetzner"}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	stdout, stderr := execConfig(t, "get", "default-provider")

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "hetzner") {
		t.Errorf("expected 'hetzner', got: %s", stdout)
	}
}

func TestGet_UnknownKey(t *testing.T) {
	setupTestConfig(t)

	_, stderr := execConfig(t, "get", "bogus-key")

	if !strings.Contains(stderr, "unknown configuration key") {
		t.Errorf("expected 'unknown configuration key' error, got: %s", stderr)
	}
}
