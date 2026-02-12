package providers

import (
	"context"
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/retry"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// CreateServer creates a new server on Hetzner Cloud.
// It maps domain.CreateServerOpts to the hcloud SDK, resolving any
// names (SSH keys, etc.) to their IDs where required by the API.
func (h *HetznerProvider) CreateServer(ctx context.Context, opts domain.CreateServerOpts) (*domain.Server, error) {
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
		var sshKey *hcloud.SSHKey
		err := retry.Do(ctx, h.retryConfig, isHetznerRetryable, func() error {
			reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
			defer cancel()
			var apiErr error
			sshKey, _, apiErr = h.client.SSHKey.Get(reqCtx, key)
			return apiErr
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve SSH key %q: %w", key, err)
		}
		if sshKey == nil {
			return nil, fmt.Errorf("SSH key %q not found", key)
		}
		hcloudOpts.SSHKeys = append(hcloudOpts.SSHKeys, sshKey)
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	result, _, err := h.client.Server.Create(reqCtx, hcloudOpts)
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeUniquenessError) {
			return nil, fmt.Errorf("failed to create server: %w", domain.ErrConflict)
		}
		if hcloud.IsError(err, hcloud.ErrorCodeUnauthorized) {
			return nil, fmt.Errorf("failed to create server: %w", domain.ErrUnauthorized)
		}
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			return nil, fmt.Errorf("failed to create server: %w", domain.ErrRateLimited)
		}
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	server := toDomainServer(result.Server)

	// Surface the root password if one was generated (i.e. no SSH keys were provided).
	if result.RootPassword != "" {
		server.Metadata["root_password"] = result.RootPassword
	}

	return &server, nil
}
