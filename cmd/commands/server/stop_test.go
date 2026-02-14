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

// stopMockProvider implements domain.Provider with configurable stop behavior.
type stopMockProvider struct {
	displayName string
	stopErr     error
	stoppedID   string
}

func (m *stopMockProvider) GetDisplayName() string { return m.displayName }
func (m *stopMockProvider) CreateServer(_ context.Context, _ domain.CreateServerOpts) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *stopMockProvider) DeleteServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *stopMockProvider) GetServer(_ context.Context, _ string) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *stopMockProvider) ListServers(_ context.Context) ([]domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *stopMockProvider) StartServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *stopMockProvider) StopServer(_ context.Context, id string) error {
	m.stoppedID = id
	return m.stopErr
}

// registerStopMockProvider resets the global registry and registers a stop mock.
func registerStopMockProvider(t *testing.T, name string, mock *stopMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (domain.Provider, error) {
		return mock, nil
	})
}

// execStop creates the server command, wires up output buffers, runs "stop --provider <provider> [flags...]",
// and returns what was written to stdout and stderr.
func execStop(t *testing.T, providerName string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	args := append([]string{"stop", "--provider", providerName}, extraArgs...)
	cmd.SetArgs(args)
	cmd.Execute()
	return outBuf.String(), errBuf.String()
}

func TestStopCommand_WithIDFlag(t *testing.T) {
	mock := &stopMockProvider{
		displayName: "Mock",
	}

	registerStopMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if mock.stoppedID != "42" {
		t.Errorf("expected StopServer called with ID '42', got %q", mock.stoppedID)
	}
	if !strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Stopping server 42") {
		t.Errorf("expected progress message on stderr, got:\n%s", stderr)
	}
}

func TestStopCommand_WithIDFlag_StopError(t *testing.T) {
	mock := &stopMockProvider{
		displayName: "Mock",
		stopErr:     fmt.Errorf("server not found"),
	}

	registerStopMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "999")

	if !strings.Contains(stderr, "server not found") {
		t.Errorf("expected 'server not found' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected no success message on stdout, got:\n%s", stdout)
	}
}

func TestStopCommand_UnknownProvider(t *testing.T) {
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })

	_, stderr := execStop(t, "nonexistent", "--id", "42")

	if !strings.Contains(stderr, "unknown provider") {
		t.Errorf("expected 'unknown provider' error on stderr, got:\n%s", stderr)
	}
}

func TestStopCommand_MissingID(t *testing.T) {
	mock := &stopMockProvider{
		displayName: "Mock",
	}

	registerStopMockProvider(t, "mock", mock)

	_, stderr := execStop(t, "mock")

	if !strings.Contains(stderr, "required flag") {
		t.Errorf("expected 'required flag' error on stderr, got:\n%s", stderr)
	}
	if mock.stoppedID != "" {
		t.Errorf("expected StopServer not to be called, but got ID %q", mock.stoppedID)
	}
}