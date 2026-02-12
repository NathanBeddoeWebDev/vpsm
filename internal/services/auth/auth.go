package auth

import (
	"errors"
	"strings"
)

const ServiceName = "vpsm"

var ErrTokenNotFound = errors.New("auth token not found")

type Store interface {
	SetToken(provider string, token string) error
	GetToken(provider string) (string, error)
	DeleteToken(provider string) error
}

func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}
