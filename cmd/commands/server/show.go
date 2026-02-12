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

// ShowCommand returns a cobra.Command that displays details for a single server.
func ShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a server",
		Long: `Display detailed information about a single server.

If --id is not provided, an interactive TUI will let you select a server
from the current list. Requires a terminal; use --id for scripting.

Examples:
  # Interactive mode (TUI)
  vpsm server show --provider hetzner

  # Non-interactive with table output
  vpsm server show --provider hetzner --id 12345

  # JSON output for scripting
  vpsm server show --provider hetzner --id 12345 -o json`,
		Run: runShow,
	}

	cmd.Flags().String("id", "", "Server ID to show (skips interactive selection)")
	cmd.Flags().StringP("output", "o", "table", "Output format: table or json")

	return cmd
}

func runShow(cmd *cobra.Command, args []string) {
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

		result, err := tui.RunServerShow(provider, providerName, "")
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		if result == nil {
			return
		}

		// Handle action from the detail view.
		if result.Action == "delete" && result.Server != nil {
			runDeleteFromList(cmd, provider, providerName, result.Server)
		}
		return
	}

	// Non-interactive mode: fetch and display directly.
	ctx := context.Background()
	server, err := provider.GetServer(ctx, serverID)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error fetching server: %v\n", err)
		return
	}

	output, _ := cmd.Flags().GetString("output")
	switch output {
	case "json":
		printServerJSON(cmd, server)
	default:
		printServerDetail(cmd, server)
	}
}
