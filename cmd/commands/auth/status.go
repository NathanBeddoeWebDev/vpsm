package auth

import (
	"errors"
	"fmt"
	"os"

	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

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
			providerNames := providers.List()

			if len(providerNames) == 0 {
				fmt.Fprintln(os.Stdout, "No providers registered.")
				return
			}

			for _, provider := range providerNames {
				_, err := store.GetToken(provider)
				switch {
				case err == nil:
					fmt.Fprintf(os.Stdout, "%s: logged in ✅\n", provider)
				case errors.Is(err, auth.ErrTokenNotFound):
					fmt.Fprintf(os.Stdout, "%s: not logged in\n", provider)
				default:
					fmt.Fprintf(os.Stdout, "%s: error (%v)\n", provider, err)
				}
			}
		},
	}

	return cmd
}
