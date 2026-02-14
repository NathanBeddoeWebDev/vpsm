package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/spf13/cobra"
)

// StopCommand returns a cobra.Command that gracefully shuts down a server.
func StopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running server",
		Long: `Gracefully shut down a running server instance.

The command waits for the operation to complete by polling the provider.
If the provider supports action tracking (e.g. Hetzner), progress is
reported via the action API. Otherwise, the server's status is polled
until it reaches "off".

The action is persisted locally so that if the CLI is interrupted, the
action can be resumed with "vpsm server actions --resume".

Examples:
  vpsm server stop --provider hetzner --id 12345`,
		Run: runStop,
	}

	cmd.Flags().String("id", "", "Server ID to stop (required)")
	cmd.MarkFlagRequired("id")

	return cmd
}

func runStop(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	serverID, _ := cmd.Flags().GetString("id")

	fmt.Fprintf(cmd.ErrOrStderr(), "Stopping server %s...\n", serverID)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	action, err := provider.StopServer(ctx, serverID)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error stopping server: %v\n", err)
		return
	}

	// Persist the action so it can be resumed if the CLI is interrupted.
	record := trackAction(providerName, serverID, action, "stop_server", "off")

	if err := waitForAction(ctx, provider, action, serverID, "off", cmd.ErrOrStderr()); err != nil {
		finalizeAction(record, domain.ActionStatusError, err.Error())
		fmt.Fprintf(cmd.ErrOrStderr(), "Error waiting for server to stop: %v\n", err)
		return
	}

	finalizeAction(record, domain.ActionStatusSuccess, "")
	fmt.Fprintf(cmd.OutOrStdout(), "Server %s stop initiated successfully.\n", serverID)
}
