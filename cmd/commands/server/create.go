package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/spf13/cobra"
)

func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Long: `Create a new server instance with the specified provider.

All three of --name, --image, and --type are required. When any are missing
the command will exit with an error (interactive mode coming soon).

Examples:
  # Minimal
  vpsm server create --provider hetzner --name web-1 --image ubuntu-24.04 --type cpx11

  # With location and SSH keys
  vpsm server create --provider hetzner \
    --name web-1 \
    --image ubuntu-24.04 \
    --type cpx11 \
    --location fsn1 \
    --ssh-key my-key \
    --ssh-key deploy-key

  # JSON output for scripting
  vpsm server create --provider hetzner \
    --name web-1 --image ubuntu-24.04 --type cpx11 \
    -o json`,
		Run: runCreate,
	}

	// Required for flag mode
	cmd.Flags().String("name", "", "Server name (must be a valid hostname)")
	cmd.Flags().String("image", "", "Image name or ID (e.g. ubuntu-24.04)")
	cmd.Flags().String("type", "", "Server type name or ID (e.g. cpx11)")

	// Optional
	cmd.Flags().String("location", "", "Location name or ID (e.g. fsn1)")
	cmd.Flags().StringArray("ssh-key", nil, "SSH key name or ID (can be specified multiple times)")
	cmd.Flags().StringArray("label", nil, "Label in key=value format (can be specified multiple times)")
	cmd.Flags().String("user-data", "", "Cloud-init user data string")
	cmd.Flags().Bool("start", true, "Start server after creation")

	// Output
	cmd.Flags().StringP("output", "o", "table", "Output format: table or json")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()
	if providerName == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error: --provider flag is required")
		return
	}

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	name, _ := cmd.Flags().GetString("name")
	image, _ := cmd.Flags().GetString("image")
	serverType, _ := cmd.Flags().GetString("type")

	// Require all three flags for now; interactive mode will remove this constraint.
	var missing []string
	if name == "" {
		missing = append(missing, "--name")
	}
	if image == "" {
		missing = append(missing, "--image")
	}
	if serverType == "" {
		missing = append(missing, "--type")
	}
	if len(missing) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: missing required flag(s): %s\n", strings.Join(missing, ", "))
		fmt.Fprintln(cmd.ErrOrStderr(), "Provide all required flags or run without them to use interactive mode (coming soon).")
		return
	}

	opts := domain.CreateServerOpts{
		Name:       name,
		Image:      image,
		ServerType: serverType,
	}

	if location, _ := cmd.Flags().GetString("location"); location != "" {
		opts.Location = location
	}
	if sshKeys, _ := cmd.Flags().GetStringArray("ssh-key"); len(sshKeys) > 0 {
		opts.SSHKeys = sshKeys
	}
	if labels, _ := cmd.Flags().GetStringArray("label"); len(labels) > 0 {
		opts.Labels = parseLabels(labels)
	}
	if userData, _ := cmd.Flags().GetString("user-data"); userData != "" {
		opts.UserData = userData
	}
	if cmd.Flags().Changed("start") {
		start, _ := cmd.Flags().GetBool("start")
		opts.StartAfterCreate = &start
	}

	server, err := provider.CreateServer(opts)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error creating server: %v\n", err)
		return
	}

	output, _ := cmd.Flags().GetString("output")
	switch output {
	case "json":
		printServerJSON(cmd, server)
	default:
		printServerTable(cmd, server)
	}
}

func parseLabels(labels []string) map[string]string {
	result := make(map[string]string, len(labels))
	for _, l := range labels {
		k, v, ok := strings.Cut(l, "=")
		if ok {
			result[k] = v
		}
	}
	return result
}

func printServerTable(cmd *cobra.Command, server *domain.Server) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "✓ Server created successfully!")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  ID:\t%s\n", server.ID)
	fmt.Fprintf(w, "  Name:\t%s\n", server.Name)
	fmt.Fprintf(w, "  Status:\t%s\n", server.Status)
	fmt.Fprintf(w, "  Type:\t%s\n", server.ServerType)
	fmt.Fprintf(w, "  Image:\t%s\n", server.Image)
	fmt.Fprintf(w, "  Region:\t%s\n", server.Region)

	if server.PublicIPv4 != "" {
		fmt.Fprintf(w, "  IPv4:\t%s\n", server.PublicIPv4)
	}
	if server.PublicIPv6 != "" {
		fmt.Fprintf(w, "  IPv6:\t%s\n", server.PublicIPv6)
	}

	if pw, ok := server.Metadata["root_password"].(string); ok && pw != "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ⚠ Root Password:\t%s\n", pw)
		fmt.Fprintln(w, "  Save this now — it will not be shown again.")
	}

	w.Flush()
}

func printServerJSON(cmd *cobra.Command, server *domain.Server) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.Encode(server)
}
