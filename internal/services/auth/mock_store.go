package auth

// MockStore is an in-memory auth store for testing.
type MockStore struct {
	tokens map[string]string
}

func NewMockStore() *MockStore {
	return &MockStore{tokens: make(map[string]string)}
}

func (m *MockStore) SetToken(provider string, token string) error {
	m.tokens[provider] = token
	return nil
}

func (m *MockStore) GetToken(provider string) (string, error) {
	token, ok := m.tokens[provider]
	if !ok {
		return "", ErrTokenNotFound
	}
	return token, nil
}

func (m *MockStore) DeleteToken(provider string) error {
	if _, ok := m.tokens[provider]; !ok {
		return ErrTokenNotFound
	}
	delete(m.tokens, provider)
	return nil
}
