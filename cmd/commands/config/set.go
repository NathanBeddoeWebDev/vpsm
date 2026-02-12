package config

import (
	"fmt"
	"strings"

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
		Long: "Set a persistent configuration value.\n\n" +
			config.KeysHelp() +
			"\nExamples:\n" +
			"  vpsm config set default-provider hetzner",
		Args: cobra.ExactArgs(2),
		Run:  runSet,
	}

	return cmd
}

// validators maps key names to optional pre-save validation functions.
// Keys not present in this map have no extra validation.
var validators = map[string]func(cmd *cobra.Command, value string) error{
	"default-provider": validateProvider,
}

func runSet(cmd *cobra.Command, args []string) {
	key := util.NormalizeKey(args[0])
	value := args[1]

	spec := config.Lookup(key)
	if spec == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown configuration key %q\n", args[0])
		fmt.Fprintf(cmd.ErrOrStderr(), "Valid keys: %s\n", strings.Join(config.KeyNames(), ", "))
		return
	}

	if validate, ok := validators[spec.Name]; ok {
		if err := validate(cmd, value); err != nil {
			return // validate already printed the error
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	normalized := util.NormalizeKey(value)
	spec.Set(cfg, normalized)
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s set to %q\n", spec.Name, normalized)
}

// validateProvider checks that the given name is a registered provider.
func validateProvider(cmd *cobra.Command, name string) error {
	normalized := util.NormalizeKey(name)
	known := providers.List()
	for _, p := range known {
		if p == normalized {
			return nil
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown provider %q\n", name)
	fmt.Fprintf(cmd.ErrOrStderr(), "Registered providers: %v\n", known)
	return fmt.Errorf("unknown provider %q", name)
}
