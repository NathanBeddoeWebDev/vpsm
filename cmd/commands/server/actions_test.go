package server

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/store"
)

// withTestStore sets up a temporary SQLite store for testing.
func withTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vpsm.db")
	store.SetPath(path)
	t.Cleanup(func() { store.ResetPath() })
	s, err := store.OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func execActions(t *testing.T, providerName string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	args := append([]string{"actions", "--provider", providerName}, extraArgs...)
	cmd.SetArgs(args)
	cmd.Execute()
	return outBuf.String(), errBuf.String()
}

func TestActionsCommand_NoPending(t *testing.T) {
	withTestStore(t)

	mock := &startMockProvider{displayName: "Mock"}
	registerStartMockProvider(t, "mock", mock)

	stdout, _ := execActions(t, "mock")

	if !strings.Contains(stdout, "No pending actions") {
		t.Errorf("expected 'No pending actions' message, got:\n%s", stdout)
	}
}

func TestActionsCommand_ShowsPending(t *testing.T) {
	s := withTestStore(t)

	mock := &startMockProvider{displayName: "Mock"}
	registerStartMockProvider(t, "mock", mock)

	// Insert a pending action.
	r := &store.ActionRecord{
		ActionID:     "act-1",
		Provider:     "mock",
		ServerID:     "42",
		Command:      "start_server",
		TargetStatus: "running",
		Status:       "running",
	}
	s.Save(r)

	stdout, stderr := execActions(t, "mock")

	if !strings.Contains(stdout, "42") {
		t.Errorf("expected server ID '42' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "start_server") {
		t.Errorf("expected 'start_server' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "running") {
		t.Errorf("expected 'running' status in output, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "--resume") {
		t.Errorf("expected '--resume' hint on stderr, got:\n%s", stderr)
	}
}

func TestActionsCommand_ShowsAll(t *testing.T) {
	s := withTestStore(t)

	mock := &startMockProvider{displayName: "Mock"}
	registerStartMockProvider(t, "mock", mock)

	// Insert a completed action.
	r := &store.ActionRecord{
		ActionID:     "act-1",
		Provider:     "mock",
		ServerID:     "42",
		Command:      "start_server",
		TargetStatus: "running",
		Status:       "success",
		Progress:     100,
	}
	s.Save(r)

	// Without --all, it should show "No pending actions".
	stdout, _ := execActions(t, "mock")
	if !strings.Contains(stdout, "No pending actions") {
		t.Errorf("expected 'No pending actions' without --all, got:\n%s", stdout)
	}

	// With --all, it should show the completed action.
	stdout, _ = execActions(t, "mock", "--all")
	if !strings.Contains(stdout, "42") {
		t.Errorf("expected server ID '42' in --all output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "success") {
		t.Errorf("expected 'success' status in --all output, got:\n%s", stdout)
	}
}

func TestActionsCommand_ResumeNoPending(t *testing.T) {
	withTestStore(t)

	mock := &startMockProvider{displayName: "Mock"}
	registerStartMockProvider(t, "mock", mock)

	stdout, _ := execActions(t, "mock", "--resume")

	if !strings.Contains(stdout, "No pending actions to resume") {
		t.Errorf("expected 'No pending actions to resume', got:\n%s", stdout)
	}
}

func TestActionsCommand_ResumeSuccess(t *testing.T) {
	withFastPolling(t)
	s := withTestStore(t)

	mock := &startPollerMockProvider{
		startMockProvider: startMockProvider{
			displayName: "Mock",
			getServer:   &domain.Server{ID: "42", Name: "web-1", Status: "running"},
		},
		pollResults: []*domain.ActionStatus{
			{ID: "act-1", Status: domain.ActionStatusSuccess, Progress: 100},
		},
	}

	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register("mock", func(st auth.Store) (domain.Provider, error) {
		return mock, nil
	})

	// Insert a pending action.
	r := &store.ActionRecord{
		ActionID:     "act-1",
		Provider:     "mock",
		ServerID:     "42",
		Command:      "start_server",
		TargetStatus: "running",
		Status:       "running",
	}
	s.Save(r)

	stdout, stderr := execActions(t, "mock", "--resume")

	if !strings.Contains(stderr, "Resuming") {
		t.Errorf("expected 'Resuming' message on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stdout, "started successfully") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}

	// Verify the record was updated.
	got, _ := s.Get(r.ID)
	if got == nil {
		t.Fatal("expected record to still exist")
	}
	if got.Status != "success" {
		t.Errorf("expected record status 'success', got %q", got.Status)
	}
}
