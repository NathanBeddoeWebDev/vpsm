package auth

import (
	"fmt"
	"os"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/services/auth"

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
				fmt.Fprintln(os.Stderr, "provider is required")
				return
			}

			token, err := cmd.Flags().GetString("token")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}

			token = strings.TrimSpace(token)
			if token == "" {
				fmt.Fprint(os.Stdout, "Enter API token: ")
				bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stdout)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return
				}
				token = strings.TrimSpace(string(bytes))
			}

			if token == "" {
				fmt.Fprintln(os.Stderr, "token cannot be empty")
				return
			}

			store := auth.DefaultStore()
			if err := store.SetToken(provider, token); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}

			fmt.Fprintf(os.Stdout, "Saved token for provider %s\n", provider)
		},
	}

	cmd.Flags().String("token", "", "API token (optional, overrides prompt)")

	return cmd
}
