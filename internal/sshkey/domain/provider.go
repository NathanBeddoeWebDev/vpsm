package domain

import (
	"context"

	platformsshkey "nathanbeddoewebdev/vpsm/internal/platform/sshkey"
)

// Provider defines SSH key management operations for a cloud provider.
type Provider interface {
	GetDisplayName() string
	CreateSSHKey(ctx context.Context, name, publicKey string) (*platformsshkey.Spec, error)
}
