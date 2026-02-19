package providers

import (
	"fmt"
	"sync"

	"nathanbeddoewebdev/vpsm/internal/platform/providers/names"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	sshkeydomain "nathanbeddoewebdev/vpsm/internal/sshkey/domain"
	"nathanbeddoewebdev/vpsm/internal/util"
)

// Factory creates an SSH key provider implementation.
type Factory func(store auth.Store) (sshkeydomain.Provider, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register registers an SSH key provider factory by name.
func Register(name string, factory Factory) {
	normalizedName := util.NormalizeKey(name)
	if normalizedName == "" {
		panic("sshkey providers: empty provider name")
	}
	if factory == nil {
		panic("sshkey providers: nil factory")
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[normalizedName]; exists {
		panic(fmt.Sprintf("sshkey providers: provider %q already registered", name))
	}

	registry[normalizedName] = factory
	names.Register(normalizedName)
}

// Get resolves and constructs an SSH key provider by name.
func Get(name string, store auth.Store) (sshkeydomain.Provider, error) {
	normalizedName := util.NormalizeKey(name)

	mu.RLock()
	factory, ok := registry[normalizedName]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sshkey providers: unknown provider %q", name)
	}

	provider, err := factory(store)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// Reset clears the SSH key provider registry. Intended for tests only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]Factory{}
}

// List returns all registered SSH key provider names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
