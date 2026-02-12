package sshkeys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns the default SSH public key path.
func DefaultPath() string {
	return "~/.ssh/id_ed25519.pub"
}

// CommonPaths returns a list of common public key paths.
func CommonPaths() []string {
	return []string{
		"~/.ssh/id_ed25519.pub",
		"~/.ssh/id_rsa.pub",
		"~/.ssh/id_ecdsa.pub",
	}
}

// ExpandHomePath expands a leading ~/ to the user's home directory.
func ExpandHomePath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

// ReadAndValidatePublicKey reads a public key from disk and validates it.
func ReadAndValidatePublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH key file: %w", err)
	}

	publicKey := strings.TrimSpace(string(data))
	if publicKey == "" {
		return "", fmt.Errorf("SSH key file is empty")
	}

	return ValidatePublicKey(publicKey)
}

// ValidatePublicKey performs basic validation on an SSH public key string.
func ValidatePublicKey(publicKey string) (string, error) {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" {
		return "", fmt.Errorf("SSH key cannot be empty")
	}

	if strings.Contains(publicKey, "PRIVATE KEY") {
		return "", fmt.Errorf("file appears to contain a private key; please provide the public key (.pub file)")
	}

	validPrefixes := []string{"ssh-rsa", "ssh-ed25519", "ssh-dss", "ecdsa-sha2-"}
	isValid := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(publicKey, prefix) {
			isValid = true
			break
		}
	}

	if !isValid {
		return "", fmt.Errorf("file does not appear to be a valid SSH public key (expected ssh-rsa, ssh-ed25519, or ecdsa-sha2-*)")
	}

	return publicKey, nil
}

// DefaultKeyName returns a safe default key name based on hostname.
func DefaultKeyName() string {
	if hostname, err := os.Hostname(); err == nil {
		name := strings.TrimSpace(hostname)
		if name != "" {
			return name
		}
	}

	return "ssh-key"
}

// SuggestKeyName suggests a key name based on the path.
func SuggestKeyName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	if name == "id_ed25519" || name == "id_rsa" || name == "id_ecdsa" {
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	}

	return name
}
