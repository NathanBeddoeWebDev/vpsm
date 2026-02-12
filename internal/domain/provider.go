package domain

type Provider interface {
	GetDisplayName() string
	Authenticate() error
	CreateServer(name string, region string, size string) (*Server, error)
	DeleteServer(id string) error
	ListServers() ([]Server, error)
}
