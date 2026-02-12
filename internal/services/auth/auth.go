package auth

import (
	"errors"

	"nathanbeddoewebdev/vpsm/internal/util"
)

const ServiceName = "vpsm"

var ErrTokenNotFound = errors.New("auth token not found")

type Store interface {
	SetToken(provider string, token string) error
	GetToken(provider string) (string, error)
	DeleteToken(provider string) error
}

// DefaultStore returns the standard auth store backed by the OS keychain.
func DefaultStore() Store {
	return NewKeyringStore(ServiceName)
}

// NormalizeProvider normalizes a provider name for consistent key lookup.
func NormalizeProvider(provider string) string {
	return util.NormalizeKey(provider)
}
