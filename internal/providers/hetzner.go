package providers

import (
	"fmt"
	"nathanbeddoewebdev/vpsm/internal/domain"
)

// create a struct which follows the Provider interface
type HetznerProvider struct{}

func RegisterHetzner() {
	Register("hetzner", func() Provider {
		return &HetznerProvider{}
	})
}

func (h *HetznerProvider) GetDisplayName() string {
	return "Hetzner"
}

func (h *HetznerProvider) CreateServer(name string, region string, size string) (*domain.Server, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *HetznerProvider) DeleteServer(id string) error {
	return fmt.Errorf("not implemented")
}
