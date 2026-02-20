package providers

import (
	"fmt"

	serverproviders "nathanbeddoewebdev/vpsm/internal/server/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
	sshkeydomain "nathanbeddoewebdev/vpsm/internal/sshkey/domain"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

var _ sshkeydomain.Provider = (*serverproviders.HetznerProvider)(nil)

// RegisterHetzner registers the Hetzner SSH key provider factory.
func RegisterHetzner() {
	Register("hetzner", func(store auth.Store) (sshkeydomain.Provider, error) {
		token, err := store.GetToken("hetzner")
		if err != nil {
			return nil, fmt.Errorf("hetzner auth: %w", err)
		}

		return serverproviders.NewHetznerProvider(hcloud.WithToken(token)), nil
	})
}
