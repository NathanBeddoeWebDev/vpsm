package server

import (
	"context"
	"fmt"
	"os"

	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func DeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a server",
		Long: `Delete a server instance from the specified provider.

If --id is not provided, an interactive TUI will let you select a server
from the current list. The TUI shows a summary and asks for confirmation
before deleting. Requires a terminal; use --id for scripting.

Examples:
  # Interactive mode (TUI)
  vpsm server delete --provider hetzner

  # Non-interactive (scripting)
  vpsm server delete --provider hetzner --id 12345`,
		Run: runDelete,
	}

	cmd.Flags().String("id", "", "Server ID to delete (skips interactive selection)")

	return cmd
}

func runDelete(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	serverID, _ := cmd.Flags().GetString("id")

	if serverID == "" {
		// Interactive mode requires a terminal.
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Fprintln(cmd.ErrOrStderr(), "Error: --id is required when not running in a terminal")
			return
		}

		result, err := tui.RunServerDelete(provider, providerName, nil)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		if result == nil || !result.Confirmed {
			fmt.Fprintln(cmd.ErrOrStderr(), "Server deletion cancelled.")
			return
		}

		serverID = result.Server.ID
		fmt.Fprintf(cmd.ErrOrStderr(), "Deleting server %q (ID: %s)...\n", result.Server.Name, serverID)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "Deleting server %s...\n", serverID)
	}

	ctx := context.Background()
	if err := provider.DeleteServer(ctx, serverID); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error deleting server: %v\n", err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Server %s deleted successfully.\n", serverID)
}
