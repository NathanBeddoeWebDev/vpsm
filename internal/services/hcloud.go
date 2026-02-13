package services

import (
	"context"
	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/retry"
	"sync"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type HCloudService struct {
	client *hcloud.Client
}

func NewHCloudService(token string) *HCloudService {
	return &HCloudService{
		client: hcloud.NewClient(hcloud.WithToken(token)),
	}
}

func (s *HCloudService) GetServer(ctx context.Context, id string) (*hcloud.Server, error) {
	server, _, err := s.client.Server.Get(ctx, id)

	return server, err
}

func (s *HCloudService) CreateServer(ctx context.Context, opts domain.CreateServerOpts) (hcloud.ServerCreateResult, *hcloud.Response, error) {
	hcloudOpts := hcloud.ServerCreateOpts{
		Name:             opts.Name,
		ServerType:       &hcloud.ServerType{Name: opts.ServerType},
		Image:            &hcloud.Image{Name: opts.Image},
		UserData:         opts.UserData,
		Labels:           opts.Labels,
		StartAfterCreate: opts.StartAfterCreate,
	}

	// can we use goroutines to run these fetches in parallel for cases where many ssh keys are selected?
	var wg sync.WaitGroup
	for _, sshKeyID := range opts.SSHKeyIdentifiers {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sshKey, _, err := s.client.SSHKey.Get(ctx, id)
			if err != nil {
				return
			}
			hcloudOpts.SSHKeys = append(hcloudOpts.SSHKeys, sshKey)
		}(sshKeyID)
	}

	if opts.Location != "" {
		hcloudOpts.Location = &hcloud.Location{Name: opts.Location}
	}

	return s.client.Server.Create(ctx, hcloudOpts)
}

// What happens if this fails?
func (s *HCloudService) GetSSHKey(ctx context.Context, id string) (*hcloud.SSHKey, error) {
	var sshKey *hcloud.SSHKey
	var err error
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Duration.Seconds(5)))
	defer cancel()

	retry.Do(ctx, retry.Config{MaxAttempts: 2, BaseDelay: 100}, func(error) bool {
		return true
	}, func() error {
		sshKey, _, err = s.client.SSHKey.Get(reqCtx, id)
		return err
	})

	return sshKey, err
}

func shouldRetryOnErrors(err error) bool {
	return err != nil && err != context.Canceled && err != context.DeadlineExceeded
}
