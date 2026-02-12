package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// deleteMockProvider extends mockProvider with configurable delete behavior.
type deleteMockProvider struct {
	displayName string
	servers     []domain.Server
	listErr     error
	deleteErr   error
	deletedID   string
}

func (m *deleteMockProvider) GetDisplayName() string { return m.displayName }
func (m *deleteMockProvider) CreateServer(opts domain.CreateServerOpts) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *deleteMockProvider) DeleteServer(id string) error {
	m.deletedID = id
	return m.deleteErr
}
func (m *deleteMockProvider) ListServers() ([]domain.Server, error) {
	return m.servers, m.listErr
}

// registerDeleteMockProvider resets the global registry and registers a delete mock.
func registerDeleteMockProvider(t *testing.T, name string, mock *deleteMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (domain.Provider, error) {
		return mock, nil
	})
}

// execDelete creates the server command, wires up output buffers, runs "delete --provider <provider> [flags...]",
// and returns what was written to stdout and stderr.
func execDelete(t *testing.T, providerName string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	args := append([]string{"delete", "--provider", providerName}, extraArgs...)
	cmd.SetArgs(args)
	cmd.Execute()
	return outBuf.String(), errBuf.String()
}

func TestDeleteCommand_WithIDFlag(t *testing.T) {
	mock := &deleteMockProvider{
		displayName: "Mock",
	}

	registerDeleteMockProvider(t, "mock", mock)

	stdout, stderr := execDelete(t, "mock", "--id", "42")

	if mock.deletedID != "42" {
		t.Errorf("expected DeleteServer called with ID '42', got %q", mock.deletedID)
	}
	if !strings.Contains(stdout, "deleted successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Deleting server 42") {
		t.Errorf("expected progress message on stderr, got:\n%s", stderr)
	}
}

func TestDeleteCommand_WithIDFlag_DeleteError(t *testing.T) {
	mock := &deleteMockProvider{
		displayName: "Mock",
		deleteErr:   fmt.Errorf("server not found"),
	}

	registerDeleteMockProvider(t, "mock", mock)

	stdout, stderr := execDelete(t, "mock", "--id", "999")

	if !strings.Contains(stderr, "server not found") {
		t.Errorf("expected 'server not found' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "deleted successfully") {
		t.Errorf("expected no success message on stdout, got:\n%s", stdout)
	}
}

func TestDeleteCommand_UnknownProvider(t *testing.T) {
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })

	_, stderr := execDelete(t, "nonexistent", "--id", "42")

	if !strings.Contains(stderr, "unknown provider") {
		t.Errorf("expected 'unknown provider' error on stderr, got:\n%s", stderr)
	}
}
