package providers

import (
	"fmt"
	"strings"
	"sync"
)

type Factory func() Provider

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func Register(name string, factory Factory) {
	normalizedName := normalize(name)
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
}

func Get(name string) (Provider, error) {
	normalizedName := normalize(name)
	mu.RLock()
	factory, ok := registry[normalizedName]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("providers: unknown provider %q", name)
	}

	return factory(), nil
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
