package providers

import (
	"fmt"
	"nathanbeddoewebdev/vpsm/internal/domain"
	"net/http"
)

// create a struct which follows the Provider interface
type HetznerProvider struct{
	client http.Client
}

func RegisterHetzner() {
	Register("hetzner", func() domain.Provider {
		return &HetznerProvider{
			client: http.Client{},
		}
	})
}

func (h *HetznerProvider) Authenticate() error {
	return fmt.Errorf("not implemented")
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

func (h *HetznerProvider) Status() error {
	// test the hetzner API
	resp, err := h.client.Get("https://api.hetzner.cloud/v1/servers")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
