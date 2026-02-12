package providers

import (
	"context"
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/domain"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// CreateServer creates a new server on Hetzner Cloud.
// It maps domain.CreateServerOpts to the hcloud SDK, resolving any
// names (SSH keys, etc.) to their IDs where required by the API.
func (h *HetznerProvider) CreateServer(opts domain.CreateServerOpts) (*domain.Server, error) {
	ctx := context.Background()

	hcloudOpts := hcloud.ServerCreateOpts{
		Name:             opts.Name,
		ServerType:       &hcloud.ServerType{Name: opts.ServerType},
		Image:            &hcloud.Image{Name: opts.Image},
		UserData:         opts.UserData,
		Labels:           opts.Labels,
		StartAfterCreate: opts.StartAfterCreate,
	}

	if opts.Location != "" {
		hcloudOpts.Location = &hcloud.Location{Name: opts.Location}
	}

	// The SDK requires SSH key IDs in the request body, so we resolve
	// each name-or-ID through the API before creating the server.
	for _, key := range opts.SSHKeys {
		sshKey, _, err := h.client.SSHKey.Get(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve SSH key %q: %w", key, err)
		}
		if sshKey == nil {
			return nil, fmt.Errorf("SSH key %q not found", key)
		}
		hcloudOpts.SSHKeys = append(hcloudOpts.SSHKeys, sshKey)
	}

	result, _, err := h.client.Server.Create(ctx, hcloudOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	server := toDomainServer(result.Server)

	// Surface the root password if one was generated (i.e. no SSH keys were provided).
	if result.RootPassword != "" {
		server.Metadata["root_password"] = result.RootPassword
	}

	return &server, nil
}
