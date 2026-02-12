package auth

import (
	"fmt"
	"os"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"golang.org/x/term"

	"github.com/spf13/cobra"
)

func LoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login <provider>",
		Short: "Store an API token for a provider",
		Long: `Store an API token for a provider using the local keychain.

Example:
  vpsm auth login hetzner`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			provider := strings.TrimSpace(args[0])
			if provider == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "provider is required")
				return
			}

			token, err := cmd.Flags().GetString("token")
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				return
			}

			token = strings.TrimSpace(token)
			store := auth.DefaultStore()

			if token == "" {
				// Interactive mode: use TUI if running in a terminal.
				if term.IsTerminal(int(os.Stdin.Fd())) {
					result, err := tui.RunAuthLogin(provider, store)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
						return
					}
					if result != nil && result.Saved {
						fmt.Fprintf(cmd.OutOrStdout(), "Saved token for provider %s\n", provider)
					} else {
						fmt.Fprintln(cmd.ErrOrStderr(), "Login cancelled.")
					}
					return
				}

				// Fallback for non-terminal (pipe).
				fmt.Fprint(cmd.OutOrStdout(), "Enter API token: ")
				bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					return
				}
				token = strings.TrimSpace(string(bytes))
			}

			if token == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "token cannot be empty")
				return
			}

			if err := store.SetToken(provider, token); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Saved token for provider %s\n", provider)
		},
	}

	cmd.Flags().String("token", "", "API token (optional, overrides prompt)")

	return cmd
}
