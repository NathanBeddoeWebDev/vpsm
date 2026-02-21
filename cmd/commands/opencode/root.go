// Package opencode provides CLI commands for provisioning and managing
// opencode VPS instances — servers pre-configured with the opencode AI
// coding assistant, a browser terminal, mDNS discovery, and a reverse proxy.
package opencode

import (
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/config"

	"github.com/spf13/cobra"
)

// NewCommand returns the root "opencode" Cobra command with all subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "opencode",
		Short:             "Provision and manage opencode VPS instances",
		PersistentPreRunE: resolveProvider,
		Long: `Provision VPS instances pre-configured with opencode, a browser terminal,
Avahi mDNS service discovery, and a Caddy or Nginx reverse proxy.

What gets installed on every opencode VPS:
  - opencode AI coding assistant (https://opencode.ai)
  - ttyd      — browser-based terminal at /terminal
  - Avahi     — mDNS so peers on the same network see <hostname>.local
  - Caddy or Nginx — reverse proxy for the terminal and deployed projects
  - UFW       — firewall (SSH / HTTP / HTTPS / mDNS only)
  - fail2ban  — SSH brute-force protection
  - SSH hardening (no root login, no password auth)

Workflow:
  1. vpsm opencode create             # Provision a secured opencode VPS
  2. Open http://<ip>/terminal        # Access the browser terminal
  3. Use opencode to build a project  # AI-assisted coding in the terminal
  4. ~/deploy-project.sh <name> <dir> # Deploy the built project via the proxy

Quick start:
  vpsm opencode create                # Interactive wizard
  vpsm opencode create --name mydev --type cpx21 --ssh-key my-key`,
	}

	cmd.AddCommand(CreateCommand())

	cmd.PersistentFlags().String("provider", "", "Cloud provider to use (overrides default)")

	return cmd
}

// resolveProvider ensures the --provider flag has a value, falling back to the
// configured default when the flag was not explicitly set.
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
