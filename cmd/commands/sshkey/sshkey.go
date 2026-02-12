package sshkey

import (
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/config"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "ssh-key",
		Short:             "Manage SSH keys across cloud providers",
		Long:              `Upload, list, and delete SSH keys from your configured cloud providers.`,
		PersistentPreRunE: resolveProvider,
	}

	cmd.AddCommand(AddCommand())

	cmd.PersistentFlags().String("provider", "", "Cloud provider to use (overrides default)")

	return cmd
}

// resolveProvider ensures the --provider flag has a value, falling back to the
// configured default when the flag was not explicitly passed.
func resolveProvider(cmd *cobra.Command, args []string) error {
	if cmd.Flag("provider").Changed {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.DefaultProvider != "" {
		cmd.Flag("provider").Value.Set(cfg.DefaultProvider)
		return nil
	}

	return fmt.Errorf("no provider specified: use --provider flag or set a default with 'vpsm config set default-provider <name>'")
}
