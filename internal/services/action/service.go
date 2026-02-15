package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"nathanbeddoewebdev/vpsm/internal/actionstore"
	"nathanbeddoewebdev/vpsm/internal/domain"
)

// PollInterval is the delay between successive poll requests.
// Exported as a variable so tests can override it for speed.
var PollInterval = 3 * time.Second

// MaxPollAttempts caps how many times we poll before giving up.
// At 3 s intervals this gives ~5 minutes, well beyond the typical
// 10-30 s for a start/stop operation.
// Exported as a variable for test flexibility and consistency with PollInterval.
var MaxPollAttempts = 100

// MaxTransientErrors is the number of consecutive non-rate-limit errors
// allowed before the poll loop gives up. This tolerates brief network
// blips without abandoning an operation that is still running server-side.
// Exported as a variable for test flexibility and consistency with PollInterval.
var MaxTransientErrors = 3

// Service encapsulates action tracking logic, including persistence
// and polling across providers.
type Service struct {
	repo         actionstore.ActionRepository
	provider     domain.Provider
	providerName string
}

// NewService creates a new action service.
func NewService(provider domain.Provider, providerName string, repo actionstore.ActionRepository) *Service {
	return &Service{
		repo:         repo,
		provider:     provider,
		providerName: providerName,
	}
}

// Close releases repository resources.
func (s *Service) Close() error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Close()
}

// SaveRecord persists an action record. It is a thin wrapper around the repository.
func (s *Service) SaveRecord(record *actionstore.ActionRecord) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Save(record)
}

// TrackAction persists a new action record for the given action.
// If persistence fails, it returns nil so callers can proceed without tracking.
func (s *Service) TrackAction(serverID, serverName string, action *domain.ActionStatus, command, targetStatus string) *actionstore.ActionRecord {
	if s.repo == nil || action == nil {
		return nil
	}

	record := &actionstore.ActionRecord{
		ActionID:     action.ID,
		Provider:     s.providerName,
		ServerID:     serverID,
		ServerName:   serverName,
		Command:      command,
		TargetStatus: targetStatus,
		Status:       action.Status,
		Progress:     action.Progress,
		ErrorMessage: action.ErrorMessage,
	}

	if err := s.repo.Save(record); err != nil {
		return nil
	}

	// Opportunistically clean up old completed records.
	_, _ = s.repo.DeleteOlderThan(24 * time.Hour)

	return record
}

// FinalizeAction updates a tracked action record with its terminal status.
func (s *Service) FinalizeAction(record *actionstore.ActionRecord, status, errMsg string) {
	if s.repo == nil || record == nil {
		return
	}

	record.Status = status
	record.ErrorMessage = errMsg
	if status == domain.ActionStatusSuccess {
		record.Progress = 100
	}

	_ = s.repo.Save(record)
}

// ListPending returns all pending action records.
func (s *Service) ListPending() ([]actionstore.ActionRecord, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("actions: repository unavailable")
	}
	return s.repo.ListPending()
}

// ListRecent returns the most recent n action records.
func (s *Service) ListRecent(n int) ([]actionstore.ActionRecord, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("actions: repository unavailable")
	}
	return s.repo.ListRecent(n)
}

// Cleanup removes old completed/errored action records.
func (s *Service) Cleanup(maxAge time.Duration) (int64, error) {
	if s.repo == nil {
		return 0, fmt.Errorf("actions: repository unavailable")
	}
	return s.repo.DeleteOlderThan(maxAge)
}

// ResumeAction resumes polling for a persisted action record.
func (s *Service) ResumeAction(ctx context.Context, record *actionstore.ActionRecord, w io.Writer) error {
	if s.provider == nil {
		return fmt.Errorf("actions: provider unavailable")
	}
	if record == nil {
		return fmt.Errorf("actions: record is nil")
	}

	action := &domain.ActionStatus{
		ID:      record.ActionID,
		Status:  domain.ActionStatusRunning,
		Command: record.Command,
	}

	if err := s.WaitForAction(ctx, action, record.ServerID, record.TargetStatus, w); err != nil {
		record.Status = domain.ActionStatusError
		record.ErrorMessage = err.Error()
		if s.repo != nil {
			_ = s.repo.Save(record)
		}
		return err
	}

	record.Status = domain.ActionStatusSuccess
	record.Progress = 100
	record.ErrorMessage = ""
	if s.repo != nil {
		_ = s.repo.Save(record)
	}

	return nil
}

// WaitForAction blocks until an in-flight action completes and the server
// reaches the expected targetStatus.
//
// Strategy selection:
//   - If the provider implements [domain.ActionPoller], the action is tracked
//     by its ID — this gives precise progress percentages and error messages.
//     After the action succeeds, the server status is verified via
//     [domain.Provider.GetServer] because some operations (e.g. Hetzner's
//     graceful Shutdown) report the action as "success" once the signal is
//     sent, before the server has actually powered off.
//   - Otherwise, [domain.Provider.GetServer] is polled until the server's
//     Status matches targetStatus. This generic fallback works for any
//     provider, even those that have no concept of "actions".
//
// If the action is already complete when called (Status == "success" or
// "error"), the server status is still verified before returning.
//
// Progress messages are written to w (typically cmd.ErrOrStderr()).
func (s *Service) WaitForAction(
	ctx context.Context,
	action *domain.ActionStatus,
	serverID string,
	targetStatus string,
	w io.Writer,
) error {
	if s.provider == nil {
		return fmt.Errorf("actions: provider unavailable")
	}
	if action == nil {
		return nil
	}

	// Handle action errors immediately — no need to check server status.
	if action.Status == domain.ActionStatusError {
		if action.ErrorMessage != "" {
			return fmt.Errorf("action failed: %s", action.ErrorMessage)
		}
		return fmt.Errorf("action failed")
	}

	// If the action is still running, poll until it completes.
	if action.Status != domain.ActionStatusSuccess {
		if poller, ok := s.provider.(domain.ActionPoller); ok && action.ID != "" {
			if err := s.pollByAction(ctx, poller, action.ID, w); err != nil {
				return err
			}
		} else {
			// No action poller — fall through to server-status polling
			// which handles everything.
			return s.pollByServerStatus(ctx, serverID, targetStatus, w)
		}
	}

	// Action succeeded — verify the server has actually reached the target
	// status. This is necessary because some provider operations (e.g.
	// Hetzner Shutdown) report the action as "success" when the signal is
	// sent, not when the server is fully stopped.
	return s.confirmServerStatus(ctx, serverID, targetStatus, w)
}

// confirmServerStatus checks that a server has actually reached targetStatus
// after an action completed successfully. It does a single immediate check
// and, if the server hasn't transitioned yet, falls into the standard
// server-status poll loop for the remaining budget.
func (s *Service) confirmServerStatus(
	ctx context.Context,
	serverID string,
	targetStatus string,
	w io.Writer,
) error {
	server, err := s.provider.GetServer(ctx, serverID)
	if err != nil {
		// If we can't check, fall through to the full poll loop which
		// has its own transient error handling.
		return s.pollByServerStatus(ctx, serverID, targetStatus, w)
	}
	if server != nil && server.Status == targetStatus {
		return nil // already there
	}
	// Server hasn't reached the target yet — poll until it does.
	fmt.Fprintf(w, "  Waiting for server to reach %q status...\n", targetStatus)
	return s.pollByServerStatus(ctx, serverID, targetStatus, w)
}

// pollByAction polls a provider's action endpoint until the action reaches
// a terminal state ("success" or "error").
//
// This is the preferred strategy when the provider implements
// [domain.ActionPoller] — it yields progress percentages and
// provider-specific error messages.
func (s *Service) pollByAction(
	ctx context.Context,
	poller domain.ActionPoller,
	actionID string,
	w io.Writer,
) error {
	var consecutiveErrors int

	for i := 0; i < MaxPollAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(PollInterval):
		}

		status, err := poller.PollAction(ctx, actionID)
		if err != nil {
			// Rate-limit errors abort immediately to avoid compounding
			// the problem.
			if errors.Is(err, domain.ErrRateLimited) {
				return fmt.Errorf("polling stopped: %w", err)
			}
			consecutiveErrors++
			if consecutiveErrors >= MaxTransientErrors {
				return fmt.Errorf("error polling action (after %d consecutive failures): %w", consecutiveErrors, err)
			}
			fmt.Fprintf(w, "  Transient error, retrying... (%d/%d)\n", consecutiveErrors, MaxTransientErrors)
			continue
		}
		consecutiveErrors = 0

		switch status.Status {
		case domain.ActionStatusSuccess:
			return nil
		case domain.ActionStatusError:
			if status.ErrorMessage != "" {
				return fmt.Errorf("action failed: %s", status.ErrorMessage)
			}
			return fmt.Errorf("action failed")
		default:
			// Still running -- log progress and continue.
			if status.Progress > 0 {
				fmt.Fprintf(w, "  Progress: %d%%\n", status.Progress)
			}
		}
	}

	return fmt.Errorf("timed out waiting for action to complete (%d polls)", MaxPollAttempts)
}

// pollByServerStatus repeatedly calls [domain.Provider.GetServer] until the
// server's Status matches targetStatus.
//
// This is the generic fallback for providers that do not expose an action
// polling API. It works for any provider since GetServer is part of the
// core [domain.Provider] interface.
func (s *Service) pollByServerStatus(
	ctx context.Context,
	serverID string,
	targetStatus string,
	w io.Writer,
) error {
	var consecutiveErrors int

	for i := 0; i < MaxPollAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(PollInterval):
		}

		server, err := s.provider.GetServer(ctx, serverID)
		if err != nil {
			if errors.Is(err, domain.ErrRateLimited) {
				return fmt.Errorf("polling stopped: %w", err)
			}
			consecutiveErrors++
			if consecutiveErrors >= MaxTransientErrors {
				return fmt.Errorf("error polling server status (after %d consecutive failures): %w", consecutiveErrors, err)
			}
			fmt.Fprintf(w, "  Transient error, retrying... (%d/%d)\n", consecutiveErrors, MaxTransientErrors)
			continue
		}
		consecutiveErrors = 0

		if server == nil {
			return fmt.Errorf("server %q disappeared while polling", serverID)
		}

		if server.Status == targetStatus {
			return nil
		}

		// Show the server's transitional status (e.g. "starting", "stopping").
		fmt.Fprintf(w, "  Status: %s\n", server.Status)
	}

	return fmt.Errorf("timed out waiting for server to reach %q status (%d polls)", targetStatus, MaxPollAttempts)
}
