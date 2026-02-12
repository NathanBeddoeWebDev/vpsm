package config

import (
	"strings"
	"testing"
)

func TestLookup_Exists(t *testing.T) {
	spec := Lookup("default-provider")
	if spec == nil {
		t.Fatal("expected to find key 'default-provider', got nil")
	}
	if spec.Name != "default-provider" {
		t.Errorf("expected Name %q, got %q", "default-provider", spec.Name)
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	spec := Lookup("DEFAULT-PROVIDER")
	if spec == nil {
		t.Fatal("expected case-insensitive lookup to succeed")
	}
	if spec.Name != "default-provider" {
		t.Errorf("expected Name %q, got %q", "default-provider", spec.Name)
	}
}

func TestLookup_NotFound(t *testing.T) {
	spec := Lookup("nonexistent-key")
	if spec != nil {
		t.Errorf("expected nil for unknown key, got %+v", spec)
	}
}

func TestKeys_AllHaveGetAndSet(t *testing.T) {
	for _, k := range Keys {
		if k.Get == nil {
			t.Errorf("key %q has nil Get function", k.Name)
		}
		if k.Set == nil {
			t.Errorf("key %q has nil Set function", k.Name)
		}
		if k.Description == "" {
			t.Errorf("key %q has empty Description", k.Name)
		}
	}
}

func TestKeys_GetSetRoundtrip(t *testing.T) {
	for _, k := range Keys {
		cfg := &Config{}
		k.Set(cfg, "test-value")
		got := k.Get(cfg)
		if got != "test-value" {
			t.Errorf("key %q: Set then Get = %q, want %q", k.Name, got, "test-value")
		}
	}
}

func TestKeyNames(t *testing.T) {
	names := KeyNames()
	if len(names) != len(Keys) {
		t.Fatalf("expected %d names, got %d", len(Keys), len(names))
	}
	for i, name := range names {
		if name != Keys[i].Name {
			t.Errorf("index %d: expected %q, got %q", i, Keys[i].Name, name)
		}
	}
}

func TestKeysHelp_ContainsAllKeys(t *testing.T) {
	help := KeysHelp()
	if !strings.Contains(help, "Available keys:") {
		t.Error("expected 'Available keys:' header in help output")
	}
	for _, k := range Keys {
		if !strings.Contains(help, k.Name) {
			t.Errorf("expected key %q in help output", k.Name)
		}
		if !strings.Contains(help, k.Description) {
			t.Errorf("expected description %q in help output", k.Description)
		}
	}
}
