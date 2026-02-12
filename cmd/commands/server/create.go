package server

import (
	"fmt"
	"nathanbeddoewebdev/vpsm/internal/providers"

	"github.com/spf13/cobra"
)

func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Long:  `Create a new server instance.`,
		Run: func(cmd *cobra.Command, args []string) {
			provider := cmd.Flag("provider").Value.String()
			Provider, err := providers.Get(provider)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("Creating server with provider %s\n", Provider.GetDisplayName())
		},
	}

	cmd.Flags().String("provider", "", "The provider to use")

	return cmd
}
