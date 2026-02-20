package auth

import (
	"fmt"
	"os"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func LoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login <provider>",
		Short: "Store an API token for a provider",
		Long: `Store an API token for a provider using the local keychain.

Example:
  vpsm auth login hetzner`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.TrimSpace(args[0])
			if provider == "" {
				return fmt.Errorf("provider is required")
			}

			token, err := cmd.Flags().GetString("token")
			if err != nil {
				return err
			}

			token = strings.TrimSpace(token)
			store := auth.DefaultStore()

			if token == "" {
				// Interactive mode: use TUI if running in a terminal.
				if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
					result, err := tui.RunAuthLogin(provider, store)
					if err != nil {
						return fmt.Errorf("auth login failed: %w", err)
					}
					if result != nil && result.Saved {
						fmt.Fprintf(cmd.OutOrStdout(), "Saved token for provider %s\n", provider)
					} else {
						fmt.Fprintln(cmd.ErrOrStderr(), "Login cancelled.")
					}
					return nil
				}

				return fmt.Errorf("non-interactive login requires --token")
			}

			if token == "" {
				return fmt.Errorf("token cannot be empty")
			}

			if err := store.SetToken(provider, token); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Saved token for provider %s\n", provider)
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().String("token", "", "API token (optional, overrides prompt)")

	return cmd
}
