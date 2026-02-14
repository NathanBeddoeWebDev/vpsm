package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// startMockProvider implements domain.Provider with configurable start behavior.
type startMockProvider struct {
	displayName string
	startErr    error
	startedID   string
}

func (m *startMockProvider) GetDisplayName() string { return m.displayName }
func (m *startMockProvider) CreateServer(_ context.Context, _ domain.CreateServerOpts) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *startMockProvider) DeleteServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *startMockProvider) GetServer(_ context.Context, _ string) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *startMockProvider) ListServers(_ context.Context) ([]domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *startMockProvider) StartServer(_ context.Context, id string) error {
	m.startedID = id
	return m.startErr
}
func (m *startMockProvider) StopServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

// registerStartMockProvider resets the global registry and registers a start mock.
func registerStartMockProvider(t *testing.T, name string, mock *startMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (domain.Provider, error) {
		return mock, nil
	})
}

// execStart creates the server command, wires up output buffers, runs "start --provider <provider> [flags...]",
// and returns what was written to stdout and stderr.
func execStart(t *testing.T, providerName string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	args := append([]string{"start", "--provider", providerName}, extraArgs...)
	cmd.SetArgs(args)
	cmd.Execute()
	return outBuf.String(), errBuf.String()
}

func TestStartCommand_WithIDFlag(t *testing.T) {
	mock := &startMockProvider{
		displayName: "Mock",
	}

	registerStartMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "42")

	if mock.startedID != "42" {
		t.Errorf("expected StartServer called with ID '42', got %q", mock.startedID)
	}
	if !strings.Contains(stdout, "started successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Starting server 42") {
		t.Errorf("expected progress message on stderr, got:\n%s", stderr)
	}
}

func TestStartCommand_WithIDFlag_StartError(t *testing.T) {
	mock := &startMockProvider{
		displayName: "Mock",
		startErr:    fmt.Errorf("server not found"),
	}

	registerStartMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "999")

	if !strings.Contains(stderr, "server not found") {
		t.Errorf("expected 'server not found' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "started successfully") {
		t.Errorf("expected no success message on stdout, got:\n%s", stdout)
	}
}

func TestStartCommand_UnknownProvider(t *testing.T) {
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })

	_, stderr := execStart(t, "nonexistent", "--id", "42")

	if !strings.Contains(stderr, "unknown provider") {
		t.Errorf("expected 'unknown provider' error on stderr, got:\n%s", stderr)
	}
}

func TestStartCommand_MissingID(t *testing.T) {
	mock := &startMockProvider{
		displayName: "Mock",
	}

	registerStartMockProvider(t, "mock", mock)

	_, stderr := execStart(t, "mock")

	if !strings.Contains(stderr, "required flag") {
		t.Errorf("expected 'required flag' error on stderr, got:\n%s", stderr)
	}
	if mock.startedID != "" {
		t.Errorf("expected StartServer not to be called, but got ID %q", mock.startedID)
	}
}

func TestStartCommand_ConflictError(t *testing.T) {
	mock := &startMockProvider{
		displayName: "Mock",
		startErr:    fmt.Errorf("failed to start server: %w", domain.ErrConflict),
	}

	registerStartMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "conflict") {
		t.Errorf("expected 'conflict' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "started successfully") {
		t.Errorf("expected no success message on stdout, got:\n%s", stdout)
	}
}