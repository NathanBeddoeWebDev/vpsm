package domain

// Provider defines the core operations every VPS provider must support.
type Provider interface {
	GetDisplayName() string

	CreateServer(opts CreateServerOpts) (*Server, error)
	DeleteServer(id string) error
	ListServers() ([]Server, error)
}

// CatalogProvider extends Provider with methods that list the available
// resources a provider offers (locations, server types, images, SSH keys).
// This is used to power interactive selection flows.
type CatalogProvider interface {
	Provider

	ListLocations() ([]Location, error)
	ListServerTypes() ([]ServerTypeSpec, error)
	ListImages() ([]ImageSpec, error)
	ListSSHKeys() ([]SSHKeySpec, error)
}
