package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// startMockProvider implements domain.Provider with configurable start behavior.
// It does NOT implement domain.ActionPoller, so polling falls back to GetServer.
type startMockProvider struct {
	displayName    string
	startAction    *domain.ActionStatus
	startErr       error
	startedID      string
	getServer      *domain.Server
	getServerErr   error
	getServerCalls int
}

func (m *startMockProvider) GetDisplayName() string { return m.displayName }
func (m *startMockProvider) CreateServer(_ context.Context, _ domain.CreateServerOpts) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *startMockProvider) DeleteServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *startMockProvider) GetServer(_ context.Context, id string) (*domain.Server, error) {
	m.getServerCalls++
	if m.getServerErr != nil {
		return nil, m.getServerErr
	}
	return m.getServer, nil
}
func (m *startMockProvider) ListServers(_ context.Context) ([]domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *startMockProvider) StartServer(_ context.Context, id string) (*domain.ActionStatus, error) {
	m.startedID = id
	if m.startErr != nil {
		return nil, m.startErr
	}
	if m.startAction != nil {
		return m.startAction, nil
	}
	return &domain.ActionStatus{Status: domain.ActionStatusSuccess}, nil
}
func (m *startMockProvider) StopServer(_ context.Context, _ string) (*domain.ActionStatus, error) {
	return nil, fmt.Errorf("not implemented")
}

// startPollerMockProvider extends startMockProvider with domain.ActionPoller
// support. This simulates a provider like Hetzner that exposes action polling.
type startPollerMockProvider struct {
	startMockProvider
	pollResults    []*domain.ActionStatus
	pollIdx        int
	pollErr        error   // if set, every PollAction call returns this error
	pollErrors     []error // per-call errors (takes precedence over pollErr when non-nil)
	pollErrIdx     int
	polledActionID string
	pollCalls      int
}

func (m *startPollerMockProvider) PollAction(_ context.Context, actionID string) (*domain.ActionStatus, error) {
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

// registerStartMockProvider resets the global registry and registers a start mock.
func registerStartMockProvider(t *testing.T, name string, mock *startMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (domain.Provider, error) {
		return mock, nil
	})
}

// registerStartPollerMockProvider resets the global registry and registers
// a start mock that also implements ActionPoller.
func registerStartPollerMockProvider(t *testing.T, name string, mock *startPollerMockProvider) {
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

// withFastPolling overrides actionPollInterval for the duration of the test
// so poll-based tests complete quickly.
func withFastPolling(t *testing.T) {
	t.Helper()
	orig := actionPollInterval
	actionPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { actionPollInterval = orig })
}

// --- Immediate completion tests (no polling) ---

func TestStartCommand_WithIDFlag(t *testing.T) {
	withFastPolling(t)

	mock := &startMockProvider{
		displayName: "Mock",
		getServer:   &domain.Server{ID: "42", Name: "test", Status: "running"},
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

// --- Action-based polling tests (provider implements ActionPoller) ---

func TestStartCommand_PollsActionUntilSuccess(t *testing.T) {
	withFastPolling(t)

	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{
			displayName: "Mock",
			startAction: &domain.ActionStatus{
				ID:       "action-1",
				Status:   domain.ActionStatusRunning,
				Progress: 0,
				Command:  "start_server",
			},
			getServer: &domain.Server{ID: "42", Name: "web-server", Status: "running"},
		},
		pollResults: []*domain.ActionStatus{
			{ID: "action-1", Status: domain.ActionStatusRunning, Progress: 50},
			{ID: "action-1", Status: domain.ActionStatusSuccess, Progress: 100},
		},
	}

	registerStartPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "42")

	if mock.startedID != "42" {
		t.Errorf("expected StartServer called with ID '42', got %q", mock.startedID)
	}
	if mock.polledActionID != "action-1" {
		t.Errorf("expected PollAction called with 'action-1', got %q", mock.polledActionID)
	}
	if mock.pollCalls < 1 {
		t.Errorf("expected at least 1 poll call, got %d", mock.pollCalls)
	}
	if !strings.Contains(stdout, "started successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
	// After action polling succeeds, GetServer is called to confirm
	// the server actually reached the target status.
	if mock.getServerCalls < 1 {
		t.Errorf("expected GetServer to be called for status verification, got %d calls", mock.getServerCalls)
	}
	_ = stderr
}

func TestStartCommand_PollActionReturnsError(t *testing.T) {
	withFastPolling(t)

	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{
			displayName: "Mock",
			startAction: &domain.ActionStatus{
				ID:     "action-1",
				Status: domain.ActionStatusRunning,
			},
		},
		pollErr: fmt.Errorf("rate limited while polling action: %w", domain.ErrRateLimited),
	}

	registerStartPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "rate limited") {
		t.Errorf("expected 'rate limited' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "started successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}

func TestStartCommand_PollActionReturnsActionError(t *testing.T) {
	withFastPolling(t)

	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{
			displayName: "Mock",
			startAction: &domain.ActionStatus{
				ID:     "action-1",
				Status: domain.ActionStatusRunning,
			},
		},
		pollResults: []*domain.ActionStatus{
			{
				ID:           "action-1",
				Status:       domain.ActionStatusError,
				ErrorMessage: "server disk corrupted",
			},
		},
	}

	registerStartPollerMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "server disk corrupted") {
		t.Errorf("expected 'server disk corrupted' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "started successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}

// --- Server-status fallback tests (provider does NOT implement ActionPoller) ---

func TestStartCommand_FallsBackToGetServer(t *testing.T) {
	withFastPolling(t)

	mock := &startMockProvider{
		displayName: "Mock",
		startAction: &domain.ActionStatus{
			Status: domain.ActionStatusRunning,
		},
		// GetServer returns the server already in "running" state, so the
		// first poll sees the target status and stops.
		getServer: &domain.Server{ID: "42", Name: "web-server", Status: "running"},
	}

	registerStartMockProvider(t, "mock", mock)

	stdout, _ := execStart(t, "mock", "--id", "42")

	if mock.getServerCalls < 1 {
		t.Errorf("expected GetServer to be called for fallback polling, got %d calls", mock.getServerCalls)
	}
	if !strings.Contains(stdout, "started successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
}

func TestStartCommand_FallbackGetServerError(t *testing.T) {
	withFastPolling(t)

	mock := &startMockProvider{
		displayName: "Mock",
		startAction: &domain.ActionStatus{
			Status: domain.ActionStatusRunning,
		},
		getServerErr: fmt.Errorf("api timeout"),
	}

	registerStartMockProvider(t, "mock", mock)

	stdout, stderr := execStart(t, "mock", "--id", "42")

	if !strings.Contains(stderr, "api timeout") {
		t.Errorf("expected 'api timeout' on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "started successfully") {
		t.Errorf("expected no success on stdout, got:\n%s", stdout)
	}
}

// --- Context cancellation test ---

func TestWaitForAction_ContextCancelled(t *testing.T) {
	withFastPolling(t)

	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{displayName: "Mock"},
		pollResults: []*domain.ActionStatus{
			{ID: "action-1", Status: domain.ActionStatusRunning, Progress: 10},
		},
	}

	registerStartPollerMockProvider(t, "mock", mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var buf bytes.Buffer
	action := &domain.ActionStatus{
		ID:     "action-1",
		Status: domain.ActionStatusRunning,
	}

	err := waitForAction(ctx, mock, action, "42", "running", &buf)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

// --- Transient error tolerance tests ---

func TestPollByAction_TransientErrorRetry(t *testing.T) {
	withFastPolling(t)

	// Two transient errors followed by a successful result.
	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{
			displayName: "Mock",
			getServer:   &domain.Server{ID: "42", Name: "test", Status: "running"},
		},
		pollErrors: []error{
			fmt.Errorf("connection reset"),
			fmt.Errorf("dns timeout"),
			nil, // third call succeeds, falls through to pollResults
		},
		pollResults: []*domain.ActionStatus{
			{ID: "action-1", Status: domain.ActionStatusSuccess, Progress: 100},
		},
	}

	registerStartPollerMockProvider(t, "mock", mock)

	var buf bytes.Buffer
	action := &domain.ActionStatus{
		ID:     "action-1",
		Status: domain.ActionStatusRunning,
	}

	err := waitForAction(context.Background(), mock, action, "42", "running", &buf)
	if err != nil {
		t.Fatalf("expected nil error after transient recovery, got: %v", err)
	}
	if mock.pollCalls != 3 {
		t.Errorf("expected 3 poll calls (2 errors + 1 success), got %d", mock.pollCalls)
	}
	output := buf.String()
	if !strings.Contains(output, "Transient error") {
		t.Errorf("expected transient error messages in output, got:\n%s", output)
	}
}

func TestPollByAction_TransientErrorExhaustion(t *testing.T) {
	withFastPolling(t)

	// Three consecutive transient errors — exceeds the budget.
	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{displayName: "Mock"},
		pollErr:           fmt.Errorf("persistent network failure"),
	}

	registerStartPollerMockProvider(t, "mock", mock)

	var buf bytes.Buffer
	action := &domain.ActionStatus{
		ID:     "action-1",
		Status: domain.ActionStatusRunning,
	}

	err := waitForAction(context.Background(), mock, action, "42", "running", &buf)
	if err == nil {
		t.Fatal("expected error after exhausting transient error budget, got nil")
	}
	if !strings.Contains(err.Error(), "consecutive failures") {
		t.Errorf("expected 'consecutive failures' in error, got: %v", err)
	}
	if mock.pollCalls != maxTransientErrors {
		t.Errorf("expected %d poll calls before giving up, got %d", maxTransientErrors, mock.pollCalls)
	}
}
