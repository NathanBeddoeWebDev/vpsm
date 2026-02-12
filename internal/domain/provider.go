package domain

import "context"

// Provider defines the core operations every VPS provider must support.
type Provider interface {
	GetDisplayName() string

	CreateServer(ctx context.Context, opts CreateServerOpts) (*Server, error)
	DeleteServer(ctx context.Context, id string) error
	GetServer(ctx context.Context, id string) (*Server, error)
	ListServers(ctx context.Context) ([]Server, error)
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
