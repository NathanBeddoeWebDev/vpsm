package config

import (
	"github.com/spf13/cobra"
)

// NewCommand returns the "config" parent command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage vpsm configuration",
		Long: `View and modify persistent vpsm settings.

Configuration is stored at ~/.config/vpsm/config.json.`,
	}

	cmd.AddCommand(SetCommand())
	cmd.AddCommand(GetCommand())

	return cmd
}
