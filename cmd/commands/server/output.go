package server

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/domain"

	"github.com/spf13/cobra"
)

// printServerJSON encodes a server as indented JSON to the command's stdout.
func printServerJSON(cmd *cobra.Command, server *domain.Server) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.Encode(server)
}

// printServersJSON encodes a slice of servers as indented JSON to stdout.
func printServersJSON(cmd *cobra.Command, servers []domain.Server) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.Encode(servers)
}

// printServerDetail prints a vertical key-value table of all server fields.
func printServerDetail(cmd *cobra.Command, server *domain.Server) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "  ID:\t%s\n", server.ID)
	fmt.Fprintf(w, "  Name:\t%s\n", server.Name)
	fmt.Fprintf(w, "  Status:\t%s\n", server.Status)
	fmt.Fprintf(w, "  Provider:\t%s\n", server.Provider)
	fmt.Fprintf(w, "  Type:\t%s\n", server.ServerType)

	if server.Image != "" {
		fmt.Fprintf(w, "  Image:\t%s\n", server.Image)
	}

	fmt.Fprintf(w, "  Region:\t%s\n", server.Region)

	if server.PublicIPv4 != "" {
		fmt.Fprintf(w, "  IPv4:\t%s\n", server.PublicIPv4)
	}
	if server.PublicIPv6 != "" {
		fmt.Fprintf(w, "  IPv6:\t%s\n", server.PublicIPv6)
	}
	if server.PrivateIPv4 != "" {
		fmt.Fprintf(w, "  Private IP:\t%s\n", server.PrivateIPv4)
	}

	if !server.CreatedAt.IsZero() {
		fmt.Fprintf(w, "  Created:\t%s\n", server.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	}

	w.Flush()
}
