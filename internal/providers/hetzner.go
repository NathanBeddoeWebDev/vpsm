package providers

import (
	"encoding/json"
	"fmt"
	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	"net/http"
	"strconv"
	"time"
)

const hetznerHTTPTimeout = 30 * time.Second

// create a struct which follows the Provider interface
const hetznerBaseURL = "https://api.hetzner.cloud/v1"

// Hetzner API response structures
type hetznerListServersResponse struct {
	Servers []hetznerServer `json:"servers"`
}

type hetznerServer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Created   time.Time `json:"created"`
	PublicNet struct {
		IPv4 *struct {
			IP string `json:"ip"`
		} `json:"ipv4"`
		IPv6 *struct {
			IP string `json:"ip"`
		} `json:"ipv6"`
	} `json:"public_net"`
	PrivateNet []struct {
		IP string `json:"ip"`
	} `json:"private_net"`
	ServerType struct {
		Name         string `json:"name"`
		Architecture string `json:"architecture"`
	} `json:"server_type"`
	Image *struct {
		Name string `json:"name"`
	} `json:"image"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Datacenter struct {
		Name string `json:"name"`
	} `json:"datacenter"`
}

type HetznerProvider struct {
	client  *http.Client
	baseURL string
	token   string
}

func RegisterHetzner() {
	Register("hetzner", func(store auth.Store) (domain.Provider, error) {
		token, err := store.GetToken("hetzner")
		if err != nil {
			return nil, fmt.Errorf("hetzner auth: %w", err)
		}

		return &HetznerProvider{
			client:  &http.Client{Timeout: hetznerHTTPTimeout},
			baseURL: hetznerBaseURL,
			token:   token,
		}, nil
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

// ListServers retrieves all servers from Hetzner Cloud API
func (h *HetznerProvider) ListServers() ([]domain.Server, error) {
	req, err := h.newRequest(http.MethodGet, "/servers")
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp hetznerListServersResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	servers := make([]domain.Server, 0, len(apiResp.Servers))
	for _, hzServer := range apiResp.Servers {
		servers = append(servers, h.toDomainServer(hzServer))
	}

	return servers, nil
}

// toDomainServer converts a Hetzner API server to domain.Server
func (h *HetznerProvider) toDomainServer(hzServer hetznerServer) domain.Server {
	server := domain.Server{
		ID:         strconv.FormatInt(hzServer.ID, 10),
		Name:       hzServer.Name,
		Status:     hzServer.Status,
		CreatedAt:  hzServer.Created,
		Region:     hzServer.Location.Name,
		ServerType: hzServer.ServerType.Name,
		Provider:   "hetzner",
		Metadata:   make(map[string]interface{}),
	}

	// Extract public IPv4
	if hzServer.PublicNet.IPv4 != nil {
		server.PublicIPv4 = hzServer.PublicNet.IPv4.IP
	}

	// Extract public IPv6
	if hzServer.PublicNet.IPv6 != nil {
		server.PublicIPv6 = hzServer.PublicNet.IPv6.IP
	}

	// Extract first private IP if available
	if len(hzServer.PrivateNet) > 0 {
		server.PrivateIPv4 = hzServer.PrivateNet[0].IP
	}

	// Extract image name
	if hzServer.Image != nil {
		server.Image = hzServer.Image.Name
	}

	// Store Hetzner-specific metadata
	server.Metadata["hetzner_id"] = hzServer.ID
	server.Metadata["datacenter"] = hzServer.Datacenter.Name
	server.Metadata["architecture"] = hzServer.ServerType.Architecture

	return server
}

func (h *HetznerProvider) newRequest(method string, path string) (*http.Request, error) {
	url := h.baseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
