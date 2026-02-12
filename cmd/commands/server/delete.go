package server

import (
	"fmt"

	"github.com/spf13/cobra"
)

func DeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a server",
		Long:  `Delete a server instance.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("delete called")
		},
	}

	return cmd
}
