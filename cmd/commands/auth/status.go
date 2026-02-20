package auth

import (
	"errors"
	"fmt"
	"os"

	providernames "nathanbeddoewebdev/vpsm/internal/platform/providers/names"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"golang.org/x/term"

	"github.com/spf13/cobra"
)

func StatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status for providers",
		Long: `Show which providers have stored API tokens.

Example:
  vpsm auth status`,
		Run: func(cmd *cobra.Command, args []string) {
			store := auth.DefaultStore()

			// Use TUI in interactive terminal.
			if term.IsTerminal(int(os.Stdout.Fd())) {
				if err := tui.RunAuthStatus(store); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				}
				return
			}

			// Non-interactive fallback.
			providerNames := providernames.List()

			if len(providerNames) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No providers registered.")
				return
			}

			for _, provider := range providerNames {
				_, err := store.GetToken(provider)
				switch {
				case err == nil:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: logged in\n", provider)
				case errors.Is(err, auth.ErrTokenNotFound):
					fmt.Fprintf(cmd.OutOrStdout(), "%s: not logged in\n", provider)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: error (%v)\n", provider, err)
				}
			}
		},
	}

	return cmd
}
