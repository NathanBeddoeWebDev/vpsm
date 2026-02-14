package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// showMockProvider extends mockProvider with configurable GetServer behavior.
type showMockProvider struct {
	displayName string
	servers     []domain.Server
	listErr     error
	getServer   *domain.Server
	getErr      error
	gotID       string
}

func (m *showMockProvider) GetDisplayName() string { return m.displayName }
func (m *showMockProvider) CreateServer(_ context.Context, _ domain.CreateServerOpts) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *showMockProvider) DeleteServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *showMockProvider) StartServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *showMockProvider) StopServer(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *showMockProvider) GetServer(_ context.Context, id string) (*domain.Server, error) {
	m.gotID = id
	return m.getServer, m.getErr
}
func (m *showMockProvider) ListServers(_ context.Context) ([]domain.Server, error) {
	return m.servers, m.listErr
}

// registerShowMockProvider resets the global registry and registers a show mock.
func registerShowMockProvider(t *testing.T, name string, mock *showMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (domain.Provider, error) {
		return mock, nil
	})
}

// execShow creates the server command, wires up output buffers, runs "show --provider <provider> [flags...]",
// and returns what was written to stdout and stderr.
func execShow(t *testing.T, providerName string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	args := append([]string{"show", "--provider", providerName}, extraArgs...)
	cmd.SetArgs(args)
	cmd.Execute()
	return outBuf.String(), errBuf.String()
}

func TestShowCommand_WithIDFlag_TableOutput(t *testing.T) {
	server := &domain.Server{
		ID:          "42",
		Name:        "web-server",
		Status:      "running",
		CreatedAt:   time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		PublicIPv4:  "1.2.3.4",
		PublicIPv6:  "2001:db8::",
		PrivateIPv4: "10.0.0.2",
		Region:      "fsn1",
		ServerType:  "cpx11",
		Image:       "ubuntu-24.04",
		Provider:    "mock",
	}

	mock := &showMockProvider{
		displayName: "Mock",
		getServer:   server,
	}

	registerShowMockProvider(t, "mock", mock)

	stdout, _ := execShow(t, "mock", "--id", "42")

	if mock.gotID != "42" {
		t.Errorf("expected GetServer called with ID '42', got %q", mock.gotID)
	}

	assertContainsAll(t, stdout, "stdout", []string{
		"42", "web-server", "running", "mock",
		"cpx11", "ubuntu-24.04", "fsn1",
		"1.2.3.4", "2001:db8::", "10.0.0.2",
		"2024-06-15 12:00:00 UTC",
	})
}

func TestShowCommand_WithIDFlag_JSONOutput(t *testing.T) {
	server := &domain.Server{
		ID:         "42",
		Name:       "web-server",
		Status:     "running",
		CreatedAt:  time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		PublicIPv4: "1.2.3.4",
		Region:     "fsn1",
		ServerType: "cpx11",
		Image:      "ubuntu-24.04",
		Provider:   "mock",
	}

	mock := &showMockProvider{
		displayName: "Mock",
		getServer:   server,
	}

	registerShowMockProvider(t, "mock", mock)

	stdout, _ := execShow(t, "mock", "--id", "42", "-o", "json")

	var got domain.Server
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput:\n%s", err, stdout)
	}

	if got.ID != "42" {
		t.Errorf("expected ID '42', got %q", got.ID)
	}
	if got.Name != "web-server" {
		t.Errorf("expected Name 'web-server', got %q", got.Name)
	}
	if got.Provider != "mock" {
		t.Errorf("expected Provider 'mock', got %q", got.Provider)
	}
}

func TestShowCommand_WithIDFlag_GetError(t *testing.T) {
	mock := &showMockProvider{
		displayName: "Mock",
		getErr:      fmt.Errorf("server not found"),
	}

	registerShowMockProvider(t, "mock", mock)

	stdout, stderr := execShow(t, "mock", "--id", "999")

	if !strings.Contains(stderr, "server not found") {
		t.Errorf("expected 'server not found' on stderr, got:\n%s", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout on error, got:\n%s", stdout)
	}
}

func TestShowCommand_UnknownProvider(t *testing.T) {
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })

	_, stderr := execShow(t, "nonexistent", "--id", "42")

	if !strings.Contains(stderr, "unknown provider") {
		t.Errorf("expected 'unknown provider' error on stderr, got:\n%s", stderr)
	}
}

func TestShowCommand_OmitsEmptyOptionalFields(t *testing.T) {
	server := &domain.Server{
		ID:         "1",
		Name:       "bare-server",
		Status:     "running",
		Region:     "hel1",
		ServerType: "cx11",
		Provider:   "mock",
	}

	mock := &showMockProvider{
		displayName: "Mock",
		getServer:   server,
	}

	registerShowMockProvider(t, "mock", mock)

	stdout, _ := execShow(t, "mock", "--id", "1")

	// These fields should not appear when empty.
	for _, absent := range []string{"IPv6:", "Private IP:", "Image:", "Created:"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("expected %q to be omitted for empty value, but found in output:\n%s", absent, stdout)
		}
	}

	// These should still appear.
	assertContainsAll(t, stdout, "stdout", []string{
		"1", "bare-server", "running", "mock", "cx11", "hel1",
	})
}
