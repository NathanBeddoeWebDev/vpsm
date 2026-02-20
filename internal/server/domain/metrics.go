package domain

import "time"

// MetricType enumerates the metric categories available from providers.
type MetricType string

const (
	// MetricCPU represents CPU usage metrics.
	MetricCPU MetricType = "cpu"
	// MetricDisk represents disk I/O metrics.
	MetricDisk MetricType = "disk"
	// MetricNetwork represents network bandwidth metrics.
	MetricNetwork MetricType = "network"
)

// MetricsPoint is a single data point in a time series.
type MetricsPoint struct {
	Timestamp float64 `json:"timestamp"`
	Value     float64 `json:"value"`
}

// MetricsTimeSeries represents a single named series of timestamped values.
type MetricsTimeSeries struct {
	Name   string         `json:"name"`
	Values []MetricsPoint `json:"values"`
}

// ServerMetrics holds all metrics for a server over a time range.
type ServerMetrics struct {
	Start      time.Time                    `json:"start"`
	End        time.Time                    `json:"end"`
	Step       float64                      `json:"step"`
	TimeSeries map[string]MetricsTimeSeries `json:"time_series"`
}
