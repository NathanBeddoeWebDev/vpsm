package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/retry"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type HCloudService struct {
	client         *hcloud.Client
	retryConfig    retry.Config
	requestTimeout time.Duration
}

const defaultRequestTimeout = 30 * time.Second

func NewHCloudService(token string) *HCloudService {
	client := hcloud.NewClient(hcloud.WithToken(token))
	return NewHCloudServiceWithClient(client, retry.DefaultConfig(), defaultRequestTimeout)
}

func NewHCloudServiceWithClient(client *hcloud.Client, retryConfig retry.Config, requestTimeout time.Duration) *HCloudService {
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}

	return &HCloudService{
		client:         client,
		retryConfig:    retryConfig,
		requestTimeout: requestTimeout,
	}
}

func (s *HCloudService) GetServer(ctx context.Context, id string) (*hcloud.Server, error) {
	reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	server, _, err := s.client.Server.Get(reqCtx, id)
	return server, err
}

func (s *HCloudService) CreateServer(ctx context.Context, opts *domain.CreateServerOpts) (domain.Server, error) {
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

	for _, key := range opts.SSHKeyIdentifiers {
		reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
		defer cancel()
		sshKey, apiErr := s.GetSSHKey(reqCtx, key)
		if apiErr != nil {
			return domain.Server{}, fmt.Errorf("failed to resolve SSH key %q: %w", key, apiErr)
		}
		if sshKey == nil {
			return domain.Server{}, fmt.Errorf("SSH key %q not found", key)
		}
		hcloudOpts.SSHKeys = append(hcloudOpts.SSHKeys, sshKey)
	}

	reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()
	res, _, err := s.client.Server.Create(reqCtx, hcloudOpts)
	if err != nil {
		return domain.Server{}, err
	}

	server := domain.Server{
		ID:        strconv.FormatInt(res.Server.ID, 10),
		Name:      res.Server.Name,
		Status:    string(res.Server.Status),
		CreatedAt: res.Server.Created,
		Provider:  "hetzner",
	}

	if res.Server.ServerType != nil {
		server.ServerType = res.Server.ServerType.Name
	}
	if res.Server.Image != nil {
		server.Image = res.Server.Image.Name
	}
	if res.Server.Location != nil {
		server.Region = res.Server.Location.Name
	}

	return server, nil
}

func (s *HCloudService) GetSSHKey(ctx context.Context, id string) (*hcloud.SSHKey, error) {
	var sshKey *hcloud.SSHKey
	if err := retry.Do(ctx, s.retryConfig, isHCloudRetryable, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
		defer cancel()
		var apiErr error
		sshKey, _, apiErr = s.client.SSHKey.Get(reqCtx, id)
		return apiErr
	}); err != nil {
		return nil, err
	}

	return sshKey, nil
}

// StartServer powers on a server by its ID and returns the resulting action
// status so callers can poll for completion. The ID must be a numeric string
// matching the Hetzner server ID.
func (s *HCloudService) StartServer(ctx context.Context, id string) (*domain.ActionStatus, error) {
	numericID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid server ID %q: %w", id, err)
	}

	var action *hcloud.Action
	err = retry.Do(ctx, s.retryConfig, isHCloudRetryable, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
		defer cancel()
		var apiErr error
		action, _, apiErr = s.client.Server.Poweron(reqCtx, &hcloud.Server{ID: numericID})
		return apiErr
	})
	if err != nil {
		return nil, err
	}

	return toDomainAction(action), nil
}

// StopServer gracefully shuts down a server by its ID and returns the
// resulting action status so callers can poll for completion. The ID must
// be a numeric string matching the Hetzner server ID.
func (s *HCloudService) StopServer(ctx context.Context, id string) (*domain.ActionStatus, error) {
	numericID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid server ID %q: %w", id, err)
	}

	var action *hcloud.Action
	err = retry.Do(ctx, s.retryConfig, isHCloudRetryable, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
		defer cancel()
		var apiErr error
		action, _, apiErr = s.client.Server.Shutdown(reqCtx, &hcloud.Server{ID: numericID})
		return apiErr
	})
	if err != nil {
		return nil, err
	}

	return toDomainAction(action), nil
}

// PollAction retrieves the current status of an action by its ID.
// This is a single, non-retried request — callers are expected to
// poll in a loop with appropriate intervals, so adding retry logic
// here would compound rate-limit pressure.
func (s *HCloudService) PollAction(ctx context.Context, actionID string) (*domain.ActionStatus, error) {
	numericID, err := strconv.ParseInt(actionID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid action ID %q: %w", actionID, err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	action, _, err := s.client.Action.GetByID(reqCtx, numericID)
	if err != nil {
		return nil, err
	}

	if action == nil {
		return nil, fmt.Errorf("action %q not found", actionID)
	}

	return toDomainAction(action), nil
}

// toDomainAction converts an hcloud.Action to a domain.ActionStatus.
// A nil action (defensive) is treated as an immediate success.
func toDomainAction(a *hcloud.Action) *domain.ActionStatus {
	if a == nil {
		return &domain.ActionStatus{Status: domain.ActionStatusSuccess}
	}
	return &domain.ActionStatus{
		ID:           strconv.FormatInt(a.ID, 10),
		Status:       string(a.Status),
		Progress:     a.Progress,
		Command:      a.Command,
		ErrorMessage: a.ErrorMessage,
	}
}

func isHCloudRetryable(err error) bool {
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
		hcloud.ErrorCodeResourceUnavailable,
		hcloud.ErrorCodeMaintenance,
		hcloud.ErrorCodeRobotUnavailable,
		hcloud.ErrorCodeLocked,
	)
}