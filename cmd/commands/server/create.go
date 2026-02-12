package server

import (
	"fmt"
	"os"

	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/spf13/cobra"
)

func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Long:  `Create a new server instance.`,
		Run: func(cmd *cobra.Command, args []string) {
			providerName := cmd.Flag("provider").Value.String()
			if providerName == "" {
				fmt.Fprintln(os.Stderr, "Error: --provider flag is required")
				return
			}

			provider, err := providers.Get(providerName, auth.DefaultStore())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}

			fmt.Printf("Creating server with provider %s\n", provider.GetDisplayName())
		},
	}

	return cmd
}
