package serverprefs

import "time"

// ServerPrefs holds per-server user preferences.
type ServerPrefs struct {
	ID        int64
	Provider  string
	ServerID  string
	SSHUser   string
	UpdatedAt time.Time
}
