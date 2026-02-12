package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
)

// ShowCommand returns a cobra.Command that displays details for a single server.
func ShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a server",
		Long: `Display detailed information about a single server.

If --id is not provided, an interactive TUI will let you select a server
from the current list.

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
	useInteractive := serverID == ""

	var server *domain.Server

	if useInteractive {
		selected, err := tui.ShowServerForm(provider)
		if err != nil {
			if errors.Is(err, tui.ErrShowAborted) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Server selection cancelled.")
				return
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}

		// Fetch the full server details via GetServer for the freshest data.
		accessible := os.Getenv("ACCESSIBLE") != ""
		var getErr error
		spinErr := spinner.New().
			Title("Fetching server details...").
			Accessible(accessible).
			Output(cmd.ErrOrStderr()).
			Action(func() {
				server, getErr = provider.GetServer(context.Background(), selected.ID)
			}).
			Run()
		if spinErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", spinErr)
			return
		}
		if getErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error fetching server: %v\n", getErr)
			return
		}
	} else {
		ctx := context.Background()
		server, err = provider.GetServer(ctx, serverID)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error fetching server: %v\n", err)
			return
		}
	}

	output, _ := cmd.Flags().GetString("output")
	switch output {
	case "json":
		printServerJSON(cmd, server)
	default:
		printServerDetail(cmd, server)
	}
}
