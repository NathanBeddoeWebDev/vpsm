package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

type KeyringStore struct {
	serviceName string
}

func NewKeyringStore(serviceName string) *KeyringStore {
	if serviceName == "" {
		serviceName = ServiceName
	}
	return &KeyringStore{serviceName: serviceName}
}

func (k *KeyringStore) SetToken(provider string, token string) error {
	providerKey := NormalizeProvider(provider)
	return keyring.Set(k.serviceName, providerKey, token)
}

func (k *KeyringStore) GetToken(provider string) (string, error) {
	providerKey := NormalizeProvider(provider)
	token, err := keyring.Get(k.serviceName, providerKey)
	if err == nil {
		return token, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrTokenNotFound
	}
	return "", err
}

func (k *KeyringStore) DeleteToken(provider string) error {
	providerKey := NormalizeProvider(provider)
	err := keyring.Delete(k.serviceName, providerKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrTokenNotFound
	}
	return err
}
