package providers

import "nathanbeddoewebdev/vpsm/internal/domain"

type Provider interface {
	GetDisplayName() string
	CreateServer(name string, region string, size string) (*domain.Server, error)
	DeleteServer(id string) error
}
