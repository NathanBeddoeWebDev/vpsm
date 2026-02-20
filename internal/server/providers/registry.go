package providers

import (
	"fmt"
	"sync"

	"nathanbeddoewebdev/vpsm/internal/platform/providers/names"
	"nathanbeddoewebdev/vpsm/internal/server/domain"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/util"
)

type Factory func(store auth.Store) (domain.Provider, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

func Register(name string, factory Factory) {
	normalizedName := util.NormalizeKey(name)
	if normalizedName == "" {
		panic("providers: empty provider name")
	}
	if factory == nil {
		panic("providers: nil factory")
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[normalizedName]; exists {
		panic(fmt.Sprintf("providers: provider %q already registered", name))
	}

	registry[normalizedName] = factory
	names.Register(normalizedName)
}

func Get(name string, store auth.Store) (domain.Provider, error) {
	normalizedName := util.NormalizeKey(name)
	mu.RLock()
	factory, ok := registry[normalizedName]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("providers: unknown provider %q", name)
	}

	provider, err := factory(store)
	if err != nil {
		return nil, err
	}

	return provider, nil
}

// Reset clears the provider registry. Intended for use in tests only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]Factory{}
}

func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}

	return names
}
