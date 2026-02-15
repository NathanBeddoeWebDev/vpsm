package actionstore

import "time"

// ActionRecord represents a persisted action. It extends
// domain.ActionStatus with metadata needed to resume polling
// after a CLI restart.
type ActionRecord struct {
	// ID is the auto-increment primary key (assigned on insert).
	ID int64

	// ActionID is the provider-specific action identifier used for polling.
	ActionID string

	// Provider is the name of the cloud provider (e.g. "hetzner").
	Provider string

	// ServerID is the ID of the server being acted upon.
	ServerID string

	// ServerName is the human-readable server name (for display).
	ServerName string

	// Command describes the operation, e.g. "start_server", "stop_server".
	Command string

	// TargetStatus is the expected server status when the action completes
	// (e.g. "running", "off").
	TargetStatus string

	// Status is the current state: "running", "success", or "error".
	Status string

	// Progress is a percentage (0–100).
	Progress int

	// ErrorMessage contains a human-readable explanation when Status is "error".
	ErrorMessage string

	// CreatedAt is when the action was first recorded.
	CreatedAt time.Time

	// UpdatedAt is the last time the record was modified.
	UpdatedAt time.Time
}
