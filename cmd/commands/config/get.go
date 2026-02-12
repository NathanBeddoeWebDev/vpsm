package config

import (
	"fmt"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/config"
	"nathanbeddoewebdev/vpsm/internal/util"

	"github.com/spf13/cobra"
)

// GetCommand returns the "config get" command.
func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: "Get a persistent configuration value.\n\n" +
			config.KeysHelp() +
			"\nExamples:\n" +
			"  vpsm config get default-provider",
		Args: cobra.ExactArgs(1),
		Run:  runGet,
	}

	return cmd
}

func runGet(cmd *cobra.Command, args []string) {
	key := util.NormalizeKey(args[0])

	spec := config.Lookup(key)
	if spec == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown configuration key %q\n", args[0])
		fmt.Fprintf(cmd.ErrOrStderr(), "Valid keys: %s\n", strings.Join(config.KeyNames(), ", "))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	value := spec.Get(cfg)
	if value == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "not set")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), value)
	}
}
