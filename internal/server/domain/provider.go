package domain

import (
	"context"
	"time"
)

// Provider defines the core operations every VPS provider must support.
type Provider interface {
	GetDisplayName() string

	CreateServer(ctx context.Context, opts CreateServerOpts) (*Server, error)
	DeleteServer(ctx context.Context, id string) error
	GetServer(ctx context.Context, id string) (*Server, error)
	ListServers(ctx context.Context) ([]Server, error)
	StartServer(ctx context.Context, id string) (*ActionStatus, error)
	StopServer(ctx context.Context, id string) (*ActionStatus, error)
}

// CatalogProvider extends Provider with methods that list the available
// resources a provider offers (locations, server types, images, SSH keys).
// This is used to power interactive selection flows.
type CatalogProvider interface {
	Provider

	ListLocations(ctx context.Context) ([]Location, error)
	ListServerTypes(ctx context.Context) ([]ServerTypeSpec, error)
	ListImages(ctx context.Context) ([]ImageSpec, error)
	ListSSHKeys(ctx context.Context) ([]SSHKeySpec, error)
}

// SSHKeyManager extends Provider with SSH key management operations.
type SSHKeyManager interface {
	Provider

	CreateSSHKey(ctx context.Context, name, publicKey string) (*SSHKeySpec, error)
}

// ActionPoller extends Provider with the ability to poll the status of a
// long-running action. Providers that support asynchronous operations
// (e.g. Hetzner Cloud) implement this so the TUI and CLI can track
// progress without blind-refreshing.
type ActionPoller interface {
	Provider

	PollAction(ctx context.Context, actionID string) (*ActionStatus, error)
}

// MetricsProvider extends Provider with server metrics retrieval.
// Providers that expose time-series telemetry (CPU, disk, network)
// implement this so the TUI can render usage charts.
type MetricsProvider interface {
	Provider

	GetServerMetrics(ctx context.Context, serverID string, types []MetricType, start, end time.Time) (*ServerMetrics, error)
}
