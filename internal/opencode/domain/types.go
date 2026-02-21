// Package domain defines types for opencode VPS provisioning.
package domain

// ProxyType identifies the reverse proxy to install on the opencode VPS.
type ProxyType string

const (
	// ProxyTypeCaddy installs Caddy as the reverse proxy (default).
	ProxyTypeCaddy ProxyType = "caddy"
	// ProxyTypeNginx installs Nginx as the reverse proxy.
	ProxyTypeNginx ProxyType = "nginx"
)

// CreateOpenCodeOpts holds the parameters for provisioning an opencode VPS.
// Server infrastructure fields map directly to the underlying provider's
// CreateServerOpts; opencode-specific fields control the software setup.
type CreateOpenCodeOpts struct {
	// Server infrastructure (passed through to provider)
	Name       string
	Location   string
	ServerType string
	Image      string
	SSHKeys    []string
	Labels     map[string]string

	// OpenCode configuration
	ProxyType    ProxyType // reverse proxy to install; defaults to caddy
	TailscaleKey string    // optional Tailscale auth key for VPN + MagicDNS access
	Domain       string    // optional public domain for Caddy HTTPS (e.g. dev.example.com)
}
