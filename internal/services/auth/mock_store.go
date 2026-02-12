package auth

import "nathanbeddoewebdev/vpsm/internal/util"

// MockStore is an in-memory auth store for testing.
// It normalizes provider keys to match KeyringStore behavior.
type MockStore struct {
	tokens map[string]string
}

func NewMockStore() *MockStore {
	return &MockStore{tokens: make(map[string]string)}
}

func (m *MockStore) SetToken(provider string, token string) error {
	m.tokens[util.NormalizeKey(provider)] = token
	return nil
}

func (m *MockStore) GetToken(provider string) (string, error) {
	token, ok := m.tokens[util.NormalizeKey(provider)]
	if !ok {
		return "", ErrTokenNotFound
	}
	return token, nil
}

func (m *MockStore) DeleteToken(provider string) error {
	key := util.NormalizeKey(provider)
	if _, ok := m.tokens[key]; !ok {
		return ErrTokenNotFound
	}
	delete(m.tokens, key)
	return nil
}
