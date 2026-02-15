package components

import (
	"fmt"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/tui/styles"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

// chartHeight is the fixed height for all metric sparklines.
const chartHeight = 5

// MetricsChart renders a single-series sparkline with a label header.
// Returns an empty string if data is empty.
func MetricsChart(label string, data []float64, width int, suffix string) string {
	if len(data) == 0 {
		return styles.MutedText.Render(label + ": no data")
	}

	// Reserve space for Y-axis labels (number + " ┤" ≈ 9 chars).
	plotWidth := width - 9
	if plotWidth < 10 {
		plotWidth = 10
	}

	chart := asciigraph.Plot(data,
		asciigraph.Height(chartHeight),
		asciigraph.Width(plotWidth),
		asciigraph.Precision(0),
		asciigraph.SeriesColors(asciigraph.DodgerBlue),
		asciigraph.LabelColor(asciigraph.Default),
	)

	// Summary line: current (latest), min, max.
	current := data[len(data)-1]
	min, max := minMax(data)
	summary := styles.MutedText.Render(
		fmt.Sprintf("  cur: %s  min: %s  max: %s",
			formatValue(current, suffix),
			formatValue(min, suffix),
			formatValue(max, suffix),
		),
	)

	header := styles.Label.Render(label)
	return lipgloss.JoinVertical(lipgloss.Left, header, chart, summary)
}

// MetricsDualChart renders two overlaid series (e.g., read/write, in/out)
// with a shared label header and per-series legends.
func MetricsDualChart(label string, series1, series2 []float64, legend1, legend2 string, width int, suffix string) string {
	if len(series1) == 0 && len(series2) == 0 {
		return styles.MutedText.Render(label + ": no data")
	}

	// Ensure both series are present for PlotMany; use empty slice as fallback.
	if len(series1) == 0 {
		series1 = make([]float64, len(series2))
	}
	if len(series2) == 0 {
		series2 = make([]float64, len(series1))
	}

	plotWidth := width - 9
	if plotWidth < 10 {
		plotWidth = 10
	}

	chart := asciigraph.PlotMany(
		[][]float64{series1, series2},
		asciigraph.Height(chartHeight),
		asciigraph.Width(plotWidth),
		asciigraph.Precision(0),
		asciigraph.SeriesColors(asciigraph.DodgerBlue, asciigraph.LightCoral),
		asciigraph.SeriesLegends(legend1, legend2),
		asciigraph.LabelColor(asciigraph.Default),
	)

	// Summary line for each series.
	var summaryParts []string
	if len(series1) > 0 {
		cur1 := series1[len(series1)-1]
		min1, max1 := minMax(series1)
		summaryParts = append(summaryParts,
			fmt.Sprintf("  %s  cur: %s  min: %s  max: %s",
				legend1, formatValue(cur1, suffix), formatValue(min1, suffix), formatValue(max1, suffix)),
		)
	}
	if len(series2) > 0 {
		cur2 := series2[len(series2)-1]
		min2, max2 := minMax(series2)
		summaryParts = append(summaryParts,
			fmt.Sprintf("  %s  cur: %s  min: %s  max: %s",
				legend2, formatValue(cur2, suffix), formatValue(min2, suffix), formatValue(max2, suffix)),
		)
	}
	summary := styles.MutedText.Render(strings.Join(summaryParts, "\n"))

	header := styles.Label.Render(label)
	return lipgloss.JoinVertical(lipgloss.Left, header, chart, summary)
}

// minMax returns the minimum and maximum values from a slice.
func minMax(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	min, max := data[0], data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// formatValue renders a float with an optional suffix, using human-readable
// formatting for large values.
func formatValue(v float64, suffix string) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.1fG%s", v/1_000_000_000, suffix)
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM%s", v/1_000_000, suffix)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK%s", v/1_000, suffix)
	default:
		return fmt.Sprintf("%.1f%s", v, suffix)
	}
}
