package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

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

	if err := waitForAction(ctx, provider, action, serverID, "running", cmd.ErrOrStderr()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Server %s started successfully.\n", serverID)
}
