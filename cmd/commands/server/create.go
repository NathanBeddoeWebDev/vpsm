package server

import (
	"fmt"

	"github.com/spf13/cobra"
)

func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Long:  `Create a new server instance.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd.Flag("provider").Value)
		},
	}

	cmd.Flags().String("provider", "", "The provider to use")

	return cmd
}
