package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	opencodedomain "nathanbeddoewebdev/vpsm/internal/opencode/domain"
	"nathanbeddoewebdev/vpsm/internal/opencode/templates"
	opencodetui "nathanbeddoewebdev/vpsm/internal/opencode/tui"
	serverdomain "nathanbeddoewebdev/vpsm/internal/server/domain"
	"nathanbeddoewebdev/vpsm/internal/server/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// CreateCommand returns the "opencode create" cobra command.
func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new opencode VPS",
		Long: `Provision a cloud server pre-configured with opencode, a browser terminal,
Avahi mDNS, and a Caddy or Nginx reverse proxy.

After the server boots, cloud-init runs the setup (~2–3 min). Once ready:

  Browser terminal:  http://<server-ip>/terminal
  mDNS (local net):  http://<hostname>.local/terminal
  Tailscale (VPN):   http://<hostname>/terminal   (requires --tailscale-key)

Inside the terminal you can run opencode, build projects, and deploy them:
  opencode "build me a todo app in React"
  ~/deploy-project.sh todo-app ./workspace/todo-app/dist

Examples:
  # Interactive wizard (recommended)
  vpsm opencode create

  # Fully specified
  vpsm opencode create \
    --name     mydev   \
    --type     cpx21   \
    --location fsn1    \
    --ssh-key  my-key  \
    --proxy    caddy

  # With Tailscale for remote mDNS access
  vpsm opencode create --name mydev --tailscale-key tskey-auth-xxxxxx

  # With a public domain (Caddy obtains TLS automatically)
  vpsm opencode create --name mydev --domain dev.example.com`,
		Run: runCreate,
	}

	cmd.Flags().String("name", "", "Server hostname (must be a valid hostname)")
	cmd.Flags().String("type", "cpx21", "Server type  (e.g. cpx11, cpx21, cx22)")
	cmd.Flags().String("location", "", "Location     (e.g. fsn1, nbg1, hel1, ash)")
	cmd.Flags().String("image", "ubuntu-24.04", "Base OS image")
	cmd.Flags().StringArray("ssh-key", nil, "SSH key name or ID (repeatable)")
	cmd.Flags().String("proxy", "caddy", "Reverse proxy: caddy or nginx")
	cmd.Flags().String("tailscale-key", "", "Tailscale auth key — joins your tailnet for MagicDNS access")
	cmd.Flags().String("domain", "", "Public domain for automatic HTTPS via Let's Encrypt (Caddy only)")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	name, _ := cmd.Flags().GetString("name")
	serverType, _ := cmd.Flags().GetString("type")
	location, _ := cmd.Flags().GetString("location")
	image, _ := cmd.Flags().GetString("image")
	sshKeys, _ := cmd.Flags().GetStringArray("ssh-key")
	proxyStr, _ := cmd.Flags().GetString("proxy")
	tailscaleKey, _ := cmd.Flags().GetString("tailscale-key")
	domain, _ := cmd.Flags().GetString("domain")

	opts := opencodedomain.CreateOpenCodeOpts{
		Name:         name,
		ServerType:   serverType,
		Location:     location,
		Image:        image,
		SSHKeys:      sshKeys,
		ProxyType:    opencodedomain.ProxyType(proxyStr),
		TailscaleKey: tailscaleKey,
		Domain:       domain,
	}

	// Launch the interactive wizard if --name was not provided.
	if name == "" {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Fprintln(cmd.ErrOrStderr(), "Error: --name is required in non-interactive mode")
			fmt.Fprintln(cmd.ErrOrStderr(), "Provide all required flags or run in a terminal for interactive mode.")
			return
		}

		catalogProvider, ok := provider.(serverdomain.CatalogProvider)
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "Error: --name is required (provider does not support catalog listing)")
			return
		}

		finalOpts, err := opencodetui.RunCreateWizard(catalogProvider, opts)
		if err != nil {
			if errors.Is(err, opencodetui.ErrAborted) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled.")
				return
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		opts = *finalOpts
	}

	if opts.ProxyType == "" {
		opts.ProxyType = opencodedomain.ProxyTypeCaddy
	}

	// Render the cloud-init user data script from the template.
	userData, err := templates.RenderUserData(templates.UserDataParams{
		Hostname:     opts.Name,
		ProxyType:    string(opts.ProxyType),
		TailscaleKey: opts.TailscaleKey,
		Domain:       opts.Domain,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error building setup script: %v\n", err)
		return
	}

	// Map opencode opts to the generic server creation opts.
	labels := map[string]string{
		"managed-by": "vpsm",
		"role":       "opencode",
	}
	for k, v := range opts.Labels {
		labels[k] = v
	}

	createOpts := serverdomain.CreateServerOpts{
		Name:              opts.Name,
		Image:             opts.Image,
		ServerType:        opts.ServerType,
		Location:          opts.Location,
		SSHKeyIdentifiers: opts.SSHKeys,
		UserData:          userData,
		Labels:            labels,
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Provisioning opencode VPS %q [type=%s, image=%s",
		opts.Name, opts.ServerType, opts.Image)
	if opts.Location != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), ", location=%s", opts.Location)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "]")
	fmt.Fprintln(cmd.ErrOrStderr(), "Cloud-init will run the opencode setup automatically after boot.")

	ctx := context.Background()
	server, err := provider.CreateServer(ctx, createOpts)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error creating server: %v\n", err)
		return
	}

	printCreateResult(cmd, server, opts)
}

func printCreateResult(cmd *cobra.Command, server *serverdomain.Server, opts opencodedomain.CreateOpenCodeOpts) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(cmd.OutOrStdout(), "opencode VPS created successfully!")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintf(w, "  Name:\t%s\n", server.Name)
	fmt.Fprintf(w, "  ID:\t%s\n", server.ID)
	fmt.Fprintf(w, "  Status:\t%s\n", server.Status)
	fmt.Fprintf(w, "  Region:\t%s\n", server.Region)
	fmt.Fprintf(w, "  Type:\t%s\n", server.ServerType)
	fmt.Fprintf(w, "  Image:\t%s\n", server.Image)
	fmt.Fprintf(w, "  Public IP:\t%s\n", server.PublicIPv4)
	fmt.Fprintf(w, "  Proxy:\t%s\n", opts.ProxyType)
	w.Flush()

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Access (available once cloud-init finishes, ~2–3 min after boot):")
	fmt.Fprintln(cmd.OutOrStdout())

	w2 := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w2, "  Browser terminal:\thttp://%s/terminal\n", server.PublicIPv4)
	fmt.Fprintf(w2, "  mDNS (same network):\thttp://%s.local/terminal\n", server.Name)
	if opts.TailscaleKey != "" {
		fmt.Fprintf(w2, "  Tailscale (VPN):\thttp://%s/terminal\n", server.Name)
	}
	if opts.Domain != "" {
		fmt.Fprintf(w2, "  Domain (HTTPS):\thttps://%s/terminal\n", opts.Domain)
	}
	w2.Flush()

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Monitor cloud-init progress:")
	fmt.Fprintf(cmd.OutOrStdout(), "  ssh root@%s 'tail -f /var/log/cloud-init-output.log'\n", server.PublicIPv4)

	if pw, ok := server.Metadata["root_password"].(string); ok && pw != "" {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "  Root password: %s  (save this — shown only once)\n", pw)
	}
}
