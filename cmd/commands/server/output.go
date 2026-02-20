package server

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/server/domain"

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

// printMetricsJSON encodes server metrics as indented JSON to stdout.
func printMetricsJSON(cmd *cobra.Command, metrics *domain.ServerMetrics) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.Encode(metrics)
}

// printMetricsSummary prints a table with per-series metric summaries.
func printMetricsSummary(cmd *cobra.Command, metrics *domain.ServerMetrics) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "METRIC\tCUR\tMIN\tMAX\tAVG")
	fmt.Fprintln(w, "------\t---\t---\t---\t---")

	// Well-known keys printed first in a stable order.
	orderedKeys := []string{
		"cpu",
		"disk.0.iops.read",
		"disk.0.iops.write",
		"network.0.bandwidth.in",
		"network.0.bandwidth.out",
	}

	seen := make(map[string]bool, len(orderedKeys))
	for _, key := range orderedKeys {
		seen[key] = true
	}

	// Collect any extra keys (e.g., disk.1.*, network.1.*) sorted alphabetically.
	var extraKeys []string
	for key := range metrics.TimeSeries {
		if !seen[key] {
			extraKeys = append(extraKeys, key)
		}
	}
	sort.Strings(extraKeys)

	allKeys := append(orderedKeys, extraKeys...)

	for _, key := range allKeys {
		ts, ok := metrics.TimeSeries[key]
		if !ok || len(ts.Values) == 0 {
			continue
		}

		suffix := metricSuffix(key)
		cur, min, max, avg := computeStats(ts.Values)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			key,
			formatMetric(cur, suffix),
			formatMetric(min, suffix),
			formatMetric(max, suffix),
			formatMetric(avg, suffix),
		)
	}

	w.Flush()

	fmt.Fprintf(cmd.OutOrStdout(), "\nTime range: %s to %s (step: %.0fs)\n",
		metrics.Start.UTC().Format("2006-01-02 15:04:05 UTC"),
		metrics.End.UTC().Format("2006-01-02 15:04:05 UTC"),
		metrics.Step,
	)
}

// metricSuffix returns the unit suffix for a metric key.
func metricSuffix(key string) string {
	switch {
	case key == "cpu":
		return "%"
	case strings.HasPrefix(key, "network."):
		return "B/s"
	default:
		return ""
	}
}

// computeStats returns cur (last), min, max, avg for a slice of values.
func computeStats(points []domain.MetricsPoint) (cur, min, max, avg float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}

	cur = points[len(points)-1].Value
	min = points[0].Value
	max = points[0].Value
	sum := 0.0

	for _, p := range points {
		v := p.Value
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg = sum / float64(len(points))
	return cur, min, max, avg
}

// formatMetric renders a value with a suffix using human-readable scaling.
func formatMetric(v float64, suffix string) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.1fG%s", v/1_000_000_000, suffix)
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM%s", v/1_000_000, suffix)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK%s", v/1_000, suffix)
	case v == 0:
		return "0" + suffix
	case math.Abs(v) < 0.01:
		return fmt.Sprintf("%.3f%s", v, suffix)
	default:
		return fmt.Sprintf("%.1f%s", v, suffix)
	}
}
