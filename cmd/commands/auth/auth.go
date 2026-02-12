package auth

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication for providers",
		Long: `Manage authentication for providers.

Use this command group to log in and store API tokens securely.`,
	}

	cmd.AddCommand(LoginCommand())
	cmd.AddCommand(StatusCommand())

	return cmd
}
