# LumoAuth CLI

A comprehensive command-line interface for managing your [LumoAuth](https://lumoauth.com) tenant — users, roles, groups, OAuth apps, AI agents, webhooks, audit logs, permissions, settings, sessions, and more.

Designed for **tenant admins** and **AI coding agents** alike.

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
lumo config init
```

That's it. The installer downloads the right binary for your platform, places it in `~/.local/bin`, and adds it to your `PATH`.

## Installation

### One-line install (recommended)

Works on **Linux**, **macOS**, and **Windows (WSL)** — no `sudo` required.

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
```

The script will:
1. Detect your OS and architecture (amd64 / arm64)
2. Download the latest release from GitHub
3. Install the `lumo` binary to `~/.local/bin`
4. Add `~/.local/bin` to your shell's `PATH` (bash, zsh, fish, or sh) if it isn't already

After installation, restart your shell (or `source` your profile) and run:

```bash
lumo config init
```

This launches an interactive wizard that configures your API key, tenant, and region — everything you need to start managing your LumoAuth tenant from the command line.

### Build from source

**Prerequisites:** Go 1.22+

```bash
git clone https://github.com/LumoAuth/cli.git && cd cli
go build -o lumo .

# Optional: install into $GOPATH/bin
go install .
```

## Authentication

The CLI uses tenant Admin API keys (prefixed `lmk_`). Create one at:

```
https://<your-lumoauth>/t/<tenant>/portal/settings/api-keys
```

### Configuration Precedence

| Priority | Method | Example |
|----------|--------|---------|
| 1 (highest) | CLI flags | `--api-key lmk_xxx --tenant acme-corp` |
| 2 | Environment variables | `LUMO_API_KEY`, `LUMO_TENANT`, `LUMO_BASE_URL` |
| 3 (lowest) | Config file | `~/.lumoauth/config.yaml` |

### Config File

```yaml
# ~/.lumoauth/config.yaml
api_key: lmk_your_key_here
tenant: acme-corp
base_url: https://app.lumoauth.dev   # or https://eu.app.lumoauth.dev for EU
format: table
insecure: false
```

Manage your config via:

```bash
lumo config init          # Interactive setup wizard
lumo config show          # Display current configuration
lumo config set tenant acme-corp
lumo config set api_key lmk_abc123
```

## Commands

### Global Flags

```
--api-key string    API key (overrides LUMO_API_KEY)
--tenant string     Tenant slug (overrides LUMO_TENANT)
--base-url string   Base URL (overrides LUMO_BASE_URL)
-o, --output string Output format: table, json, yaml (default: table)
--insecure          Skip TLS verification (useful for local dev)
-q, --quiet         Suppress non-essential output
-v, --verbose       Enable verbose output
```

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

# Sub-resources
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

# Members
lumo groups members list <group-id>
lumo groups members add <group-id> --users user1,user2
lumo groups members remove <group-id> --users user1,user2
```

### OAuth Applications

```bash
lumo apps list [--search "my-app"]
lumo apps get <client-id>
lumo apps create --name "My App" [--type web|spa|native|m2m] [--redirect-uris http://localhost:3000/callback]
lumo apps update <id> [--name "Updated App"]
lumo apps delete <id>
lumo apps rotate-secret <id>
```

### AI Agents

```bash
lumo agents list [--search "assistant"]
lumo agents get <agent-id>
lumo agents create --name "My Agent" [--type ai_assistant] [--capabilities read_data,write_data]
lumo agents update <id> [--name "Updated Agent"]
lumo agents delete <id>
lumo agents enable <id>
lumo agents disable <id>
lumo agents rotate-credentials <id>
lumo agents token <id> [--ttl 3600]    # Generate a bearer token
```

### Permissions

```bash
lumo permissions list
lumo permissions get <id-or-slug>
lumo permissions create --slug "document.edit" [--name "Edit Documents"] [--description "..."]
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

### Audit Logs

```bash
lumo logs list [--action login] [--user <user-id>] [--from 2025-01-01] [--to 2025-12-31] [--status success]
lumo logs get <log-id>
lumo logs stats [--from 2025-01-01] [--to 2025-12-31]
lumo logs export [--export-format csv|json] [--from ...] [--to ...]
```

### Settings

```bash
# View settings
lumo settings get tenant
lumo settings get auth
lumo settings get branding
lumo settings get ai

# Update settings
lumo settings update tenant --name "Acme Corp" --display-name "Acme Corporation"
lumo settings update auth --mfa-required --session-lifetime 86400
lumo settings update branding --primary-color "#5865F2" --logo-url "https://example.com/logo.png"
lumo settings update ai --data '{"agentRegistrationEnabled": true}'
```

### Sessions & Tokens

```bash
lumo sessions list [--user <user-id>]
lumo sessions revoke <session-id>
lumo sessions revoke-all --user <user-id>

lumo tokens list [--user <user-id>] [--client <client-id>]
lumo tokens revoke <token-id>
```

### Social Login Providers

```bash
lumo social list
lumo social get <id>
lumo social create --provider google --client-id <id> --client-secret <secret> [--scopes email,profile]
lumo social delete <id>
```

### Raw API Access

For endpoints not covered by named commands, or for use by AI agents:

```bash
lumo api GET /t/acme-corp/api/v1/admin/users
lumo api POST /t/acme-corp/api/v1/admin/roles --data '{"name": "Editor"}'
lumo api DELETE /t/acme-corp/api/v1/admin/users/abc-123
```

## Output Formats

```bash
# ASCII table (default for terminals)
lumo users list

# JSON (default when piping, ideal for jq and AI agents)
lumo users list -o json
lumo users list -o json | jq '.data[].email'

# YAML
lumo users list -o yaml
```

The CLI auto-detects non-TTY environments (pipes, scripts) and switches to JSON automatically — no `-o json` needed when piping.

## AI Agent Integration

This CLI is designed to be easily used by AI coding agents (Copilot, Cursor, Claude, etc.) to manage LumoAuth resources programmatically.

### Features for AI Agents

| Feature | Details |
|---------|---------|
| **Auto-JSON output** | Non-TTY environments automatically get JSON |
| **Structured errors** | `{"error": true, "message": "..."}` |
| **Typed exit codes** | `0` = success, `1` = error, `2` = auth error, `3` = not found |
| **`--quiet` mode** | Suppresses decorative output (spinners, confirmations) |
| **Raw API** | `lumo api` allows any endpoint call without a dedicated command |
| **No interactivity** | All operations are non-interactive (no prompts during execution) |

### Example: AI Agent Prompt

```
You have access to the LumoAuth CLI (`lumo`). Use it to manage users, roles, 
and permissions for the tenant. Environment variables LUMO_API_KEY and 
LUMO_TENANT are pre-configured.

To list users:     lumo users list -o json
To create a role:  lumo roles create --name "Editor" --permissions doc.edit,doc.view -o json
To check settings: lumo settings get auth -o json
To make raw calls: lumo api GET /t/{tenant}/api/v1/admin/users -o json

Always use -o json for parseable output.
```

### Example: Scripting

```bash
#!/bin/bash
# Create a role and assign it to a user
ROLE=$(lumo roles create --name "Reviewer" -o json | jq -r '.data.id')
lumo users roles set user-abc-123 --roles "$ROLE"
echo "Role $ROLE assigned"
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LUMO_API_KEY` | Admin API key (`lmk_...`) | — |
| `LUMO_TENANT` | Tenant slug | — |
| `LUMO_BASE_URL` | LumoAuth server URL | `https://app.lumoauth.dev` (US) |
| `LUMO_OUTPUT_FORMAT` | Default output format | `table` |
| `LUMO_INSECURE` | Skip TLS verification (`true`/`1`) | `false` |
| `LUMO_CONFIG_DIR` | Custom config directory | `~/.lumoauth` |

## API Scopes

Each API key has granular scopes that control access. The CLI requires appropriate scopes for each operation:

| Command | Required Scope |
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
| `logs list/get/stats/export` | `admin:audit:read` |
| `permissions list/get` | `admin:permissions:read` |
| `permissions create/update/delete` | `admin:permissions:write` |
| `settings get` | `admin:settings:read` |
| `settings update` | `admin:settings:write` |
| `sessions/tokens` | `admin:sessions:read`, `admin:sessions:write` |
| `social list/get` | `admin:social:read` |
| `social create/delete` | `admin:social:write` |

## Project Structure

```
cli/
├── main.go                        # Entry point
├── go.mod
├── cmd/
│   ├── root.go                    # Root command, global flags, exit codes
│   ├── config.go                  # lumo config init/show/set
│   ├── users.go                   # User management (13 subcommands)  
│   ├── roles.go                   # Role management
│   ├── groups.go                  # Group management with members
│   ├── apps.go                    # OAuth application management
│   ├── agents.go                  # AI agent management
│   ├── webhooks.go                # Webhook management
│   ├── logs.go                    # Audit log access
│   ├── permissions.go             # Permission management
│   ├── settings.go                # Tenant/auth/branding/AI settings
│   ├── sessions.go                # Session & token management
│   ├── social.go                  # Social provider management
│   └── api.go                     # Raw API passthrough
└── internal/
    ├── config/config.go           # Config loading & persistence
    ├── client/client.go           # HTTP client with auth
    └── output/output.go           # Table/JSON/YAML output
```

## License

See the root repository license.
