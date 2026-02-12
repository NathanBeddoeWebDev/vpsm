package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"nathanbeddoewebdev/vpsm/internal/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all servers",
		Long: `List all servers from the specified provider.

In interactive mode (default), opens a full-window TUI with keyboard
navigation. Use --output json or --output table for non-interactive output.

Examples:
  # Interactive TUI
  vpsm server list

  # Non-interactive table
  vpsm server list -o table

  # JSON output for scripting
  vpsm server list -o json`,
		Run: runList,
	}

	cmd.Flags().StringP("output", "o", "", "Output format: table or json (omit for interactive TUI)")

	return cmd
}

func runList(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	output, _ := cmd.Flags().GetString("output")

	// Non-interactive mode for scripting, or when no TTY is available.
	if output == "json" || output == "table" || !term.IsTerminal(int(os.Stdout.Fd())) {
		if output == "" {
			output = "table"
		}
		runListNonInteractive(cmd, provider, output)
		return
	}

	// Interactive full-window TUI.
	selected, action, err := tui.RunServerList(provider, providerName)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	// Handle actions from the list view.
	switch action {
	case "show":
		if selected != nil {
			runShowFromList(cmd, provider, providerName, selected)
		}
	case "delete":
		if selected != nil {
			runDeleteFromList(cmd, provider, providerName, selected)
		}
	case "create":
		runCreateFromList(cmd, provider, providerName)
	}
}

func runShowFromList(cmd *cobra.Command, provider domain.Provider, providerName string, server *domain.Server) {
	result, err := tui.RunServerShowDirect(provider, providerName, server)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	if result != nil && result.Action == "delete" {
		runDeleteFromList(cmd, provider, providerName, result.Server)
	}
}

func runDeleteFromList(cmd *cobra.Command, provider domain.Provider, providerName string, server *domain.Server) {
	result, err := tui.RunServerDelete(provider, providerName, server)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	if result != nil && result.Confirmed {
		ctx := context.Background()
		if err := provider.DeleteServer(ctx, result.Server.ID); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error deleting server: %v\n", err)
			return
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Server %q (ID: %s) deleted successfully.\n", result.Server.Name, result.Server.ID)
	}
}

func runCreateFromList(cmd *cobra.Command, provider domain.Provider, providerName string) {
	catalogProvider, ok := provider.(domain.CatalogProvider)
	if !ok {
		fmt.Fprintln(cmd.ErrOrStderr(), "Interactive server creation is not supported for this provider.")
		return
	}

	opts, err := tui.RunServerCreate(catalogProvider, providerName, domain.CreateServerOpts{})
	if err != nil {
		if errors.Is(err, tui.ErrAborted) {
			return
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	if opts == nil {
		return
	}

	ctx := context.Background()
	server, err := provider.CreateServer(ctx, *opts)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error creating server: %v\n", err)
		return
	}

	printCreateTable(cmd, server)
}

func runListNonInteractive(cmd *cobra.Command, provider domain.Provider, output string) {
	ctx := context.Background()
	servers, err := provider.ListServers(ctx)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error listing servers: %v\n", err)
		return
	}

	if output == "json" {
		printServersJSON(cmd, servers)
		return
	}

	if len(servers) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No servers found.")
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tREGION\tTYPE\tPUBLIC IPv4\tIMAGE")
	fmt.Fprintln(w, "--\t----\t------\t------\t----\t-----------\t-----")

	for _, server := range servers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			server.ID,
			server.Name,
			server.Status,
			server.Region,
			server.ServerType,
			server.PublicIPv4,
			server.Image,
		)
	}

	w.Flush()
}
