package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/store"

	"github.com/spf13/cobra"
)

// StartCommand returns a cobra.Command that powers on a server.
func StartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a server",
		Long: `Power on a stopped server instance from the specified provider.

The command waits for the operation to complete by polling the provider
for action progress (or falling back to server-status polling).

The action is persisted locally so that if the CLI is interrupted, the
action can be resumed with "vpsm server actions --resume".

Examples:
  vpsm server start --provider hetzner --id 12345`,
		Run: runStart,
	}

	cmd.Flags().String("id", "", "Server ID to start (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	serverID, _ := cmd.Flags().GetString("id")

	fmt.Fprintf(cmd.ErrOrStderr(), "Starting server %s...\n", serverID)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	action, err := provider.StartServer(ctx, serverID)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error starting server: %v\n", err)
		return
	}

	// Persist the action so it can be resumed if the CLI is interrupted.
	record := trackAction(providerName, serverID, action, "start_server", "running")

	if err := waitForAction(ctx, provider, action, serverID, "running", cmd.ErrOrStderr()); err != nil {
		finalizeAction(record, domain.ActionStatusError, err.Error())
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	finalizeAction(record, domain.ActionStatusSuccess, "")
	fmt.Fprintf(cmd.OutOrStdout(), "Server %s started successfully.\n", serverID)
}

// trackAction persists a new action record to the store. If the store
// cannot be opened the action proceeds without persistence — the CLI
// should not fail just because local tracking is unavailable.
func trackAction(providerName, serverID string, action *domain.ActionStatus, command, targetStatus string) *store.ActionRecord {
	if action == nil {
		return nil
	}

	s, err := store.Open()
	if err != nil {
		return nil
	}

	record := &store.ActionRecord{
		ActionID:     action.ID,
		Provider:     providerName,
		ServerID:     serverID,
		Command:      command,
		TargetStatus: targetStatus,
		Status:       action.Status,
		Progress:     action.Progress,
		ErrorMessage: action.ErrorMessage,
	}

	if err := s.Save(record); err != nil {
		return nil
	}

	// Opportunistically clean up old completed records.
	s.DeleteOlderThan(24 * time.Hour)

	return record
}

// finalizeAction updates a tracked action record with its terminal status.
func finalizeAction(record *store.ActionRecord, status, errMsg string) {
	if record == nil {
		return
	}

	s, err := store.Open()
	if err != nil {
		return
	}

	record.Status = status
	record.ErrorMessage = errMsg
	if status == domain.ActionStatusSuccess {
		record.Progress = 100
	}

	s.Save(record)
}
