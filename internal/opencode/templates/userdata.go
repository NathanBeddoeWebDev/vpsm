// Package templates provides cloud-init user data generation for opencode VPS instances.
package templates

import (
	"bytes"
	"fmt"
	"text/template"
)

// UserDataParams holds the substitution values for the cloud-init template.
type UserDataParams struct {
	// Hostname is set as the system hostname and used for mDNS advertisement.
	Hostname string
	// ProxyType is "caddy" or "nginx".
	ProxyType string
	// TailscaleKey is an optional Tailscale auth key. When non-empty, Tailscale
	// is installed and the server joins the user's tailnet for MagicDNS access.
	TailscaleKey string
	// Domain is an optional public FQDN. When set with Caddy, automatic HTTPS
	// via Let's Encrypt is configured.
	Domain string
}

// RenderUserData renders the cloud-init user data script for an opencode VPS.
func RenderUserData(p UserDataParams) (string, error) {
	if p.Hostname == "" {
		return "", fmt.Errorf("hostname is required")
	}
	if p.ProxyType == "" {
		p.ProxyType = "caddy"
	}

	tmpl, err := template.New("userdata").Parse(userDataTemplate)
	if err != nil {
		return "", fmt.Errorf("parse user data template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("render user data template: %w", err)
	}

	return buf.String(), nil
}

// userDataTemplate is the cloud-init bash script that:
//   - Hardens SSH (no root login, no password auth)
//   - Configures UFW firewall (SSH, HTTP, HTTPS, mDNS)
//   - Installs fail2ban with SSH jail
//   - Installs Avahi for mDNS service advertisement
//   - Creates a 'coder' user with a workspace
//   - Installs Node.js 20.x and opencode
//   - Installs ttyd for browser-based terminal access
//   - Installs Caddy or Nginx as a reverse proxy
//   - Optionally installs Tailscale for VPN + MagicDNS access
//   - Adds a deploy-project.sh helper script
const userDataTemplate = `#!/bin/bash
set -euo pipefail

HOSTNAME_VAL="{{ .Hostname }}"
PROXY_TYPE="{{ .ProxyType }}"
TAILSCALE_KEY="{{ .TailscaleKey }}"
DOMAIN="{{ .Domain }}"

log() { echo "[vpsm-opencode] $*" >&2; }

log "=== Starting opencode VPS setup ==="

# ── Hostname ─────────────────────────────────────────────────────────────────
hostnamectl set-hostname "$HOSTNAME_VAL"
printf '127.0.1.1\t%s\n' "$HOSTNAME_VAL" >> /etc/hosts

# ── System update ─────────────────────────────────────────────────────────────
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get upgrade -y

apt-get install -y \
  curl \
  wget \
  git \
  unzip \
  ufw \
  fail2ban \
  avahi-daemon \
  avahi-utils \
  libnss-mdns

log "=== Base packages installed ==="

# ── SSH hardening ─────────────────────────────────────────────────────────────
# Drop-in config to avoid conflicting with the base sshd_config.
cat > /etc/ssh/sshd_config.d/99-hardening.conf << 'SSHEOF'
PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
X11Forwarding no
MaxAuthTries 3
LoginGraceTime 30
AllowAgentForwarding no
AllowTcpForwarding no
SSHEOF

# Restart via the correct service name (varies by Ubuntu release)
systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
log "=== SSH hardened ==="

# ── Firewall (UFW) ─────────────────────────────────────────────────────────────
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   # SSH
ufw allow 80/tcp   # HTTP
ufw allow 443/tcp  # HTTPS
ufw allow 5353/udp # mDNS
ufw --force enable
log "=== Firewall configured ==="

# ── Fail2ban ──────────────────────────────────────────────────────────────────
cat > /etc/fail2ban/jail.local << 'F2BEOF'
[DEFAULT]
bantime  = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
F2BEOF

systemctl enable fail2ban
systemctl start fail2ban
log "=== Fail2ban configured ==="

# ── Avahi mDNS ────────────────────────────────────────────────────────────────
# Enable mDNS in nsswitch so that <hostname>.local resolves on the same network.
sed -i 's/^hosts:.*/hosts:          files mdns4_minimal [NOTFOUND=return] dns mdns4/' \
  /etc/nsswitch.conf 2>/dev/null || true

systemctl enable avahi-daemon
systemctl start avahi-daemon

# Advertise the opencode web terminal via mDNS (_http._tcp on port 80).
cat > /etc/avahi/services/opencode.service << 'AVAHIEOF'
<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name replace-wildcards="yes">opencode on %h</name>
  <service>
    <type>_http._tcp</type>
    <port>80</port>
    <txt-record>path=/terminal</txt-record>
  </service>
</service-group>
AVAHIEOF

systemctl restart avahi-daemon
log "=== Avahi mDNS configured ==="

# ── Coder user ────────────────────────────────────────────────────────────────
useradd -m -s /bin/bash coder || true
mkdir -p /home/coder/workspace /home/coder/www
chown -R coder:coder /home/coder

# Allow coder to reload the proxy config without a password.
echo 'coder ALL=(ALL) NOPASSWD: /usr/bin/systemctl reload caddy, /usr/bin/systemctl reload nginx, /usr/bin/caddy reload --config /etc/caddy/Caddyfile' \
  > /etc/sudoers.d/coder-proxy
chmod 0440 /etc/sudoers.d/coder-proxy
log "=== Coder user created ==="

# ── Node.js 20.x ──────────────────────────────────────────────────────────────
curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt-get install -y nodejs
log "=== Node.js installed: $(node --version) ==="

# ── opencode ──────────────────────────────────────────────────────────────────
# Install opencode globally via the official install script.
if curl -fsSL https://opencode.ai/install | bash; then
  # The installer places the binary in /root/.local/bin; expose it globally.
  OPENCODE_BIN=$(find /root/.local/bin /usr/local/bin -name opencode 2>/dev/null | head -1 || true)
  if [ -n "$OPENCODE_BIN" ] && [ "$OPENCODE_BIN" != "/usr/local/bin/opencode" ]; then
    cp "$OPENCODE_BIN" /usr/local/bin/opencode
    chmod +x /usr/local/bin/opencode
  fi
else
  log "opencode install script failed; falling back to npm"
  npm install -g opencode-ai 2>/dev/null || \
    log "WARNING: opencode could not be installed automatically."
fi
log "=== opencode installed ==="

# ── ttyd (browser terminal) ───────────────────────────────────────────────────
ARCH=$(dpkg --print-architecture)
TTYD_VERSION="1.7.7"
case "$ARCH" in
  amd64)   TTYD_ARCH="x86_64"  ;;
  arm64)   TTYD_ARCH="aarch64" ;;
  *)       log "Unsupported arch $ARCH for ttyd binary download; skipping."; TTYD_ARCH="" ;;
esac

if [ -n "$TTYD_ARCH" ]; then
  wget -qO /usr/local/bin/ttyd \
    "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${TTYD_ARCH}"
  chmod +x /usr/local/bin/ttyd
  log "=== ttyd installed ==="
fi

cat > /etc/systemd/system/ttyd.service << 'TTYDEOF'
[Unit]
Description=OpenCode Web Terminal (ttyd)
After=network.target

[Service]
Type=simple
User=coder
WorkingDirectory=/home/coder/workspace
ExecStart=/usr/local/bin/ttyd \
  --port 7681 \
  --interface 127.0.0.1 \
  bash
Restart=always
RestartSec=5
Environment=HOME=/home/coder
Environment=PATH=/usr/local/bin:/usr/bin:/bin:/home/coder/.local/bin

[Install]
WantedBy=multi-user.target
TTYDEOF

systemctl daemon-reload
systemctl enable ttyd
systemctl start ttyd 2>/dev/null || log "ttyd start deferred (binary may not exist)"
log "=== ttyd service configured ==="

# ── Project deploy helper ─────────────────────────────────────────────────────
# deploy-project.sh copies a build directory into ~/www/<name> so that Caddy/Nginx
# serves it at http://<server>/<name>. opencode can call this after building.
cat > /home/coder/deploy-project.sh << 'DEPLOYEOF'
#!/bin/bash
# Usage: ./deploy-project.sh <project-name> <build-directory>
# Example: ./deploy-project.sh myapp ./workspace/myapp/dist
set -euo pipefail

PROJECT="${1:?Usage: $0 <project-name> <build-dir>}"
BUILD_DIR="${2:?Usage: $0 <project-name> <build-dir>}"

if [ ! -d "$BUILD_DIR" ]; then
  echo "Error: build directory '$BUILD_DIR' does not exist." >&2
  exit 1
fi

TARGET="/home/coder/www/$PROJECT"
rm -rf "$TARGET"
cp -r "$BUILD_DIR" "$TARGET"
chmod -R a+rX "$TARGET"

echo "Deployed '$PROJECT' to $TARGET"
echo "Access at: http://$(curl -s --max-time 3 ifconfig.me 2>/dev/null || echo '<server-ip>')/$PROJECT"
DEPLOYEOF

chmod +x /home/coder/deploy-project.sh
chown coder:coder /home/coder/deploy-project.sh
log "=== deploy-project.sh installed ==="

{{ if eq .ProxyType "nginx" }}
# ── Nginx ─────────────────────────────────────────────────────────────────────
apt-get install -y nginx

cat > /etc/nginx/sites-available/opencode << 'NGINXEOF'
server {
    listen 80;
    server_name _;

    # Web terminal (WebSocket-capable proxy to ttyd)
    location /terminal/ {
        proxy_pass         http://127.0.0.1:7681/;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade    $http_upgrade;
        proxy_set_header   Connection "upgrade";
        proxy_set_header   Host       $host;
        proxy_read_timeout 86400;
    }

    # Static project files deployed by deploy-project.sh
    location / {
        root       /home/coder/www;
        index      index.html index.htm;
        try_files  $uri $uri/ $uri/index.html =404;
        autoindex  on;
    }
}
NGINXEOF

ln -sf /etc/nginx/sites-available/opencode /etc/nginx/sites-enabled/opencode
rm -f /etc/nginx/sites-enabled/default

nginx -t
systemctl enable nginx
systemctl start nginx
log "=== Nginx configured ==="

{{ else }}
# ── Caddy ─────────────────────────────────────────────────────────────────────
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  | tee /etc/apt/sources.list.d/caddy-stable.list
apt-get update
apt-get install -y caddy

{{ if .Domain }}
# Domain provided: Caddy will obtain a Let's Encrypt TLS certificate automatically.
cat > /etc/caddy/Caddyfile << 'CADDYEOF'
{{ .Domain }} {
    # Web terminal
    route /terminal* {
        uri strip_prefix /terminal
        reverse_proxy 127.0.0.1:7681 {
            transport http {
                read_timeout 0
            }
        }
    }

    # Static project files
    route /* {
        root * /home/coder/www
        file_server browse
    }
}

:80 {
    redir https://{{ .Domain }}{uri} permanent
}
CADDYEOF
{{ else }}
# No domain: serve over plain HTTP on port 80.
cat > /etc/caddy/Caddyfile << 'CADDYEOF'
:80 {
    # Web terminal (WebSocket support is automatic in Caddy v2)
    route /terminal* {
        uri strip_prefix /terminal
        reverse_proxy 127.0.0.1:7681 {
            transport http {
                read_timeout 0
            }
        }
    }

    # Static project files deployed by deploy-project.sh
    route /* {
        root * /home/coder/www
        file_server browse
    }
}
CADDYEOF
{{ end }}

systemctl enable caddy
systemctl start caddy
log "=== Caddy configured ==="
{{ end }}

# ── Tailscale (optional) ──────────────────────────────────────────────────────
{{ if .TailscaleKey }}
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up \
  --authkey="{{ .TailscaleKey }}" \
  --hostname="{{ .Hostname }}" \
  --accept-routes
log "=== Tailscale joined tailnet as {{ .Hostname }} ==="
{{ end }}

# ── Final summary ─────────────────────────────────────────────────────────────
PUBLIC_IP=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || echo '<public-ip>')

log "=== opencode VPS setup complete ==="
log ""
log "  Hostname   : $HOSTNAME_VAL"
log "  Public IP  : $PUBLIC_IP"
log "  Terminal   : http://$PUBLIC_IP/terminal"
log "  mDNS       : http://$HOSTNAME_VAL.local/terminal  (same network / Tailscale)"
{{ if .TailscaleKey }}
log "  Tailscale  : http://$HOSTNAME_VAL/terminal"
{{ end }}
{{ if .Domain }}
log "  Domain     : https://{{ .Domain }}/terminal"
{{ end }}
log ""
log "  Deploy a project: ~/deploy-project.sh <name> <build-dir>"
`
