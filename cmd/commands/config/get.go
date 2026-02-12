package config

import (
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/config"
	"nathanbeddoewebdev/vpsm/internal/util"

	"github.com/spf13/cobra"
)

// GetCommand returns the "config get" command.
func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a persistent configuration value.

Supported keys:
  default-provider   The provider to use when --provider is not specified.

Examples:
  vpsm config get default-provider`,
		Args: cobra.ExactArgs(1),
		Run:  runGet,
	}

	return cmd
}

func runGet(cmd *cobra.Command, args []string) {
	key := util.NormalizeKey(args[0])

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	switch key {
	case "default-provider":
		if cfg.DefaultProvider == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "not set")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), cfg.DefaultProvider)
		}
	default:
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown configuration key %q\n", args[0])
	}
}
