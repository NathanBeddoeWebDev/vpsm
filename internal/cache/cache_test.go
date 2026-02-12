package cache

import (
	"os"
	"testing"
	"time"
)

func TestCache_SetGetRoundTrip(t *testing.T) {
	c := New(t.TempDir())
	key := "locations"

	want := []string{"fsn1", "nbg1"}
	if err := c.Set(key, want); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	var got []string
	hit, err := c.Get(key, time.Hour, &got)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit, got miss")
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCache_ExpiredEntry(t *testing.T) {
	c := New(t.TempDir())
	key := "images"

	if err := c.Set(key, []string{"ubuntu"}); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	path := c.pathForKey(key)
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to update cache mtime: %v", err)
	}

	var got []string
	hit, err := c.Get(key, time.Hour, &got)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}
	if hit {
		t.Fatal("expected cache miss for expired entry")
	}
}

func TestCache_CorruptEntry(t *testing.T) {
	c := New(t.TempDir())
	key := "server_types"

	path := c.pathForKey(key)
	if err := os.WriteFile(path, []byte("{invalid json"), 0o600); err != nil {
		t.Fatalf("failed to write corrupt cache file: %v", err)
	}

	var got []string
	hit, err := c.Get(key, time.Hour, &got)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}
	if hit {
		t.Fatal("expected cache miss for corrupt entry")
	}
}
