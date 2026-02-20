package sshkey

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	platformsshkey "nathanbeddoewebdev/vpsm/internal/platform/sshkey"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	sshkeydomain "nathanbeddoewebdev/vpsm/internal/sshkey/domain"
	"nathanbeddoewebdev/vpsm/internal/sshkey/providers"
	"nathanbeddoewebdev/vpsm/internal/sshkeys"
)

// sshKeyMockProvider implements ssh key provider behavior for testing.
type sshKeyMockProvider struct {
	displayName       string
	createErr         error
	createdKey        *platformsshkey.Spec
	capturedName      string
	capturedPublicKey string
}

func (m *sshKeyMockProvider) GetDisplayName() string { return m.displayName }

func (m *sshKeyMockProvider) CreateSSHKey(_ context.Context, name, publicKey string) (*platformsshkey.Spec, error) {
	m.capturedName = name
	m.capturedPublicKey = publicKey
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createdKey != nil {
		return m.createdKey, nil
	}
	return &platformsshkey.Spec{
		ID:          "123",
		Name:        name,
		Fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
	}, nil
}

// registerSSHKeyMockProvider resets the global registry and registers the mock.
func registerSSHKeyMockProvider(t *testing.T, name string, mock *sshKeyMockProvider) {
	t.Helper()
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })
	providers.Register(name, func(store auth.Store) (sshkeydomain.Provider, error) {
		return mock, nil
	})
}

// execAdd creates the ssh-key command, wires up output buffers, runs "add --provider <provider> [args...]",
// and returns what was written to stdout and stderr.
func execAdd(t *testing.T, providerName string, extraArgs ...string) (stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := NewCommand()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	args := append([]string{"add", "--provider", providerName}, extraArgs...)
	cmd.SetArgs(args)
	cmd.Execute()
	return outBuf.String(), errBuf.String()
}

// createTempSSHKey creates a temporary SSH public key file for testing.
func createTempSSHKey(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_sshkey_*.pub")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

func TestAddCommand_WithNameFlag(t *testing.T) {
	keyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyData12345 test@example.com"
	keyPath := createTempSSHKey(t, keyContent)

	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	stdout, stderr := execAdd(t, "mock", keyPath, "--name", "my-test-key")

	if mock.capturedName != "my-test-key" {
		t.Errorf("expected CreateSSHKey called with name 'my-test-key', got %q", mock.capturedName)
	}
	if !strings.Contains(mock.capturedPublicKey, "ssh-ed25519") {
		t.Errorf("expected public key to be passed, got %q", mock.capturedPublicKey)
	}
	if !strings.Contains(stdout, "SSH key added") {
		t.Errorf("expected 'SSH key added' on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "my-test-key") {
		t.Errorf("expected key name on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Uploading SSH key") {
		t.Errorf("expected 'Uploading SSH key' on stderr, got:\n%s", stderr)
	}
}

func TestAddCommand_PublicKeyFlag(t *testing.T) {
	keyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyData12345 test@example.com"

	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	stdout, stderr := execAdd(t, "mock", "--public-key", keyContent, "--name", "pasted-key")

	if mock.capturedName != "pasted-key" {
		t.Errorf("expected CreateSSHKey called with name 'pasted-key', got %q", mock.capturedName)
	}
	if mock.capturedPublicKey != keyContent {
		t.Errorf("expected public key to be passed, got %q", mock.capturedPublicKey)
	}
	if !strings.Contains(stdout, "SSH key added") {
		t.Errorf("expected 'SSH key added' on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Uploading SSH key") {
		t.Errorf("expected 'Uploading SSH key' on stderr, got:\n%s", stderr)
	}
}

func TestAddCommand_PathAndPublicKeyConflict(t *testing.T) {
	keyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyData12345 test@example.com"
	keyPath := createTempSSHKey(t, keyContent)

	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	_, stderr := execAdd(t, "mock", keyPath, "--public-key", keyContent, "--name", "pasted-key")

	if !strings.Contains(stderr, "provide a path or --public-key") {
		t.Errorf("expected conflict error on stderr, got:\n%s", stderr)
	}
	if mock.capturedName != "" {
		t.Errorf("expected CreateSSHKey not to be called, but it was called with name %q", mock.capturedName)
	}
}

func TestAddCommand_RSAKey(t *testing.T) {
	keyContent := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... user@host"
	keyPath := createTempSSHKey(t, keyContent)

	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	stdout, stderr := execAdd(t, "mock", keyPath, "--name", "rsa-key")

	if !strings.Contains(mock.capturedPublicKey, "ssh-rsa") {
		t.Errorf("expected RSA key to be captured, got %q", mock.capturedPublicKey)
	}
	if !strings.Contains(stdout, "SSH key added") {
		t.Errorf("expected success message on stdout, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "done") {
		t.Errorf("expected 'done' message on stderr, got:\n%s", stderr)
	}
}

func TestAddCommand_FileNotFound(t *testing.T) {
	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	_, stderr := execAdd(t, "mock", "/nonexistent/path/key.pub", "--name", "test")

	if !strings.Contains(stderr, "SSH key file not found") {
		t.Errorf("expected 'file not found' error on stderr, got:\n%s", stderr)
	}
	if mock.capturedName != "" {
		t.Errorf("expected CreateSSHKey not to be called, but it was called with name %q", mock.capturedName)
	}
}

func TestAddCommand_InvalidKeyFormat(t *testing.T) {
	keyContent := "this is not a valid ssh key"
	keyPath := createTempSSHKey(t, keyContent)

	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	_, stderr := execAdd(t, "mock", keyPath, "--name", "test")

	if !strings.Contains(stderr, "does not appear to be a valid SSH public key") {
		t.Errorf("expected validation error on stderr, got:\n%s", stderr)
	}
	if mock.capturedName != "" {
		t.Errorf("expected CreateSSHKey not to be called, but it was called")
	}
}

func TestAddCommand_PrivateKeyRejected(t *testing.T) {
	keyContent := "-----BEGIN OPENSSH PRIVATE KEY-----\nfake private key data\n-----END OPENSSH PRIVATE KEY-----"
	keyPath := createTempSSHKey(t, keyContent)

	mock := &sshKeyMockProvider{
		displayName: "Mock",
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	_, stderr := execAdd(t, "mock", keyPath, "--name", "test")

	if !strings.Contains(stderr, "private key") {
		t.Errorf("expected private key rejection on stderr, got:\n%s", stderr)
	}
	if mock.capturedName != "" {
		t.Errorf("expected CreateSSHKey not to be called, but it was called")
	}
}

func TestAddCommand_CreateError(t *testing.T) {
	keyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyData test@example.com"
	keyPath := createTempSSHKey(t, keyContent)

	mock := &sshKeyMockProvider{
		displayName: "Mock",
		createErr:   fmt.Errorf("duplicate key name"),
	}
	registerSSHKeyMockProvider(t, "mock", mock)

	stdout, stderr := execAdd(t, "mock", keyPath, "--name", "duplicate")

	if !strings.Contains(stderr, "duplicate key name") {
		t.Errorf("expected create error on stderr, got:\n%s", stderr)
	}
	if strings.Contains(stdout, "SSH key added") {
		t.Errorf("expected no success message on stdout, got:\n%s", stdout)
	}
}

func TestAddCommand_UnknownProvider(t *testing.T) {
	providers.Reset()
	t.Cleanup(func() { providers.Reset() })

	_, stderr := execAdd(t, "nonexistent", "/fake/path", "--name", "test")

	if !strings.Contains(stderr, "unknown provider") {
		t.Errorf("expected 'unknown provider' error on stderr, got:\n%s", stderr)
	}
}

func TestSuggestKeyName(t *testing.T) {
	tests := []struct {
		path        string
		wantPattern string // Either exact name or "hostname" to indicate hostname fallback
	}{
		{"/home/user/.ssh/id_ed25519.pub", "hostname"}, // Common key, falls back to hostname
		{"/home/user/.ssh/id_rsa.pub", "hostname"},     // Common key, falls back to hostname
		{"/home/user/.ssh/work_key.pub", "work_key"},   // Custom key, returns base name
		{"~/.ssh/laptop.pub", "laptop"},                // Custom key, returns base name
	}

	for _, tc := range tests {
		got := sshkeys.SuggestKeyName(tc.path)
		if tc.wantPattern == "hostname" {
			// For common keys, expect hostname (non-empty and not the original name)
			if got == "" {
				t.Errorf("SuggestKeyName(%q) = empty string, expected hostname", tc.path)
			}
			base := filepath.Base(tc.path)
			baseName := strings.TrimSuffix(base, filepath.Ext(base))
			if got == baseName {
				t.Errorf("SuggestKeyName(%q) = %q, expected hostname fallback not base name", tc.path, got)
			}
		} else {
			// For custom keys, expect the exact name
			if got != tc.wantPattern {
				t.Errorf("SuggestKeyName(%q) = %q, expected %q", tc.path, got, tc.wantPattern)
			}
		}
	}
}
