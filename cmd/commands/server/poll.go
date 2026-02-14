package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
)

// actionPollInterval is the delay between successive poll requests.
// It is a variable (not a constant) so tests can override it for speed.
var actionPollInterval = 3 * time.Second

// maxPollAttempts caps how many times we poll before giving up.
// At 3 s intervals this gives ~5 minutes, well beyond the typical
// 10-30 s for a start/stop operation.
const maxPollAttempts = 100

// maxTransientErrors is the number of consecutive non-rate-limit errors
// allowed before the poll loop gives up. This tolerates brief network
// blips without abandoning an operation that is still running server-side.
const maxTransientErrors = 3

// waitForAction blocks until an in-flight action completes and the server
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
func waitForAction(
	ctx context.Context,
	provider domain.Provider,
	action *domain.ActionStatus,
	serverID string,
	targetStatus string,
	w io.Writer,
) error {
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
		if poller, ok := provider.(domain.ActionPoller); ok && action.ID != "" {
			if err := pollByAction(ctx, poller, action.ID, w); err != nil {
				return err
			}
		} else {
			// No action poller — fall through to server-status polling
			// which handles everything.
			return pollByServerStatus(ctx, provider, serverID, targetStatus, w)
		}
	}

	// Action succeeded — verify the server has actually reached the target
	// status. This is necessary because some provider operations (e.g.
	// Hetzner Shutdown) report the action as "success" when the signal is
	// sent, not when the server is fully stopped.
	return confirmServerStatus(ctx, provider, serverID, targetStatus, w)
}

// confirmServerStatus checks that a server has actually reached targetStatus
// after an action completed successfully. It does a single immediate check
// and, if the server hasn't transitioned yet, falls into the standard
// server-status poll loop for the remaining budget.
func confirmServerStatus(
	ctx context.Context,
	provider domain.Provider,
	serverID string,
	targetStatus string,
	w io.Writer,
) error {
	server, err := provider.GetServer(ctx, serverID)
	if err != nil {
		// If we can't check, fall through to the full poll loop which
		// has its own transient error handling.
		return pollByServerStatus(ctx, provider, serverID, targetStatus, w)
	}
	if server != nil && server.Status == targetStatus {
		return nil // already there
	}
	// Server hasn't reached the target yet — poll until it does.
	fmt.Fprintf(w, "  Waiting for server to reach %q status...\n", targetStatus)
	return pollByServerStatus(ctx, provider, serverID, targetStatus, w)
}

// pollByAction polls a provider's action endpoint until the action reaches
// a terminal state ("success" or "error").
//
// This is the preferred strategy when the provider implements
// [domain.ActionPoller] — it yields progress percentages and
// provider-specific error messages.
func pollByAction(
	ctx context.Context,
	poller domain.ActionPoller,
	actionID string,
	w io.Writer,
) error {
	var consecutiveErrors int

	for i := 0; i < maxPollAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(actionPollInterval):
		}

		status, err := poller.PollAction(ctx, actionID)
		if err != nil {
			// Rate-limit errors abort immediately to avoid compounding
			// the problem.
			if errors.Is(err, domain.ErrRateLimited) {
				return fmt.Errorf("polling stopped: %w", err)
			}
			consecutiveErrors++
			if consecutiveErrors >= maxTransientErrors {
				return fmt.Errorf("error polling action (after %d consecutive failures): %w", consecutiveErrors, err)
			}
			fmt.Fprintf(w, "  Transient error, retrying... (%d/%d)\n", consecutiveErrors, maxTransientErrors)
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

	return fmt.Errorf("timed out waiting for action to complete (%d polls)", maxPollAttempts)
}

// pollByServerStatus repeatedly calls [domain.Provider.GetServer] until the
// server's Status matches targetStatus.
//
// This is the generic fallback for providers that do not expose an action
// polling API. It works for any provider since GetServer is part of the
// core [domain.Provider] interface.
func pollByServerStatus(
	ctx context.Context,
	provider domain.Provider,
	serverID string,
	targetStatus string,
	w io.Writer,
) error {
	var consecutiveErrors int

	for i := 0; i < maxPollAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(actionPollInterval):
		}

		server, err := provider.GetServer(ctx, serverID)
		if err != nil {
			if errors.Is(err, domain.ErrRateLimited) {
				return fmt.Errorf("polling stopped: %w", err)
			}
			consecutiveErrors++
			if consecutiveErrors >= maxTransientErrors {
				return fmt.Errorf("error polling server status (after %d consecutive failures): %w", consecutiveErrors, err)
			}
			fmt.Fprintf(w, "  Transient error, retrying... (%d/%d)\n", consecutiveErrors, maxTransientErrors)
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

	return fmt.Errorf("timed out waiting for server to reach %q status (%d polls)", targetStatus, maxPollAttempts)
}
