package server

import (
	"fmt"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/spf13/cobra"
)

func ListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all servers",
		Long:  `List all servers from the specified provider.`,
		Run: func(cmd *cobra.Command, args []string) {
			providerName := cmd.Flag("provider").Value.String()

			provider, err := providers.Get(providerName, auth.DefaultStore())
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				return
			}

			servers, err := provider.ListServers()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error listing servers: %v\n", err)
				return
			}

			if len(servers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No servers found.")
				return
			}

			// Create a tabwriter for pretty output
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
		},
	}

	return cmd
}
