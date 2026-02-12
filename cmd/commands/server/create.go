package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
)

func CreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new server",
		Long: `Create a new server instance with the specified provider.

All three of --name, --image, and --type are required unless you use
interactive mode. If any are missing and the provider supports catalog
listing, a TUI wizard will guide you through the required choices.

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

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	name, _ := cmd.Flags().GetString("name")
	image, _ := cmd.Flags().GetString("image")
	serverType, _ := cmd.Flags().GetString("type")
	location, _ := cmd.Flags().GetString("location")
	sshKeys, _ := cmd.Flags().GetStringArray("ssh-key")
	labels, _ := cmd.Flags().GetStringArray("label")
	userData, _ := cmd.Flags().GetString("user-data")

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

	opts := domain.CreateServerOpts{
		Name:       name,
		Image:      image,
		ServerType: serverType,
	}

	if location != "" {
		opts.Location = location
	}
	if len(sshKeys) > 0 {
		opts.SSHKeys = sshKeys
	}
	if len(labels) > 0 {
		opts.Labels = parseLabels(labels)
	}
	if userData != "" {
		opts.UserData = userData
	}
	if cmd.Flags().Changed("start") {
		start, _ := cmd.Flags().GetBool("start")
		opts.StartAfterCreate = &start
	}

	useInteractive := len(missing) > 0
	if useInteractive {
		catalogProvider, ok := provider.(domain.CatalogProvider)
		if !ok {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: missing required flag(s): %s\n", strings.Join(missing, ", "))
			fmt.Fprintln(cmd.ErrOrStderr(), "Interactive mode is not supported for this provider.")
			return
		}

		finalOpts, err := tui.CreateServerForm(catalogProvider, opts)
		if err != nil {
			if errors.Is(err, tui.ErrAborted) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Server creation cancelled.")
				return
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		opts = *finalOpts
	}

	logCreateOpts(cmd, opts)

	var server *domain.Server
	if useInteractive {
		accessible := os.Getenv("ACCESSIBLE") != ""
		var createErr error
		spinErr := spinner.New().
			Title("Creating server...").
			Accessible(accessible).
			Output(cmd.ErrOrStderr()).
			Action(func() {
				server, createErr = provider.CreateServer(opts)
			}).
			Run()
		if spinErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", spinErr)
			return
		}
		err = createErr
	} else {
		server, err = provider.CreateServer(opts)
	}
	if err != nil {
		logCreateOptsFull(cmd, opts)
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

func logCreateOpts(cmd *cobra.Command, opts domain.CreateServerOpts) {
	location := opts.Location
	if location == "" {
		location = "(auto)"
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Creating server %q [type=%s, image=%s, location=%s]\n",
		opts.Name, opts.ServerType, opts.Image, location)
}

func logCreateOptsFull(cmd *cobra.Command, opts domain.CreateServerOpts) {
	w := cmd.ErrOrStderr()
	fmt.Fprintln(w, "\nRequest details:")
	fmt.Fprintf(w, "  Name:        %s\n", opts.Name)
	fmt.Fprintf(w, "  Server type: %s\n", opts.ServerType)
	fmt.Fprintf(w, "  Image:       %s\n", opts.Image)
	if opts.Location != "" {
		fmt.Fprintf(w, "  Location:    %s\n", opts.Location)
	} else {
		fmt.Fprintf(w, "  Location:    (auto)\n")
	}
	if len(opts.SSHKeys) > 0 {
		fmt.Fprintf(w, "  SSH keys:    %s\n", strings.Join(opts.SSHKeys, ", "))
	}
	if len(opts.Labels) > 0 {
		parts := make([]string, 0, len(opts.Labels))
		for k, v := range opts.Labels {
			parts = append(parts, k+"="+v)
		}
		fmt.Fprintf(w, "  Labels:      %s\n", strings.Join(parts, ", "))
	}
	if opts.UserData != "" {
		fmt.Fprintf(w, "  User data:   %d bytes\n", len(opts.UserData))
	}
	if opts.StartAfterCreate != nil {
		fmt.Fprintf(w, "  Start after: %t\n", *opts.StartAfterCreate)
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
