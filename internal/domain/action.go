package domain

// ActionStatus represents the state of an in-flight provider action
// such as starting or stopping a server. Providers return this from
// operations that are asynchronous, allowing callers to poll for
// completion.
type ActionStatus struct {
	// ID is the provider-specific action identifier, used for polling.
	ID string `json:"id"`

	// Status is the current state of the action.
	// Known values: "running", "success", "error".
	Status string `json:"status"`

	// Progress is a percentage (0–100) indicating how far along the
	// action is. Not all providers supply meaningful progress values.
	Progress int `json:"progress"`

	// Command describes the operation, e.g. "start_server", "stop_server".
	Command string `json:"command,omitempty"`

	// ErrorMessage contains a human-readable explanation when Status is "error".
	ErrorMessage string `json:"error_message,omitempty"`
}

// Action status constants mirror the values common across cloud providers.
const (
	ActionStatusRunning = "running"
	ActionStatusSuccess = "success"
	ActionStatusError   = "error"
)

// IsComplete reports whether the action has finished, regardless of outcome.
func (a *ActionStatus) IsComplete() bool {
	return a.Status == ActionStatusSuccess || a.Status == ActionStatusError
}