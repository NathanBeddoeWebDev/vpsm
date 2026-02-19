package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/server/domain"
	"nathanbeddoewebdev/vpsm/internal/server/providers"
	"nathanbeddoewebdev/vpsm/internal/server/services/action"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// stopMockProvider implements domain.Provider with configurable stop behavior.
// It does NOT implement domain.ActionPoller, so polling falls back to GetServer.
type stopMockProvider struct {
	displayName    string
	stopAction     *domain.ActionStatus
	stopErr        error
	stoppedID      string
	getServer      *domain.Server
	getServerErr   error
	getServerCalls int
}

func (m *stopMockProvider) GetDisplayName() string { return m.displayName }
func (m *stopMockProvider) CreateServer(_ context.Context, _ domain.CreateServerOpts) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *stopMockProvider) DeleteServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *stopMockProvider) GetServer(_ context.Context, id string) (*domain.Server, error) {
	m.getServerCalls++
	if m.getServerErr != nil {
		return nil, m.getServerErr
	}
	return m.getServer, nil
}
func (m *stopMockProvider) ListServers(_ context.Context) ([]domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *stopMockProvider) StartServer(_ context.Context, _ string) (*domain.ActionStatus, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *stopMockProvider) StopServer(_ context.Context, id string) (*domain.ActionStatus, error) {
	m.stoppedID = id
	if m.stopErr != nil {
		return nil, m.stopErr
	}
	if m.stopAction != nil {
		return m.stopAction, nil
	}
	return &domain.ActionStatus{Status: domain.ActionStatusSuccess}, nil
}

// stopPollerMockProvider extends stopMockProvider with domain.ActionPoller
// support. This simulates a provider like Hetzner that exposes action polling.
type stopPollerMockProvider struct {
	stopMockProvider
	pollResults    []*domain.ActionStatus
	pollIdx        int
	pollErr        error   // if set, every PollAction call returns this error
	pollErrors     []error // per-call errors (takes precedence over pollErr when non-nil)
	pollErrIdx     int
	polledActionID string
	pollCalls      int
}

func (m *stopPollerMockProvider) PollAction(_ context.Context, actionID string) (*domain.ActionStatus, error) {
	m.polledActionID = actionID
	m.pollCalls++
	// Per-call errors take precedence.
	if m.pollErrIdx < len(m.pollErrors) {
		err := m.pollErrors[m.pollErrIdx]
		m.pollErrIdx++
		if err != nil {
			return nil, err
		}
	} else if m.pollErr != nil {
		return nil, m.pollErr
	}
	if m.pollIdx < len(m.pollResults) {
		result := m.pollResults[m.pollIdx]
		m.pollIdx++
		return result, nil
	}
	return &domain.ActionStatus{Status: domain.ActionStatusSuccess, Progress: 100}, nil
}

// --- Helpers ---

// registerStopMockProvider resets the global registry and registers a stop mock.
func registerStopMockProvider(t *testing.T, name string, mock *stopMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (domain.Provider, error) {
		return mock, nil
	})
}

// registerStopPollerMockProvider resets the global registry and registers
// a stop mock that also implements ActionPoller.
func registerStopPollerMockProvider(t *testing.T, name string, mock *stopPollerMockProvider) {
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

// --- Immediate completion tests (no polling) ---

func TestStopCommand_WithIDFlag(t *testing.T) {
	withFastPolling(t)

	mock := &stopMockProvider{
		displayName: "Mock",
		getServer:   &domain.Server{ID: "42", Name: "test", Status: "off"},
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

// --- Action-based polling tests (provider implements ActionPoller) ---

func TestStopCommand_PollsActionUntilSuccess(t *testing.T) {
	withFastPolling(t)

	mock := &stopPollerMockProvider{
		stopMockProvider: stopMockProvider{
			displayName: "Mock",
			stopAction: &domain.ActionStatus{
				ID:       "action-99",
				Status:   domain.ActionStatusRunning,
				Progress: 0,
				Command:  "stop_server",
			},
			getServer: &domain.Server{ID: "42", Name: "web-server", Status: "off"},
		},
		pollResults: []*domain.ActionStatus{
			{ID: "action-99", Status: domain.ActionStatusRunning, Progress: 50},
			{ID: "action-99", Status: domain.ActionStatusSuccess, Progress: 100},
		},
	}

	registerStopPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if mock.stoppedID != "42" {
		t.Errorf("expected StopServer called with ID '42', got %q", mock.stoppedID)
	}
	if mock.polledActionID != "action-99" {
		t.Errorf("expected PollAction called with 'action-99', got %q", mock.polledActionID)
	}
	if mock.pollCalls < 1 {
		t.Errorf("expected at least 1 poll call, got %d", mock.pollCalls)
	}
	if !strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
	// After action polling succeeds, GetServer is called to confirm
	// the server actually reached the target status.
	if mock.getServerCalls < 1 {
		t.Errorf("expected GetServer to be called for status verification, got %d calls", mock.getServerCalls)
	}
	_ = stderr
}

func TestStopCommand_PollActionReturnsError(t *testing.T) {
	withFastPolling(t)

	mock := &stopPollerMockProvider{
		stopMockProvider: stopMockProvider{
			displayName: "Mock",
			stopAction: &domain.ActionStatus{
				ID:     "action-99",
				Status: domain.ActionStatusRunning,
			},
		},
		pollErr: fmt.Errorf("rate limited while polling action: %w", domain.ErrRateLimited),
	}

	registerStopPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "rate limited") {
		t.Errorf("expected 'rate limited' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}

func TestStopCommand_PollActionReturnsActionError(t *testing.T) {
	withFastPolling(t)

	mock := &stopPollerMockProvider{
		stopMockProvider: stopMockProvider{
			displayName: "Mock",
			stopAction: &domain.ActionStatus{
				ID:     "action-99",
				Status: domain.ActionStatusRunning,
			},
		},
		pollResults: []*domain.ActionStatus{
			{
				ID:           "action-99",
				Status:       domain.ActionStatusError,
				ErrorMessage: "shutdown signal timeout",
			},
		},
	}

	registerStopPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "shutdown signal timeout") {
		t.Errorf("expected 'shutdown signal timeout' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}

// --- Server-status fallback tests (provider does NOT implement ActionPoller) ---

func TestStopCommand_FallsBackToGetServer(t *testing.T) {
	withFastPolling(t)

	mock := &stopMockProvider{
		displayName: "Mock",
		stopAction: &domain.ActionStatus{
			Status: domain.ActionStatusRunning,
		},
		// GetServer returns the server already in "off" state, so the
		// first poll sees the target status and stops.
		getServer: &domain.Server{ID: "42", Name: "web-server", Status: "off"},
	}

	registerStopMockProvider(t, "mock", mock)

	stdout, _ := execStop(t, "mock", "--id", "42")

	if mock.getServerCalls < 1 {
		t.Errorf("expected GetServer to be called for fallback polling, got %d calls", mock.getServerCalls)
	}
	if !strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
}

func TestStopCommand_FallbackGetServerError(t *testing.T) {
	withFastPolling(t)

	mock := &stopMockProvider{
		displayName: "Mock",
		stopAction: &domain.ActionStatus{
			Status: domain.ActionStatusRunning,
		},
		getServerErr: fmt.Errorf("api timeout"),
	}

	registerStopMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "api timeout") {
		t.Errorf("expected 'api timeout' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}

// --- Transient error tolerance tests ---

func TestStopCommand_PollTransientErrorRetry(t *testing.T) {
	withFastPolling(t)

	mock := &stopPollerMockProvider{
		stopMockProvider: stopMockProvider{
			displayName: "Mock",
			stopAction: &domain.ActionStatus{
				ID:     "action-99",
				Status: domain.ActionStatusRunning,
			},
			getServer: &domain.Server{ID: "42", Name: "web-server", Status: "off"},
		},
		pollErrors: []error{
			fmt.Errorf("connection reset"),
			nil, // second call succeeds, falls through to pollResults
		},
		pollResults: []*domain.ActionStatus{
			{ID: "action-99", Status: domain.ActionStatusSuccess, Progress: 100},
		},
	}

	registerStopPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if mock.pollCalls != 2 {
		t.Errorf("expected 2 poll calls (1 error + 1 success), got %d", mock.pollCalls)
	}
	if !strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected success on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Transient error") {
		t.Errorf("expected transient error message on stderr, got:\n%s", stderr)
	}
}

func TestStopCommand_PollTransientErrorExhaustion(t *testing.T) {
	withFastPolling(t)

	mock := &stopPollerMockProvider{
		stopMockProvider: stopMockProvider{
			displayName: "Mock",
			stopAction: &domain.ActionStatus{
				ID:     "action-99",
				Status: domain.ActionStatusRunning,
			},
		},
		pollErr: fmt.Errorf("persistent network failure"),
	}

	registerStopPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStop(t, "mock", "--id", "42")

	if mock.pollCalls != action.MaxTransientErrors {
		t.Errorf("expected %d poll calls before giving up, got %d", action.MaxTransientErrors, mock.pollCalls)
	}
	if !strings.Contains(stderr, "consecutive failures") {
		t.Errorf("expected 'consecutive failures' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "stop initiated successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}
