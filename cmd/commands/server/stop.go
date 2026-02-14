package server

import (
	"context"
	"fmt"

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

	ctx := context.Background()
	if err := provider.StopServer(ctx, serverID); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error stopping server: %v\n", err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Server %s stop initiated successfully.\n", serverID)
}