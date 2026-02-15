// Package serverprefs provides a service layer for per-server user preferences.
package serverprefs

import (
	"nathanbeddoewebdev/vpsm/internal/serverprefs"
)

// Service wraps the serverprefs repository with higher-level operations.
type Service struct {
	repo serverprefs.Repository
}

// NewService creates a new preferences service.
func NewService(repo serverprefs.Repository) *Service {
	return &Service{repo: repo}
}

// Close releases repository resources.
func (s *Service) Close() error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Close()
}

// GetSSHUser returns the stored SSH username for a server, or "" if not set.
func (s *Service) GetSSHUser(provider, serverID string) string {
	if s.repo == nil {
		return ""
	}
	prefs, err := s.repo.Get(provider, serverID)
	if err != nil || prefs == nil {
		return ""
	}
	return prefs.SSHUser
}

// SetSSHUser persists the SSH username for a server (best-effort).
func (s *Service) SetSSHUser(provider, serverID, username string) {
	if s.repo == nil {
		return
	}
	prefs := &serverprefs.ServerPrefs{
		Provider: provider,
		ServerID: serverID,
		SSHUser:  username,
	}
	_ = s.repo.Save(prefs)
}
