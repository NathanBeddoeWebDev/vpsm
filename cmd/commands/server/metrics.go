package server

import (
	"context"
	"fmt"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/spf13/cobra"
)

func MetricsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show server metrics",
		Long: `Display CPU, disk IOPS, and network bandwidth metrics for a server.

Fetches metrics from the last hour and prints a summary with current,
minimum, maximum, and average values for each time series.

Examples:
  # Table output (default)
  vpsm server metrics --provider hetzner --id 12345

  # JSON output for scripting
  vpsm server metrics --provider hetzner --id 12345 -o json`,
		Run: runMetrics,
	}

	cmd.Flags().String("id", "", "Server ID (required)")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringP("output", "o", "table", "Output format: table or json")

	return cmd
}

func runMetrics(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	mp, ok := provider.(domain.MetricsProvider)
	if !ok {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: provider %q does not support metrics\n", providerName)
		return
	}

	serverID, _ := cmd.Flags().GetString("id")

	ctx := context.Background()
	end := time.Now()
	start := end.Add(-1 * time.Hour)

	metrics, err := mp.GetServerMetrics(ctx, serverID, []domain.MetricType{
		domain.MetricCPU,
		domain.MetricDisk,
		domain.MetricNetwork,
	}, start, end)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error fetching metrics: %v\n", err)
		return
	}

	output, _ := cmd.Flags().GetString("output")
	switch output {
	case "json":
		printMetricsJSON(cmd, metrics)
	default:
		printMetricsSummary(cmd, metrics)
	}
}
