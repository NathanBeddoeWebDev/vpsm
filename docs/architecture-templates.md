# Architecture Plan: VPS Template System

**Status:** Proposal
**Scope:** `vpsm` CLI + new `vpsm-templates` registry repo

---

## Problem statement

The current `vpsm opencode` implementation is a one-off command — a hard-coded
cloud-init script wired to a Cobra subcommand. Adding atproto PDS and
Vaultwarden the same way would mean three disconnected command trees, three
separate TUI wizards, and three places to duplicate the "server creation +
user-data rendering" loop.

The right abstraction is a **template**: a versioned, declarative description
of a VPS workload that drives a generic creation flow. The CLI becomes a
rendering engine; the templates live in a separate registry it can query.

---

## High-level architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  vpsm CLI                                                        │
│                                                                  │
│  vpsm template list / show                                       │
│  vpsm server create --template opencode                          │
│                     --template atproto-pds                       │
│                     --template vaultwarden                       │
│                                                                  │
│  ┌──────────────────┐    ┌─────────────────────────────────────┐ │
│  │  template client  │───▶│  dynamic TUI form (from schema)    │ │
│  │  (HTTP + cache)   │    │  + standard catalog wizard         │ │
│  └──────────────────┘    └─────────────────────────────────────┘ │
│           │                              │                        │
│           │                              ▼                        │
│           │               domain.CreateServerOpts                 │
│           │               (UserData = rendered template)          │
│           │                              │                        │
│           │                              ▼                        │
│           │                  provider.CreateServer()              │
└───────────┼──────────────────────────────────────────────────────┘
            │
            ▼ HTTPS
┌───────────────────────────┐
│  vpsm-templates registry  │
│                           │
│  Phase 1: static files    │  GET /index.json
│  (GitHub Pages / CDN)     │  GET /templates/opencode/template.yaml
│                           │
│  Phase 2: Go API server   │  GET /api/v1/templates
│  (search, versioning,     │  GET /api/v1/templates/opencode
│   community contrib)      │  GET /api/v1/templates/opencode/versions
└───────────────────────────┘
```

---

## Template schema

Templates are YAML documents. The CLI unmarshals them into a Go struct; the
registry serves them verbatim.

```yaml
# templates/opencode/template.yaml

id:          opencode
name:        opencode Dev Server
version:     1.0.0
description: >
  Secured VPS with the opencode AI coding assistant, a browser-based
  terminal (ttyd), Avahi mDNS service discovery, and a Caddy or Nginx
  reverse proxy. SSH hardening, UFW, and fail2ban are pre-configured.
category:    development
tags:        [ai, coding, browser-terminal, caddy, nginx]
author:      vpsm
license:     MIT

# ── Server requirements ────────────────────────────────────────────────────────
server:
  recommended_type: cpx21
  min_cpu:          2
  min_ram_gb:       4
  min_disk_gb:      20
  default_image:    ubuntu-24.04
  supported_images: [ubuntu-24.04]

# ── Ports opened in the firewall ───────────────────────────────────────────────
ports:
  - { port: 22,   proto: tcp, description: SSH }
  - { port: 80,   proto: tcp, description: HTTP }
  - { port: 443,  proto: tcp, description: HTTPS }
  - { port: 5353, proto: udp, description: mDNS }

# ── Access points (shown to user after creation) ───────────────────────────────
access:
  - { path: /terminal, description: Browser terminal, protocol: http }

# ── Parameters (drives the TUI wizard and --param flags) ──────────────────────
parameters:
  - id:          proxy_type
    name:        Reverse proxy
    type:        enum
    default:     caddy
    description: The reverse proxy to install and configure.
    options:
      - { value: caddy,  label: "Caddy  (automatic HTTPS, recommended)" }
      - { value: nginx,  label: "Nginx  (classic, widely supported)"    }

  - id:          tailscale_key
    name:        Tailscale auth key
    type:        secret
    required:    false
    description: >
      Joins the server to your tailnet for VPN access and MagicDNS.
      Generate one at https://login.tailscale.com/admin/settings/keys.
    placeholder: tskey-auth-...

  - id:          domain
    name:        Public domain
    type:        string
    required:    false
    pattern:     '^[a-z0-9][a-z0-9.-]+\.[a-z]{2,}$'
    description: >
      FQDN for automatic HTTPS via Let's Encrypt (Caddy only).
      The A record must point to the server IP before cloud-init runs.
    placeholder: dev.example.com

# ── Cloud-init user data (Go text/template syntax) ────────────────────────────
user_data: |
  #!/bin/bash
  # ... parameterised cloud-init script
  # Substitution uses {{.Hostname}}, {{.Param "proxy_type"}}, etc.
```

### The three initial templates — parameter sketches

#### `atproto-pds` (AT Protocol Personal Data Server — self-hosted Bluesky)

```yaml
id:       atproto-pds
category: social
server:
  recommended_type: cpx21
  min_cpu:          2
  min_ram_gb:       4
  min_disk_gb:      40      # SQLite DB grows over time
  default_image:    ubuntu-24.04
ports:
  - { port: 22,  proto: tcp, description: SSH }
  - { port: 80,  proto: tcp, description: HTTP (ACME challenge) }
  - { port: 443, proto: tcp, description: HTTPS (PDS + XRPC) }
access:
  - { path: /, description: PDS XRPC endpoint, protocol: https }
parameters:
  - { id: pds_hostname,     type: string, required: true,
      description: "Public FQDN for the PDS (e.g. pds.example.com). Required." }
  - { id: pds_admin_email,  type: string, required: true,
      description: "Admin email — used for Let's Encrypt and account recovery." }
  - { id: admin_password,   type: secret, required: true,
      description: "Password for the PDS admin account." }
  - { id: invite_required,  type: bool,   default: true,
      description: "Require an invite code to create new accounts." }
# Installs: Docker, Caddy, official atproto/pds Docker image
# Caddy handles TLS termination and proxies to the container on :3000
```

#### `vaultwarden` (self-hosted Bitwarden-compatible password manager)

```yaml
id:       vaultwarden
category: security
server:
  recommended_type: cpx11   # very lightweight
  min_cpu:          1
  min_ram_gb:       1
  min_disk_gb:      10
  default_image:    ubuntu-24.04
ports:
  - { port: 22,  proto: tcp, description: SSH }
  - { port: 80,  proto: tcp, description: HTTP (ACME challenge) }
  - { port: 443, proto: tcp, description: HTTPS (Vaultwarden) }
access:
  - { path: /, description: Vaultwarden web vault, protocol: https }
parameters:
  - { id: domain,           type: string, required: true,
      description: "FQDN for HTTPS. Bitwarden clients require a valid TLS cert." }
  - { id: admin_token,      type: secret, required: true,
      description: "Token for the /admin panel. Use a long random string." }
  - { id: signups_allowed,  type: bool,   default: false,
      description: "Allow public registrations. Disable after creating your account." }
  - { id: tailscale_key,    type: secret, required: false,
      description: "Optionally restrict access to your tailnet only." }
# Installs: Docker, Caddy, vaultwarden/server Docker image
# Data persisted at /opt/vaultwarden/data (bind-mounted into container)
```

---

## Part 1 — `vpsm` CLI changes

### New package structure

```
internal/
└── template/
    ├── domain/
    │   └── types.go          Template, Parameter, ParameterType, ParamOption,
    │                          ServerReqs, AccessPoint, RenderContext
    ├── client/
    │   └── client.go         HTTP client: FetchIndex(), FetchTemplate(id, version)
    ├── registry/
    │   └── registry.go       Combines remote + local file templates; cache layer
    └── tui/
        └── params.go         BuildParamForm([]Parameter) → *huh.Form + values map

cmd/commands/
└── template/
    ├── root.go               vpsm template
    ├── list.go               vpsm template list [--category x] [--search q]
    └── show.go               vpsm template show <id>[@version]
```

The existing `cmd/commands/server/create.go` gains a single new optional flag:

```
--template <id>[@version]    Apply a template (fetches from registry, renders user_data)
--param <key=value>          Override a template parameter (repeatable)
```

The existing `cmd/commands/opencode/` package is **removed** once the template
system is in place and an `opencode` template is published to the registry.

### Domain types (`internal/template/domain/types.go`)

```go
// Template is the full parsed representation of a template.yaml.
type Template struct {
    ID          string        `yaml:"id"`
    Name        string        `yaml:"name"`
    Version     string        `yaml:"version"`
    Description string        `yaml:"description"`
    Category    string        `yaml:"category"`
    Tags        []string      `yaml:"tags"`
    Author      string        `yaml:"author"`

    Server      ServerReqs    `yaml:"server"`
    Ports       []PortRule    `yaml:"ports"`
    Access      []AccessPoint `yaml:"access"`
    Parameters  []Parameter   `yaml:"parameters"`
    UserData    string        `yaml:"user_data"`
}

type ParameterType string
const (
    ParamTypeString  ParameterType = "string"
    ParamTypeEnum    ParameterType = "enum"
    ParamTypeSecret  ParameterType = "secret"
    ParamTypeBool    ParameterType = "bool"
)

type Parameter struct {
    ID          string        `yaml:"id"`
    Name        string        `yaml:"name"`
    Type        ParameterType `yaml:"type"`
    Default     string        `yaml:"default"`
    Required    bool          `yaml:"required"`
    Description string        `yaml:"description"`
    Placeholder string        `yaml:"placeholder"`
    Pattern     string        `yaml:"pattern"` // regex for validation
    Options     []ParamOption `yaml:"options"` // for enum type
}

// RenderContext is passed to text/template when rendering UserData.
type RenderContext struct {
    Hostname string
    Params   map[string]string
}

// Param is a convenience method for use inside templates: {{.Param "key"}}
func (r RenderContext) Param(key string) string { return r.Params[key] }
```

### Registry client (`internal/template/client/client.go`)

```go
// Client fetches templates from the registry over HTTPS.
type Client struct {
    baseURL    string       // e.g. "https://templates.vpsm.sh"
    httpClient *http.Client
    cache      *cache.Cache
}

func (c *Client) ListTemplates(ctx context.Context) ([]TemplateMeta, error)
func (c *Client) FetchTemplate(ctx context.Context, id, version string) (*Template, error)
```

**Caching strategy:** reuse the existing `internal/cache` package.

- Index (`ListTemplates`): TTL 1 hour (same as catalog caching)
- Individual templates: TTL 24 hours (they change infrequently)
- Cache key: `template-index` and `template-{id}-{version}`
- `vpsm template list --refresh` forces re-fetch by calling `cache.Invalidate`

**Fallback:** if the registry is unreachable and a cached copy exists (even
expired), use it and warn the user. If there is no cache, return an error
pointing to `--template-file` as an alternative.

### Dynamic TUI form (`internal/template/tui/params.go`)

The key design win: the TUI for template parameters is **generated from the
schema** rather than hard-coded.

```go
// BuildParamForm creates a huh.Form from a template's parameter list.
// values is pre-populated with defaults and any --param overrides.
// Returns the form and a pointer to the values map (mutated by the form).
func BuildParamForm(params []domain.Parameter, values map[string]string) (*huh.Form, error)
```

Mapping from `ParameterType` to `huh` field type:

| ParameterType | huh field | Notes |
|---|---|---|
| `string` | `huh.NewInput()` | Pattern validated with `Validate()` func |
| `enum` | `huh.NewSelect[string]()` | Options from `Parameter.Options` |
| `secret` | `huh.NewInput().EchoMode(huh.EchoModePassword)` | Masked input |
| `bool` | `huh.NewConfirm()` | Bound to `"true"` / `"false"` string |

Required parameters with no default emit a `Validate(huh.ValidateNotEmpty())`
call. Optional parameters render with the description hinting they can be
skipped.

### Updated `server create` flow with `--template`

```
vpsm server create --template opencode --param proxy_type=nginx --param domain=dev.example.com
```

1. Resolve provider (existing logic)
2. Fetch template from registry (new)
3. Validate and merge `--param` overrides with template defaults
4. If any **required** parameters are missing and we have a TTY:
   - Run `template/tui.BuildParamForm` to collect them interactively
5. Run the existing **catalog wizard** (location, server type, SSH keys) seeded
   with `template.Server.RecommendedType` and `template.Server.DefaultImage` as
   defaults — so the user sees sensible pre-selections without being forced to
   accept them
6. Render `template.UserData` via `text/template` with a `RenderContext{Hostname, Params}`
7. Call `provider.CreateServer` with the rendered `UserData` plus the labels
   `managed-by=vpsm` and `template=<id>@<version>`
8. Print access URLs from `template.Access` (substituting the real IP/hostname)

### New `vpsm template` commands

```
vpsm template list                  # List all templates
vpsm template list --category dev   # Filter by category
vpsm template list --search vault   # Fuzzy search name/description/tags
vpsm template show opencode         # Full details: params, access, ports
vpsm template show opencode --user-data  # Also prints the raw cloud-init script
```

---

## Part 2 — `vpsm-templates` registry repo

### Phase 1: Static file registry (ship first)

```
vpsm-templates/
├── templates/
│   ├── opencode/
│   │   └── template.yaml
│   ├── atproto-pds/
│   │   └── template.yaml
│   └── vaultwarden/
│       └── template.yaml
├── index.json          ← generated by CI from all template.yaml files
├── schema.json         ← JSON Schema for template.yaml (for authoring + validation)
└── README.md
```

**`index.json`** is generated by a GitHub Actions workflow on every merge to
`main`. It contains the metadata from each `template.yaml` (everything except
`user_data`) so the CLI can list templates with a single small HTTP request.

```json
{
  "generated_at": "2026-02-21T10:00:00Z",
  "templates": [
    {
      "id": "opencode",
      "name": "opencode Dev Server",
      "version": "1.0.0",
      "description": "...",
      "category": "development",
      "tags": ["ai", "coding"],
      "url": "https://raw.githubusercontent.com/vpsm-sh/vpsm-templates/main/templates/opencode/template.yaml"
    }
  ]
}
```

The CLI fetches index from:
```
https://raw.githubusercontent.com/vpsm-sh/vpsm-templates/main/index.json
```
and individual templates from the `url` field in each entry.

**Default registry URL** is compiled into the CLI binary via ldflags so it can
be updated in releases without a code change:

```go
// cmd/commands/template/root.go
var defaultRegistryURL = "https://raw.githubusercontent.com/vpsm-sh/vpsm-templates/main"
```

Users can override it:
```bash
vpsm config set template-registry https://raw.githubusercontent.com/my-org/my-templates/main
```

### Phase 2: Go API server (when the static approach hits its limits)

A separate Go HTTP server in the same repo (or a sibling repo) provides:

```
GET  /api/v1/templates                → []TemplateMeta  (same as index.json)
GET  /api/v1/templates/:id            → Template (latest version)
GET  /api/v1/templates/:id@:version   → Template (specific version)
GET  /api/v1/templates/:id/versions   → []string
GET  /api/v1/categories               → []string
GET  /api/v1/search?q=vault           → []TemplateMeta
```

The server reads templates from the Git-backed `templates/` directory — no
database required. It adds value over raw GitHub access by providing:

- **Search** across name, description, and tags
- **Version history** by inspecting Git log for a template file
- **CDN caching headers** with appropriate `Cache-Control` and `ETag`
- **Template validation** on push (rejects malformed YAML)
- **Future:** authenticated private templates, community submission queue

The CLI detects Phase 2 by checking whether the registry URL responds to
`/api/v1/templates` and falls back to the Phase 1 path if it doesn't.

---

## Migration path

1. **Keep `cmd/commands/opencode/` as-is** in the `claude/vps-opencode-deployment-nt4qA`
   branch while the template system is being built.
2. Add the template system to a new branch (`feat/template-system`).
3. Publish an `opencode` template to the registry and verify `vpsm server create
   --template opencode` produces the same result as the current `vpsm opencode create`.
4. Deprecate `vpsm opencode create` (print a notice pointing to `--template`).
5. Remove `cmd/commands/opencode/` and `internal/opencode/` in a subsequent
   major version bump.

---

## Configuration additions

Two new config keys (added to `internal/config/keys.go`):

| Key | Default | Description |
|---|---|---|
| `template-registry` | *(compiled-in URL)* | Base URL for the template registry |
| `template-cache-ttl` | `24h` | How long individual templates are cached |

---

## Open questions

1. **Template signing.** Should templates be signed so the CLI can verify they
   haven't been tampered with in transit? A simple approach: ship a public key in
   the binary and sign `index.json` + each `template.yaml` with it on release.

2. **Local templates.** `vpsm server create --template-file ./my-template.yaml`
   would let users apply private or work-in-progress templates without publishing
   them. Low effort to add given the schema is already defined.

3. **Template versioning in the static phase.** Without the API server there is
   no easy version history. Options: (a) accept that only `latest` is available
   in Phase 1, (b) use Git tags and point to tag-scoped raw URLs
   (`…/vpsm-templates/v1.0.0/templates/opencode/template.yaml`).

4. **Parameter secrets in CI.** The `atproto-pds` and `vaultwarden` templates
   require passwords/tokens. These should never appear in the `--param` flags
   in a form that gets logged. The TUI masked-input path is fine; the flag path
   should support reading from env vars (`--param admin_token=$VAULT_TOKEN`).

5. **Repo name.** `vpsm-templates` keeps the two repos clearly related.
   An alternative is `opentemplate` if the registry is intended to grow beyond
   the vpsm project — but that adds scope/complexity and should be a deliberate
   future decision.
