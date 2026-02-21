# How to use vpsm opencode

This guide walks through the end-to-end workflow: provisioning a secured
opencode VPS, accessing the browser terminal, building a project with AI
assistance, and deploying it behind the reverse proxy.

---

## Prerequisites

- `vpsm` installed and on your `$PATH`
- A Hetzner Cloud account with an API token
- An SSH key registered with Hetzner (upload one with `vpsm sshkey upload` if
  needed)

---

## Step 1 — Authenticate

Store your Hetzner API token in the OS keychain:

```bash
vpsm auth login hetzner
# Paste your API token when prompted
```

Set Hetzner as the default provider so you do not need to pass `--provider` on
every command:

```bash
vpsm config set default-provider hetzner
```

---

## Step 2 — Create an opencode VPS

### Option A: Interactive wizard

Run the command with no flags. A step-by-step wizard will guide you through
every option:

```bash
vpsm opencode create
```

The wizard collects:

1. Server name (becomes the hostname and the mDNS name)
2. Datacenter location
3. Server type — `cpx21` (2 vCPU / 4 GB) or larger is recommended
4. SSH keys to inject
5. Reverse proxy: Caddy or Nginx
6. Tailscale auth key (optional — enables remote mDNS access)
7. Domain name (optional — enables automatic HTTPS with Caddy)

Review the summary and confirm to provision.

### Option B: Flags

```bash
vpsm opencode create \
  --name     mydev   \
  --type     cpx21   \
  --location fsn1    \
  --ssh-key  my-key  \
  --proxy    caddy
```

### With Tailscale (recommended for remote access)

Generate a one-time auth key in the [Tailscale admin panel](https://login.tailscale.com/admin/settings/keys)
and pass it at creation time:

```bash
vpsm opencode create \
  --name         mydev \
  --tailscale-key tskey-auth-xxxxxxxxxxxxxx
```

The server joins your tailnet during cloud-init. Once complete, every device on
your tailnet can reach the terminal at `http://mydev/terminal` — no IP address
needed.

### With a public domain and automatic HTTPS

Point your domain's A record at the server IP **before** cloud-init runs (or
within a few minutes of boot), then pass `--domain`:

```bash
vpsm opencode create \
  --name   mydev               \
  --domain dev.example.com
```

Caddy will obtain a Let's Encrypt certificate and serve the terminal at
`https://dev.example.com/terminal`.

---

## Step 3 — Wait for cloud-init

The API immediately reports the server as running, but cloud-init is still
installing packages in the background. The full setup takes **2–3 minutes**.

Monitor progress over SSH:

```bash
ssh root@<server-ip> 'tail -f /var/log/cloud-init-output.log'
```

You will see `[vpsm-opencode] === opencode VPS setup complete ===` when done.

---

## Step 4 — Open the browser terminal

Navigate to the terminal URL printed by `vpsm opencode create`:

| Access method | URL |
|---|---|
| Public IP | `http://<server-ip>/terminal` |
| mDNS (same network) | `http://<hostname>.local/terminal` |
| Tailscale MagicDNS | `http://<hostname>/terminal` |
| Domain + HTTPS | `https://<domain>/terminal` |

A full bash session opens in your browser, running as the `coder` user in
`/home/coder/workspace`.

---

## Step 5 — Use opencode to build a project

Inside the browser terminal, run opencode with a natural-language prompt:

```bash
opencode "build me a React todo app with local storage persistence"
```

opencode will:

1. Scaffold the project under the current directory
2. Install dependencies
3. Build the project

The working directory `/home/coder/workspace` is where all project files live.

### Tips

- Give opencode a specific directory to work in:
  ```bash
  mkdir todo-app && cd todo-app
  opencode "build a React todo app here, output the built files to ./dist"
  ```

- Ask opencode to fix errors or add features by continuing the session or
  starting a new one in the same directory:
  ```bash
  opencode "add dark mode support to the todo app"
  ```

- Inspect the generated files at any time:
  ```bash
  ls workspace/todo-app/dist
  ```

---

## Step 6 — Deploy the project

Once the project is built, deploy it so the reverse proxy serves it:

```bash
~/deploy-project.sh todo-app ./workspace/todo-app/dist
```

The script copies the build output into `~/www/todo-app` and sets the correct
permissions. The project is immediately served at:

```
http://<server-ip>/todo-app
http://<hostname>.local/todo-app   (mDNS)
http://<hostname>/todo-app         (Tailscale)
```

### Deploy a different project

Each call to `deploy-project.sh` with a new name creates an independent
subdirectory. You can run multiple projects at once:

```bash
~/deploy-project.sh blog      ./workspace/blog/public
~/deploy-project.sh dashboard ./workspace/dashboard/build
~/deploy-project.sh api-docs  ./workspace/api-docs/out
```

All three are then available under separate paths on the same server.

### Serve a Node.js or other backend app

If the project runs a local server rather than producing static files, add a
reverse proxy block to the Caddy or Nginx config:

**Caddy** — append to `/etc/caddy/Caddyfile`:

```caddy
# Append inside the existing :80 { } block
route /myapp* {
    uri strip_prefix /myapp
    reverse_proxy 127.0.0.1:3001
}
```

Then reload:

```bash
sudo systemctl reload caddy
```

**Nginx** — append to `/etc/nginx/sites-available/opencode`:

```nginx
location /myapp/ {
    proxy_pass http://127.0.0.1:3001/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
}
```

Then reload:

```bash
sudo systemctl reload nginx
```

---

## Common tasks

### SSH into the server

Use the standard `ssh` command with the public IP printed at creation time:

```bash
ssh coder@<server-ip>          # as the coder user
ssh root@<server-ip>           # as root (key auth only — password auth is disabled)
```

### Check service status

```bash
systemctl status ttyd        # browser terminal daemon
systemctl status caddy       # or: systemctl status nginx
systemctl status avahi-daemon
systemctl status fail2ban
systemctl status tailscaled  # only if Tailscale was installed
```

### View fail2ban bans

```bash
sudo fail2ban-client status sshd
```

### Restart the browser terminal

```bash
sudo systemctl restart ttyd
```

### Add an SSH key after creation

```bash
# From your local machine — append a public key to the authorized_keys file
ssh-copy-id -i ~/.ssh/id_ed25519.pub coder@<server-ip>
```

### Update opencode

```bash
# Inside the browser terminal or via SSH
curl -fsSL https://opencode.ai/install | bash
# Then copy the new binary to a globally accessible location if needed:
sudo cp ~/.local/bin/opencode /usr/local/bin/opencode
```

---

## Troubleshooting

### The terminal URL returns a connection error

Cloud-init is still running. Wait for it to finish and check progress:

```bash
ssh root@<server-ip> 'tail -50 /var/log/cloud-init-output.log'
```

### `<hostname>.local` does not resolve

mDNS only works on the same L2 network segment. From a remote machine,
use Tailscale (`--tailscale-key`) for name resolution, or access the server
by IP address.

### The browser terminal shows a blank page or crashes

The `ttyd` service may have failed to start (e.g. the binary was not
downloaded because the architecture was unsupported). Check:

```bash
ssh root@<server-ip> 'systemctl status ttyd; journalctl -u ttyd -n 50'
```

### Caddy is not obtaining a TLS certificate

Ensure:
1. The domain's A record points to the server's public IP.
2. Port 80 is reachable from the internet (UFW allows it; no additional
   firewall blocks it).
3. The domain was passed at creation time via `--domain`. If it was not, edit
   `/etc/caddy/Caddyfile` to add the domain block, then run
   `sudo systemctl reload caddy`.

### opencode is not found in the terminal

The install script may have placed the binary in `~/.local/bin` for the `root`
user but not copied it to `/usr/local/bin`. Fix:

```bash
ssh root@<server-ip> \
  'find /root/.local/bin -name opencode -exec cp {} /usr/local/bin/opencode \;'
```

---

## Cleaning up

Delete the server when you are done to stop billing:

```bash
vpsm server delete
```

Select the opencode server from the interactive list, or pass `--name` directly:

```bash
vpsm server delete --name mydev
```

> **Note:** Deletion is permanent. Download any project files you want to keep
> before deleting the server.
