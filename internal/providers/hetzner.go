package providers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"nathanbeddoewebdev/vpsm/internal/cache"
	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/retry"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// Compile-time check that HetznerProvider satisfies CatalogProvider
// (which embeds Provider).
var _ domain.CatalogProvider = (*HetznerProvider)(nil)
var _ domain.SSHKeyManager = (*HetznerProvider)(nil)

// HetznerProvider implements domain.Provider using the Hetzner Cloud API.
type HetznerProvider struct {
	client *hcloud.Client
	cache  *cache.Cache
}

const (
	requestTimeout         = 30 * time.Second
	defaultCatalogCacheTTL = time.Hour
)

// NewHetznerProvider creates a HetznerProvider with the given hcloud client options.
// Default options (application name) are applied first; callers can override them.
func NewHetznerProvider(opts ...hcloud.ClientOption) *HetznerProvider {
	defaults := []hcloud.ClientOption{
		hcloud.WithApplication("vpsm", "0.1.0"),
	}
	allOpts := append(defaults, opts...)
	return &HetznerProvider{
		client: hcloud.NewClient(allOpts...),
		cache:  cache.NewDefault(),
	}
}

// RegisterHetzner registers the Hetzner provider factory with the global registry.
func RegisterHetzner() {
	Register("hetzner", func(store auth.Store) (domain.Provider, error) {
		token, err := store.GetToken("hetzner")
		if err != nil {
			return nil, fmt.Errorf("hetzner auth: %w", err)
		}

		return NewHetznerProvider(hcloud.WithToken(token)), nil
	})
}

func (h *HetznerProvider) GetDisplayName() string {
	return "Hetzner"
}

// DeleteServer removes a server by its ID. The ID must be a numeric string
// matching the Hetzner server ID.
func (h *HetznerProvider) DeleteServer(ctx context.Context, id string) error {
	numericID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid server ID %q: %w", id, err)
	}

	err = retry.Do(ctx, retry.DefaultConfig(), isHetznerRetryable, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		_, _, err := h.client.Server.DeleteWithResult(reqCtx, &hcloud.Server{ID: numericID})
		return err
	})
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeNotFound) {
			return fmt.Errorf("failed to delete server: %w", domain.ErrNotFound)
		}
		if hcloud.IsError(err, hcloud.ErrorCodeUnauthorized) {
			return fmt.Errorf("failed to delete server: %w", domain.ErrUnauthorized)
		}
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			return fmt.Errorf("failed to delete server: %w", domain.ErrRateLimited)
		}
		return fmt.Errorf("failed to delete server: %w", err)
	}

	return nil
}

// GetServer retrieves a single server by its ID.
func (h *HetznerProvider) GetServer(ctx context.Context, id string) (*domain.Server, error) {
	numericID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid server ID %q: %w", id, err)
	}

	var hzServer *hcloud.Server
	err = retry.Do(ctx, retry.DefaultConfig(), isHetznerRetryable, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		var apiErr error
		hzServer, _, apiErr = h.client.Server.GetByID(reqCtx, numericID)
		return apiErr
	})
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeUnauthorized) {
			return nil, fmt.Errorf("failed to get server: %w", domain.ErrUnauthorized)
		}
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			return nil, fmt.Errorf("failed to get server: %w", domain.ErrRateLimited)
		}
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	if hzServer == nil {
		return nil, fmt.Errorf("server %q: %w", id, domain.ErrNotFound)
	}

	server := toDomainServer(hzServer)
	return &server, nil
}

// ListServers retrieves all servers from the Hetzner Cloud API.
func (h *HetznerProvider) ListServers(ctx context.Context) ([]domain.Server, error) {
	var hzServers []*hcloud.Server
	err := retry.Do(ctx, retry.DefaultConfig(), isHetznerRetryable, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		var apiErr error
		hzServers, apiErr = h.client.Server.All(reqCtx)
		return apiErr
	})
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeUnauthorized) {
			return nil, fmt.Errorf("failed to list servers: %w", domain.ErrUnauthorized)
		}
		if hcloud.IsError(err, hcloud.ErrorCodeRateLimitExceeded) {
			return nil, fmt.Errorf("failed to list servers: %w", domain.ErrRateLimited)
		}
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	servers := make([]domain.Server, 0, len(hzServers))
	for _, s := range hzServers {
		servers = append(servers, toDomainServer(s))
	}

	return servers, nil
}

// toDomainServer converts an hcloud.Server to a domain.Server.
func toDomainServer(s *hcloud.Server) domain.Server {
	server := domain.Server{
		ID:        strconv.FormatInt(s.ID, 10),
		Name:      s.Name,
		Status:    string(s.Status),
		CreatedAt: s.Created,
		Provider:  "hetzner",
		Metadata:  make(map[string]interface{}),
	}

	if !s.PublicNet.IPv4.IsUnspecified() {
		server.PublicIPv4 = s.PublicNet.IPv4.IP.String()
	}

	if !s.PublicNet.IPv6.IsUnspecified() {
		server.PublicIPv6 = s.PublicNet.IPv6.IP.String()
	}

	if len(s.PrivateNet) > 0 && s.PrivateNet[0].IP != nil {
		server.PrivateIPv4 = s.PrivateNet[0].IP.String()
	}

	if s.ServerType != nil {
		server.ServerType = s.ServerType.Name
		server.Metadata["architecture"] = string(s.ServerType.Architecture)
	}

	if s.Image != nil {
		server.Image = s.Image.Name
	}

	if s.Location != nil {
		server.Region = s.Location.Name
	}

	// Store Hetzner-specific metadata
	server.Metadata["hetzner_id"] = s.ID

	return server
}

func isHetznerRetryable(err error) bool {
	if retry.IsRetryable(err) {
		return true
	}

	return hcloud.IsError(
		err,
		hcloud.ErrorCodeRateLimitExceeded,
		hcloud.ErrorCodeServiceError,
		hcloud.ErrorCodeServerError,
		hcloud.ErrorCodeTimeout,
		hcloud.ErrorCodeUnknownError,
		hcloud.ErrorCodeConflict,
		hcloud.ErrorCodeResourceUnavailable,
		hcloud.ErrorCodeMaintenance,
		hcloud.ErrorCodeRobotUnavailable,
		hcloud.ErrorCodeLocked,
	)
}
