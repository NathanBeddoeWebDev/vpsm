package config

import (
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/config"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/util"

	"github.com/spf13/cobra"
)

// SetCommand returns the "config set" command.
func SetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a persistent configuration value.

Supported keys:
  default-provider   The provider to use when --provider is not specified.

Examples:
  vpsm config set default-provider hetzner`,
		Args: cobra.ExactArgs(2),
		Run:  runSet,
	}

	return cmd
}

func runSet(cmd *cobra.Command, args []string) {
	key := util.NormalizeKey(args[0])
	value := args[1]

	switch key {
	case "default-provider":
		setDefaultProvider(cmd, value)
	default:
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown configuration key %q\n", args[0])
	}
}

func setDefaultProvider(cmd *cobra.Command, name string) {
	normalized := util.NormalizeKey(name)

	// Validate that the provider is registered.
	known := providers.List()
	found := false
	for _, p := range known {
		if p == normalized {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown provider %q\n", name)
		fmt.Fprintf(cmd.ErrOrStderr(), "Registered providers: %v\n", known)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	cfg.DefaultProvider = normalized
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Default provider set to %q\n", normalized)
}
