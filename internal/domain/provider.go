package domain

type Provider interface {
	GetDisplayName() string
	CreateServer(name string, region string, size string) (*Server, error)
	DeleteServer(id string) error
}
