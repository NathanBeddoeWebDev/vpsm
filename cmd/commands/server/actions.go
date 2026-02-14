package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"text/tabwriter"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/store"

	"github.com/spf13/cobra"
)

// ActionsCommand returns a cobra.Command that lists and resumes tracked actions.
func ActionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "List or resume tracked actions",
		Long: `Show actions that were started by previous CLI invocations.

By default, only pending (in-flight) actions are shown. Use --all to
include completed and failed actions as well.

If a previous start/stop command was interrupted (Ctrl+C), the action
remains tracked locally. Use --resume to resume polling all pending
actions until they complete.

Examples:
  vpsm server actions                     # Show pending actions
  vpsm server actions --all               # Show all recent actions
  vpsm server actions --resume            # Resume polling pending actions`,
		Run: runActions,
	}

	cmd.Flags().Bool("all", false, "Show all recent actions, not just pending")
	cmd.Flags().Bool("resume", false, "Resume polling all pending actions")

	return cmd
}

func runActions(cmd *cobra.Command, args []string) {
	showAll, _ := cmd.Flags().GetBool("all")
	resume, _ := cmd.Flags().GetBool("resume")

	s, err := store.Open()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error opening action store: %v\n", err)
		return
	}
	defer s.Close()

	if resume {
		resumePendingActions(cmd, s)
		return
	}

	var actions []store.ActionRecord
	if showAll {
		actions, err = s.ListRecent(20)
	} else {
		actions, err = s.ListPending()
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error listing actions: %v\n", err)
		return
	}

	if len(actions) == 0 {
		if showAll {
			fmt.Fprintln(cmd.OutOrStdout(), "No recent actions.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No pending actions.")
		}
		return
	}

	printActions(cmd, actions)

	// Hint about --resume when there are pending actions.
	if !showAll {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nUse --resume to resume polling these actions.\n")
	}
}

func printActions(cmd *cobra.Command, actions []store.ActionRecord) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tPROVIDER\tSERVER\tCOMMAND\tSTATUS\tAGE\n")

	for _, a := range actions {
		age := time.Since(a.CreatedAt).Truncate(time.Second)
		ageStr := formatDuration(age)

		status := a.Status
		if a.Status == "error" && a.ErrorMessage != "" {
			status = fmt.Sprintf("error: %s", truncate(a.ErrorMessage, 40))
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Provider, a.ServerID, a.Command, status, ageStr)
	}

	w.Flush()
}

func resumePendingActions(cmd *cobra.Command, s *store.SQLiteStore) {
	pending, err := s.ListPending()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error listing pending actions: %v\n", err)
		return
	}

	if len(pending) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No pending actions to resume.")
		return
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Resuming %d pending action(s)...\n\n", len(pending))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	for _, record := range pending {
		resumeAction(ctx, cmd, s, record)
	}
}

func resumeAction(ctx context.Context, cmd *cobra.Command, s *store.SQLiteStore, record store.ActionRecord) {
	providerName := record.Provider
	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "[%s] Error resolving provider %q: %v\n", record.ServerID, providerName, err)
		return
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "[%s] Resuming %s (action %s)...\n", record.ServerID, record.Command, record.ActionID)

	// Reconstruct the ActionStatus to reuse existing polling logic.
	action := &domain.ActionStatus{
		ID:      record.ActionID,
		Status:  domain.ActionStatusRunning,
		Command: record.Command,
	}

	if err := waitForAction(ctx, provider, action, record.ServerID, record.TargetStatus, cmd.ErrOrStderr()); err != nil {
		record.Status = domain.ActionStatusError
		record.ErrorMessage = err.Error()
		s.Save(&record)
		fmt.Fprintf(cmd.ErrOrStderr(), "[%s] Error: %v\n", record.ServerID, err)
		return
	}

	record.Status = domain.ActionStatusSuccess
	record.Progress = 100
	s.Save(&record)

	verb := "completed"
	if record.Command == "start_server" {
		verb = "started"
	} else if record.Command == "stop_server" {
		verb = "stopped"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Server %s %s successfully.\n", record.ServerID, verb)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
