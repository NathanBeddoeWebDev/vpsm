package server

import (
	"errors"
	"fmt"
	"os"

	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
)

func DeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a server",
		Long: `Delete a server instance from the specified provider.

If --id is not provided, an interactive TUI will let you select a server
from the current list. The TUI shows a summary and asks for confirmation
before deleting.

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
	serverName := ""
	useInteractive := serverID == ""

	if useInteractive {
		selected, err := tui.DeleteServerForm(provider)
		if err != nil {
			if errors.Is(err, tui.ErrDeleteAborted) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Server deletion cancelled.")
				return
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		serverID = selected.ID
		serverName = selected.Name
	}

	if serverName != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Deleting server %q (ID: %s)...\n", serverName, serverID)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "Deleting server %s...\n", serverID)
	}

	if useInteractive {
		accessible := os.Getenv("ACCESSIBLE") != ""
		var deleteErr error
		spinErr := spinner.New().
			Title("Deleting server...").
			Accessible(accessible).
			Output(cmd.ErrOrStderr()).
			Action(func() {
				deleteErr = provider.DeleteServer(serverID)
			}).
			Run()
		if spinErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", spinErr)
			return
		}
		err = deleteErr
	} else {
		err = provider.DeleteServer(serverID)
	}

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error deleting server: %v\n", err)
		return
	}

	if serverName != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Server %q (ID: %s) deleted successfully.\n", serverName, serverID)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Server %s deleted successfully.\n", serverID)
	}
}
