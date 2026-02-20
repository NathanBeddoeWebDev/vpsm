package names

import (
	"sync"

	"nathanbeddoewebdev/vpsm/internal/util"
)

var (
	mu       sync.RWMutex
	registry = map[string]struct{}{}
)

// Register adds a provider name to the global provider-name registry.
func Register(name string) {
	normalizedName := util.NormalizeKey(name)
	if normalizedName == "" {
		panic("provider names: empty provider name")
	}

	mu.Lock()
	registry[normalizedName] = struct{}{}
	mu.Unlock()
}

// List returns all registered provider names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}

	return names
}

// Reset clears the provider-name registry. Intended for tests only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]struct{}{}
}
