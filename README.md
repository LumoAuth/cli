# LumoAuth CLI

A comprehensive command-line interface for managing your [LumoAuth](https://lumoauth.dev) organization — users, roles, groups, OAuth apps, AI agents, webhooks, audit logs, permissions, settings, sessions, and more.

Designed for **org admins**, **AI coding agents**, and **developers building local-first** with LumoAuth.

---

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
lumo login --org acme-corp
```

Two commands. The installer drops the binary in `~/.local/bin`, and `lumo login` opens your browser to confirm the device-flow approval. No API key paste, no `config init` wizard — credentials land in `~/.lumoauth/credentials.yaml` with mode 0600 and you're ready to go.

For scripted/CI use see the [API key authentication](#authentication) section below.

---

## What's new

The CLI now covers the full first-15-minutes loop without leaving the terminal:

| Command | Purpose |
|---|---|
| `lumo login` / `logout` / `whoami` | Browser-based OAuth 2.0 device-flow sign-in (RFC 8628). |
| `lumo dev start` / `dev list` / `dev stop` | Spawn ephemeral sandbox tenants — Neon-branch style — for branch previews and PR environments. |
| `lumo tunnel --to <url>` | Forward live webhook events to localhost (like `stripe listen`). |
| `lumo init --framework <name>` | Scaffold a working starter project (next, express, fastapi, go) wired to your org. |
| `lumo --version` | Print version, commit, build date, Go version. |

The full pre-existing surface (users, roles, groups, apps, agents, webhooks, permissions, settings, sessions, social, raw `api`) continues to work unchanged.

---

## Installation

### One-line install (recommended)

Works on **Linux**, **macOS**, and **Windows (WSL)** — no `sudo` required.

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
```

The script:

1. Detects your OS and architecture (amd64 / arm64).
2. Downloads the latest release from GitHub.
3. Verifies the SHA-256 checksum.
4. Installs the `lumo` binary to `~/.local/bin`.
5. Adds `~/.local/bin` to your shell's `PATH` (bash, zsh, fish, or sh) if it isn't already.

Restart your shell (or `source` your profile) and run `lumo --version` to confirm.

### Build from source

**Prerequisites:** Go 1.22+

```bash
git clone https://github.com/LumoAuth/cli.git && cd cli
go build -o lumo .

# Optional: install into $GOPATH/bin
go install .
```

---

## Authentication

The CLI accepts auth in two ways. Pick whichever fits your context — both work everywhere.

### A. Device-flow login (recommended for humans)

```bash
lumo login --org acme-corp
```

Behind the scenes:

1. CLI calls `POST /orgs/{org}/api/v1/oauth/device_authorization` with the well-known first-party client `lumoauth-cli`.
2. CLI prints a `user_code` and opens your browser to the verification URL.
3. You approve in the dashboard.
4. CLI polls the token endpoint and stores tokens at `~/.lumoauth/credentials.yaml` (0600).

Subsequent commands use the stored bearer token automatically — no flags required.

```bash
lumo whoami    # Shows current org, user, expiry
lumo logout    # Wipes credentials.yaml
```

The first time you log in to a tenant, the server lazy-creates a `lumoauth-cli` public OAuth client for it (device_code + refresh_token grants, no PKCE — the device flow already binds approval to the verified user code).

### B. API key (recommended for CI / scripting)

```bash
export LUMO_API_KEY=lmk_your_key_here
export LUMO_ORG_ID=acme-corp
lumo users list
```

API keys live at `https://<your-lumoauth>/orgs/<orgId>/portal/settings/api-keys`. Use scoped keys — create one per CI job, revoke when no longer needed.

### Configuration precedence

| Priority | Method | Example |
|----------|--------|---------|
| 1 (highest) | CLI flags | `--api-key lmk_xxx --org-id acme-corp` |
| 2 | Environment variables | `LUMO_API_KEY`, `LUMO_ORG_ID`, `LUMO_BASE_URL` |
| 3 | Config file | `~/.lumoauth/config.yaml` |
| 4 (fallback) | Stored credentials | `~/.lumoauth/credentials.yaml` (from `lumo login`) |

When no API key is set anywhere, the CLI uses the credentials file. Stored credentials inherit `org_id` and `base_url` so you don't have to repeat them.

### Config file format

```yaml
# ~/.lumoauth/config.yaml — manual settings (no secrets)
org_id: acme-corp
base_url: https://app.lumoauth.dev   # or https://eu.app.lumoauth.dev
format: table
insecure: false
```

```yaml
# ~/.lumoauth/credentials.yaml — managed by `lumo login`/`logout` (0600)
base_url: https://app.lumoauth.dev
org_id: acme-corp
access_token: eyJhbGciOi...
refresh_token: ...
expires_at: 2026-05-08T17:00:00Z
token_type: Bearer
```

Manage settings via:

```bash
lumo config init          # Interactive setup wizard
lumo config show          # Display current configuration
lumo config set org_id acme-corp
```

---

## Commands

### Global flags

```
--api-key string    API key (overrides LUMO_API_KEY)
--org-id string     Organization ID (overrides LUMO_ORG_ID)
--base-url string   Base URL (overrides LUMO_BASE_URL)
-o, --output string Output format: table, json, yaml (default: table)
--insecure          Skip TLS verification (useful for local dev)
-q, --quiet         Suppress non-essential output
-v, --verbose       Enable verbose output
```

### `login` / `logout` / `whoami`

```bash
lumo login [--org acme-corp] [--no-browser] [--insecure]
lumo logout
lumo whoami
```

`--no-browser` prints the verification URL instead of trying to open a browser — useful in headless or remote environments.

### `dev` — Ephemeral sandbox tenants

Spawn throwaway tenants for branch previews, PR environments, or scratch experiments. Each sandbox is a separate, fully-isolated tenant with its own slug; the daily cleanup cron deletes any sandbox past its TTL (default 24h, max 7 days).

```bash
lumo dev start [--name pr-1234] [--ttl-hours 24]
lumo dev list
lumo dev stop sandbox-pr-1234-7a3b21
```

`lumo dev start` prints the sandbox slug. Use it like any other org by passing `--org-id` or by exporting `LUMO_ORG_ID`. The sandbox can be destroyed early with `lumo dev stop`; only the sandbox's owner (the user who created it) can destroy it.

### `tunnel` — Forward webhooks to localhost

```bash
lumo tunnel --to http://localhost:3000/webhooks
lumo tunnel --to http://localhost:8000/hooks --filter user.created --filter user.deleted
```

Equivalent to `stripe listen --forward-to`. Behind the scenes:

1. CLI asks the server to register a transient `tunnel://<session>` webhook subscribed to `*`.
2. CLI opens an SSE stream of that webhook's deliveries.
3. Each event is replayed as a POST to the `--to` URL with `X-LumoAuth-Event` and `X-LumoAuth-Delivery` headers.
4. On Ctrl+C, the tunnel webhook is deleted server-side.

The CLI auto-reconnects when the server's max-stream-duration expires.

### `init` — Scaffold a starter project

```bash
lumo init --framework next [--dir ./my-app] [--org acme-corp]
lumo init --framework express
lumo init --framework fastapi
lumo init --framework go
```

Generates a minimal-but-working project wired to your org. `.env` files are pre-filled with the org slug and base URL from your current credentials.

| Framework | What you get |
|---|---|
| `next` | Next.js 14 (App Router) + `@lumoauth/react`, with `<SignIn />` and `<UserButton />` wired up. |
| `express` | Express + `@lumoauth/sdk/express` middleware, session cookie, `requireAuth` example. |
| `fastapi` | FastAPI + `lumoauth.fastapi` router, `get_current_user` dependency. |
| `go` | Go (chi) + `lumo-auth-go` middleware, session-cookie protected routes. |

Existing files are left alone unless `--force` is passed.

### Users

```bash
lumo users list [--search "john"] [--role admin] [--blocked] [--page 1] [--limit 25]
lumo users get <user-id-or-email>
lumo users create --email user@example.com [--name "John Doe"] [--password pw] [--roles admin,editor]
lumo users update <id> [--name "New Name"] [--email new@email.com]
lumo users delete <id>
lumo users block <id>
lumo users unblock <id>
lumo users set-password <id> --password "newpassword"
lumo users mfa-reset <id>

lumo users roles get <user-id>
lumo users roles set <user-id> --roles admin,editor
lumo users groups get <user-id>
lumo users groups set <user-id> --groups team-a,team-b
```

### Roles

```bash
lumo roles list
lumo roles get <id-or-slug>
lumo roles create --name "Editor" [--description "..."] [--permissions doc.edit,doc.view]
lumo roles update <id> [--name "New Name"] [--permissions p1,p2]
lumo roles delete <id>
```

### Groups

```bash
lumo groups list [--search "engineering"]
lumo groups get <id-or-slug>
lumo groups create --name "Engineering" [--description "..."]
lumo groups update <id> [--name "New Name"]
lumo groups delete <id>

lumo groups members list <group-id>
lumo groups members add <group-id> --users user1,user2
lumo groups members remove <group-id> --users user1,user2
```

### OAuth applications

```bash
lumo apps list [--search "my-app"]
lumo apps get <client-id>
lumo apps create --name "My App" [--type web|spa|native|m2m] [--redirect-uris http://localhost:3000/callback]
lumo apps update <id> [--name "Updated App"]
lumo apps delete <id>
lumo apps rotate-secret <id>
```

### AI agents

```bash
lumo agents list [--search "assistant"]
lumo agents get <agent-id>
lumo agents create --name "My Agent" [--type ai_assistant] [--capabilities read_data,write_data]
lumo agents update <id> [--name "Updated Agent"]
lumo agents delete <id>
lumo agents enable <id>
lumo agents disable <id>
lumo agents rotate-credentials <id>
lumo agents token <id> [--ttl 3600]    # mint a test bearer token
```

### Permissions

```bash
lumo permissions list
lumo permissions get <id-or-slug>
lumo permissions create --slug "document.edit" [--name "Edit Documents"]
lumo permissions update <id> [--name "New Name"]
lumo permissions delete <id>
```

### Webhooks

```bash
lumo webhooks list
lumo webhooks get <id>
lumo webhooks create --url https://example.com/hook --events user.created,user.login [--secret s3cr3t]
lumo webhooks update <id> [--url "..."] [--events e1,e2]
lumo webhooks delete <id>
```

### Audit logs

```bash
lumo logs list [--action login] [--user <user-id>] [--from 2025-01-01] [--to 2025-12-31] [--status success]
lumo logs get <log-id>
lumo logs stats [--from 2025-01-01] [--to 2025-12-31]
lumo logs export [--export-format csv|json] [--from ...] [--to ...]
```

### Settings

```bash
lumo settings get tenant
lumo settings get auth
lumo settings get branding
lumo settings get ai

lumo settings update tenant --name "Acme Corp" --display-name "Acme Corporation"
lumo settings update auth --mfa-required --session-lifetime 86400
lumo settings update branding --primary-color "#5865F2" --logo-url "https://example.com/logo.png"
lumo settings update ai --data '{"agentRegistrationEnabled": true}'
```

### Sessions & tokens

```bash
lumo sessions list [--user <user-id>]
lumo sessions revoke <session-id>
lumo sessions revoke-all --user <user-id>

lumo tokens list [--user <user-id>] [--client <client-id>]
lumo tokens revoke <token-id>
```

### Social login providers

```bash
lumo social list
lumo social get <id>
lumo social create --provider google --client-id <id> --client-secret <secret> [--scopes email,profile]
lumo social delete <id>
```

### Raw API access

For endpoints not covered by named commands, or for use by AI agents:

```bash
lumo api GET /orgs/<orgId>/api/v1/admin/users
lumo api POST /orgs/<orgId>/api/v1/admin/roles --data '{"name": "Editor"}'
lumo api DELETE /orgs/<orgId>/api/v1/admin/users/abc-123
```

The full set of routes is documented at `/api/doc` on your LumoAuth instance (auto-generated OpenAPI 3 spec).

---

## Output formats

```bash
lumo users list                # ASCII table (default in TTYs)
lumo users list -o json        # JSON
lumo users list -o yaml        # YAML
lumo users list | jq '.data[].email'    # auto-JSON when piped
```

The CLI auto-detects non-TTY environments and switches to JSON automatically — no `-o json` needed when piping into `jq` or another tool.

---

## AI agent integration

This CLI is designed to be easily used by AI coding agents (Copilot, Cursor, Claude Code, etc.) to manage LumoAuth resources programmatically.

| Feature | Details |
|---------|---------|
| Auto-JSON output | Non-TTY environments automatically get JSON |
| Structured errors | `{"error": true, "message": "..."}` |
| Typed exit codes | `0` = success, `1` = error, `2` = auth error, `3` = not found |
| `--quiet` mode | Suppresses decorative output |
| Raw API | `lumo api` allows any endpoint without a dedicated command |
| No interactivity | All operations are non-interactive (no prompts during execution) |

### Example agent prompt

```
You have access to the LumoAuth CLI (`lumo`). Use it to manage users, roles,
and permissions for the organization. Authenticate ONE of these ways:

  - LUMO_API_KEY + LUMO_ORG_ID env vars (CI / scripted), OR
  - `lumo login --org <slug>` once at the start (interactive).

To list users:     lumo users list -o json
To create a role:  lumo roles create --name "Editor" --permissions doc.edit,doc.view -o json
To check settings: lumo settings get auth -o json
To make raw calls: lumo api GET /orgs/{orgId}/api/v1/admin/users -o json

Always use -o json for parseable output.
```

### Example: scripting

```bash
#!/bin/bash
# Spawn a sandbox tenant for a PR and seed it
SLUG=$(lumo dev start --name pr-1234 -o json | jq -r '.data.slug')
lumo --org-id "$SLUG" users create --email demo@example.com --name "Demo"
lumo --org-id "$SLUG" agents create --name "PR Bot"
echo "Sandbox $SLUG ready"
```

---

## Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LUMO_API_KEY` | Admin API key (`lmk_...`) | — |
| `LUMO_ORG_ID` | Organization ID | — |
| `LUMO_BASE_URL` | LumoAuth server URL | `https://app.lumoauth.dev` (US) |
| `LUMO_OUTPUT_FORMAT` | Default output format | `table` |
| `LUMO_INSECURE` | Skip TLS verification (`true`/`1`) | `false` |
| `LUMO_CONFIG_DIR` | Custom config directory | `~/.lumoauth` |

---

## API scopes

Each API key has granular scopes that control access. Device-flow logins inherit the scopes granted to the `lumoauth-cli` first-party client (configurable per tenant in the dashboard).

| Command | Required scope |
|---------|---------------|
| `users list/get` | `admin:users:read` |
| `users create/update/delete` | `admin:users:write` |
| `roles list/get` | `admin:roles:read` |
| `roles create/update/delete` | `admin:roles:write` |
| `groups list/get` | `admin:groups:read` |
| `groups create/update/delete` | `admin:groups:write` |
| `apps list/get` | `admin:clients:read` |
| `apps create/update/delete` | `admin:clients:write` |
| `agents list/get` | `admin:agents:read` |
| `agents create/update/delete` | `admin:agents:write` |
| `webhooks list/get` | `admin:webhooks:read` |
| `webhooks create/update/delete` | `admin:webhooks:write` |
| `webhooks tunnel start/stream/stop` | `admin:webhooks:write` |
| `logs list/get/stats/export/follow` | `admin:audit:read` |
| `permissions list/get` | `admin:permissions:read` |
| `permissions create/update/delete` | `admin:permissions:write` |
| `settings get` | `admin:settings:read` |
| `settings update` | `admin:settings:write` |
| `sessions/tokens` | `admin:sessions:read`, `admin:sessions:write` |
| `social list/get` | `admin:social:read` |
| `social create/delete` | `admin:social:write` |
| `dev start/list/stop` | `admin:tenant:write` (sandbox) |

---

## Troubleshooting

**`lumo --version` works but `lumo login` says "command not found"** — restart your shell so `~/.local/bin` is on `PATH`.

**Browser doesn't open during `lumo login`** — pass `--no-browser` and copy the verification URL by hand. The flow still works.

**`stored credentials have expired`** — run `lumo login` again. The CLI does not currently auto-refresh; this lands in a future release.

**Webhooks don't show up in `lumo tunnel`** — check `lumo webhooks list` for any *other* webhooks subscribed to the same events; the dispatcher fires every subscriber. The tunnel still receives them, but make sure you're reproducing the action that triggers the event.

**`lumo dev stop` says "does not own this sandbox"** — sandboxes are owned by the user who created them. If a teammate spawned it, ask them to destroy it (or wait for the cleanup cron to expire it).

---

## Project structure

```
cli/
├── main.go                            # Entry point
├── go.mod
├── .goreleaser.yaml                   # Build matrix + ldflags
├── install.sh                         # One-line install script
├── cmd/
│   ├── root.go                        # Root command, global flags, exit codes
│   ├── version.go                     # `lumo version` / `--version`
│   ├── login.go                       # `lumo login/logout/whoami`
│   ├── dev.go                         # `lumo dev start/stop/list`
│   ├── tunnel.go                      # `lumo tunnel`
│   ├── init.go                        # `lumo init --framework ...`
│   ├── templates/init/                # Embedded starter project templates
│   │   ├── next/
│   │   ├── express/
│   │   ├── fastapi/
│   │   └── go/
│   ├── config.go                      # `lumo config init/show/set`
│   ├── users.go, roles.go, groups.go, apps.go, agents.go, …
│   └── api.go                         # Raw API passthrough
└── internal/
    ├── auth/device_flow.go            # RFC 8628 device flow client
    ├── config/config.go               # Settings (config.yaml)
    ├── config/credentials.go          # Tokens (credentials.yaml)
    ├── client/client.go               # HTTP client w/ Bearer + ApiKey fallback
    └── output/output.go               # Table/JSON/YAML output
```

## License

See the root repository license.
