package server

import (
	"context"
	"fmt"

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

	ctx := context.Background()
	if err := provider.StartServer(ctx, serverID); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error starting server: %v\n", err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Server %s started successfully.\n", serverID)
}