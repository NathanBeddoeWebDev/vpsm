package config

import (
	"nathanbeddoewebdev/vpsm/internal/config"

	"github.com/spf13/cobra"
)

// NewCommand returns the "config" parent command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage vpsm configuration",
		Long: "View and modify persistent vpsm settings.\n\n" +
			"Configuration is stored at ~/.config/vpsm/config.json.\n\n" +
			config.KeysHelp(),
	}

	cmd.AddCommand(SetCommand())
	cmd.AddCommand(GetCommand())

	return cmd
}
