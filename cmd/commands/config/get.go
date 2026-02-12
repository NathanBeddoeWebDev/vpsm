package config

import (
	"fmt"
	"os"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/config"
	"nathanbeddoewebdev/vpsm/internal/tui"
	"nathanbeddoewebdev/vpsm/internal/util"

	"golang.org/x/term"

	"github.com/spf13/cobra"
)

// GetCommand returns the "config get" command.
func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Get a configuration value",
		Long: "Get a persistent configuration value.\n\n" +
			"If no key is provided and running in a terminal, opens an interactive\n" +
			"config viewer where you can browse and edit all settings.\n\n" +
			config.KeysHelp() +
			"\nExamples:\n" +
			"  vpsm config get                   # interactive viewer\n" +
			"  vpsm config get default-provider   # print a single value",
		Args: cobra.MaximumNArgs(1),
		Run:  runGet,
	}

	return cmd
}

func runGet(cmd *cobra.Command, args []string) {
	// No args: open interactive config viewer.
	if len(args) == 0 {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			if err := tui.RunConfigView(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			}
			return
		}

		// Non-interactive: list all values.
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		for _, spec := range config.Keys {
			value := spec.Get(cfg)
			if value == "" {
				value = "(not set)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", spec.Name, value)
		}
		return
	}

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
