<div align="center">

# lumo

**The LumoAuth command line.**
Users, roles, groups, OAuth apps, AI agents, webhooks, audit logs, MFA policy, sandboxes — all from your terminal, and all scriptable.

[![Release](https://img.shields.io/github/v/release/LumoAuth/cli?label=release&color=111)](https://github.com/LumoAuth/cli/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/LumoAuth/cli?color=111)](go.mod)
[![License](https://img.shields.io/github/license/LumoAuth/cli?color=111)](LICENSE)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%C2%B7%20Linux%20%C2%B7%20Windows-111)](#installation)
[![Built with GoReleaser](https://img.shields.io/badge/built%20with-GoReleaser-111)](.goreleaser.yaml)

[Install](#installation) · [Quick start](#quick-start) · [Authentication](#authentication) · [Commands](#commands) · [AI agents](#ai-agent-integration) · [Troubleshooting](#troubleshooting)

</div>

---

```bash
brew install lumoauth/tap/lumo      # macOS / Linux
scoop install lumo                  # Windows (after: scoop bucket add lumoauth https://github.com/lumoauth/scoop-bucket)
winget install --id LumoAuth.lumo   # Windows
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh   # anywhere else
```

```console
$ lumo login --org acme-corp
→ Starting login on https://app.lumoauth.dev for org acme-corp

  ┌─ Open this URL in any browser to approve the login ─
  │  https://app.lumoauth.dev/orgs/acme-corp/device
  │  User code: WXYZ-1234
  └─

  (opened in your browser; or copy the URL above)
Waiting for approval (Ctrl+C to cancel)...
✓ Logged in to acme-corp. Credentials saved to profile "default" in ~/.lumoauth/credentials.yaml
  Signed in as:   jane@acme.com
  Next: lumo doctor

$ lumo doctor
✓ profile       default (~/.lumoauth/credentials.yaml)
✓ organization  acme-corp
✓ server        https://app.lumoauth.dev
✓ credential    jane@acme.com, login token, expires 2026-10-07T04:12:09Z
✓ scopes        openid profile email admin
✓ discovery     issuer https://app.lumoauth.dev/orgs/acme-corp
✓ admin api     GET /admin/organization succeeded

lumo 1.1.0
```

Built for three audiences: **org admins** who want to get things done without the dashboard, **developers** wiring LumoAuth into an app, and **AI coding agents** that need predictable JSON, typed exit codes and errors that say how to fix themselves.

## Highlights

<table>
<tr>
<td width="33%" valign="top">

**Sign in like a human**
`lumo login` runs the OAuth device flow in your browser, stores a scoped token with mode 0600, and refreshes it silently. Narrow scopes with `--scope` for least-privilege sessions.

</td>
<td width="33%" valign="top">

**Know it works before you start**
`lumo doctor` checks profile, org, server, credential, scopes and a real Admin API call, and prints the fix for anything that fails. Exit 1 on failure, so it belongs in CI.

</td>
<td width="33%" valign="top">

**Sandboxes on demand**
`lumo dev start` spawns an isolated throwaway tenant with a TTL. Perfect for PR previews. `lumo dev stop` or the cleanup cron takes it away.

</td>
</tr>
<tr>
<td valign="top">

**Webhooks to localhost**
`lumo tunnel --to http://localhost:3000/hooks` replays live events to your dev server, like `stripe listen`. Filter by event type.

</td>
<td valign="top">

**Scaffold a working app**
`lumo init --framework next|express|fastapi|go` writes a minimal project already wired to your org, env files filled in.

</td>
<td valign="top">

**Made for agents and scripts**
Auto-JSON when piped, structured errors with a `hint`, typed exit codes, no prompts, and `lumo api` for any endpoint without a command.

</td>
</tr>
</table>

## Table of contents

- [Quick start](#quick-start)
- [Installation](#installation) — [Homebrew](#homebrew-macos--linux) · [Scoop](#scoop-windows) · [winget](#winget-windows) · [install script](#install-script-linux--macos--wsl) · [direct download](#direct-download) · [`go install`](#go-install) · [source](#build-from-source) · [completions](#shell-completions) · [upgrading](#upgrading)
- [Authentication](#authentication) — [device flow](#a-device-flow-login-recommended-for-humans) · [API keys](#b-api-key-recommended-for-ci--scripting) · [profiles](#named-profiles-multi-org) · [precedence](#configuration-precedence)
- [Commands](#commands)
- [Output formats](#output-formats)
- [AI agent integration](#ai-agent-integration)
- [Environment variables](#environment-variables)
- [API scopes](#api-scopes)
- [Troubleshooting](#troubleshooting)
- [Releasing](#releasing) · [Project structure](#project-structure) · [License](#license)

---

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
lumo login --org acme-corp     # approve in the browser
lumo doctor                    # ✓ credential, scopes, server, admin api
lumo users list                # you're in
```

The installer drops the binary in `~/.local/bin`. `lumo login` opens your browser for the device-flow approval and stores a token with the `admin` scope in `~/.lumoauth/credentials.yaml` (mode 0600). `lumo doctor` proves it works before you run anything else.

For CI and scripts, skip the browser and use an [API key](#b-api-key-recommended-for-ci--scripting):

```bash
LUMO_API_KEY=lmk_… LUMO_ORG=acme-corp lumo users list -o json
```

---

## Installation

Every channel installs the same statically linked binary, and `lumo upgrade` later upgrades through whichever one you used.

| | Recommended | Also works |
|---|---|---|
| **macOS** | [Homebrew](#homebrew-macos--linux) | install script · direct download · `go install` |
| **Linux** | [Homebrew](#homebrew-macos--linux) or the [install script](#install-script-linux--macos--wsl) | direct download · `go install` |
| **Windows** | [Scoop](#scoop-windows) or [winget](#winget-windows) | direct download · `go install` · WSL + install script |

Builds ship for macOS (Apple silicon, Intel), Linux (x86_64, arm64, i386) and Windows (x86_64, i386). There is no Windows arm64 build; use `go install` there.

### Homebrew (macOS / Linux)

```bash
brew install lumoauth/tap/lumo
```

The tap is [`LumoAuth/homebrew-tap`](https://github.com/LumoAuth/homebrew-tap) and `lumo` is published as a cask. On macOS the cask strips the Gatekeeper quarantine attribute for you, so the unsigned binary runs without a warning. Shell completions for bash, zsh and fish are installed alongside.

```bash
brew upgrade lumoauth/tap/lumo   # or just: lumo upgrade
```

### Scoop (Windows)

```powershell
scoop bucket add lumoauth https://github.com/lumoauth/scoop-bucket
scoop install lumo
```

The bucket is [`LumoAuth/scoop-bucket`](https://github.com/LumoAuth/scoop-bucket). Upgrade with `scoop update lumo` or `lumo upgrade`.

### winget (Windows)

```powershell
winget install --id LumoAuth.lumo
```

Each release is submitted to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs/pulls?q=LumoAuth.lumo) automatically and becomes installable once Microsoft merges it, usually within a few days. If `winget` cannot find the package yet, use Scoop or the direct download in the meantime. Upgrade with `winget upgrade --id LumoAuth.lumo` or `lumo upgrade`.

### Install script (Linux / macOS / WSL)

No `sudo` required.

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
```

The script detects your OS and architecture, downloads the latest release, verifies its SHA-256 against the release's `checksums.txt` (and refuses to install on a mismatch; `LUMO_SKIP_CHECKSUM=1` bypasses this, don't), installs `lumo` to `~/.local/bin`, and adds that directory to your shell's `PATH` for bash, zsh, fish or sh if needed. Restart your shell and run `lumo --version` to confirm.

On macOS a binary installed this way is quarantined by Gatekeeper because it is not notarized. Clear the attribute once, or prefer Homebrew:

```bash
xattr -d com.apple.quarantine ~/.local/bin/lumo
```

<details>
<summary><strong>Direct download</strong> — grab an archive from the releases page and verify it yourself</summary>

<a name="direct-download"></a>

Every release on the [releases page](https://github.com/LumoAuth/cli/releases/latest) ships an archive per platform plus `checksums.txt`. Archives are named `lumo_<OS>_<Arch>` with `OS` ∈ `Darwin`, `Linux`, `Windows` and `Arch` ∈ `x86_64`, `arm64`, `i386`; Windows archives are `.zip`, the rest `.tar.gz`.

```bash
# Example: Linux x86_64
VERSION=$(curl -fsSL https://api.github.com/repos/LumoAuth/cli/releases/latest | grep tag_name | cut -d '"' -f4)
curl -fsSLO "https://github.com/LumoAuth/cli/releases/download/${VERSION}/lumo_Linux_x86_64.tar.gz"
curl -fsSLO "https://github.com/LumoAuth/cli/releases/download/${VERSION}/checksums.txt"
sha256sum --check --ignore-missing checksums.txt      # shasum -a 256 on macOS
tar -xzf lumo_Linux_x86_64.tar.gz lumo
install -m 0755 lumo ~/.local/bin/lumo
```

```powershell
# Example: Windows x86_64
Invoke-WebRequest https://github.com/LumoAuth/cli/releases/latest/download/lumo_Windows_x86_64.zip -OutFile lumo.zip
Expand-Archive lumo.zip -DestinationPath "$env:LOCALAPPDATA\Programs\lumo"
# then add that directory to your PATH
```

A hand-placed binary is still upgradable with `lumo upgrade`, which downloads and verifies the next release the same way.

</details>

<details>
<summary><strong><code>go install</code></strong> — build from the module for any platform Go supports</summary>

<a name="go-install"></a>

**Prerequisites:** Go 1.22+. Builds from source for whatever platform you are on, including ones without a published archive (for example Windows arm64).

```bash
go install github.com/lumoauth/cli@latest          # latest release
go install github.com/lumoauth/cli@v1.1.0          # a specific version
```

The binary lands in `$(go env GOPATH)/bin` as `cli`, because the module root is the main package. Rename or alias it:

```bash
mv "$(go env GOPATH)/bin/cli" "$(go env GOPATH)/bin/lumo"
```

`lumo --version` reports `dev` for `go install` builds: the version, commit and date are injected only by the release pipeline.

</details>

<details>
<summary><strong>Build from source</strong></summary>

<a name="build-from-source"></a>

```bash
git clone https://github.com/LumoAuth/cli.git && cd cli
go build -o lumo .

# Optional: install into $GOPATH/bin
go install .
```

</details>

### Shell completions

Homebrew installs completions automatically. For every other channel, generate them from the binary:

```bash
lumo completion bash | sudo tee /etc/bash_completion.d/lumo > /dev/null   # bash
lumo completion zsh > "${fpath[1]}/_lumo"                                 # zsh
lumo completion fish > ~/.config/fish/completions/lumo.fish               # fish
lumo completion powershell | Out-String | Invoke-Expression               # PowerShell
```

`lumo completion --help` has the full per-shell instructions.

### Upgrading

```bash
lumo upgrade            # detect install channel and upgrade in place
lumo upgrade --check    # only report whether a newer version exists
lumo upgrade --exec     # upgrade without a confirmation prompt
```

`lumo upgrade` detects how the binary was installed and uses the matching
channel: Homebrew/Scoop/winget installs are upgraded through the package
manager (so its state stays consistent), while manual installs
(install.sh / hand-placed binaries) are self-updated — the release
archive for your OS/arch is downloaded, verified against the release's
`checksums.txt`, and swapped in atomically. On Windows, if the running
`lumo.exe` can't be replaced while in use, the new version is staged as
`lumo.exe.new` with printed instructions to finish the swap.

---

## Authentication

The CLI accepts auth in two ways. Pick whichever fits your context — both work everywhere.

### A. Device-flow login (recommended for humans)

```bash
lumo login --org acme-corp
```

Behind the scenes:

1. CLI calls `POST /orgs/{org}/api/v1/oauth/device_authorization` with the well-known first-party client `lumoauth-cli` and an explicit `scope` (default `openid profile email admin`).
2. CLI prints a `user_code` and opens your browser to the verification URL.
3. You approve in the dashboard.
4. CLI polls the token endpoint and stores the tokens and granted scopes at `~/.lumoauth/credentials.yaml` (0600).

Subsequent commands use the stored bearer token automatically. Access tokens last an hour and are refreshed silently from the 30-day refresh token — also once more, transparently, when the server answers `401`.

```bash
lumo whoami    # profile, org, credential in use, scopes, expiry (-o json for scripts)
lumo doctor    # verifies the credential against the server
lumo logout    # clears the active profile's tokens
```

**Scopes.** `admin` is the blanket Admin API scope and is what every `lumo` resource command needs: the Admin API accepts an OAuth token only when it carries `admin` or `admin:<resource>:<read|write>` *and* the signed-in user holds `settings.manage`. For a least-privilege session request resource scopes instead:

```bash
lumo login --org acme-corp --scope openid,admin:users:read,admin:audit:read
```

A command that needs more answers `403` naming the missing scope and the exact `lumo login --scope …` that fixes it.

The first time you log in to an organization, the server lazy-creates a `lumoauth-cli:<slug>` public OAuth client (device_code + refresh_token grants, no PKCE — the device flow already binds approval to the verified user code) allowing the default scopes. Administrators can narrow that list to force least-privilege sessions; requesting a scope it does not allow fails with `invalid_scope` and a hint.

### B. API key (recommended for CI / scripting)

```bash
export LUMO_API_KEY=lmk_your_key_here
export LUMO_ORG=acme-corp
lumo users list
```

API keys live at `https://<your-lumoauth>/orgs/<orgId>/portal/settings/api-keys`. They are checked **per resource**: a key with `admin:users:read` can list users and nothing else, `:read` never authorises a write, and the key stops working if its creator is deactivated or loses `settings.manage`. Create one key per job with only the scopes in the [API scopes](#api-scopes) table for the commands it runs.

### Named profiles (multi-org)

Work across multiple organizations or deployments (consultant with two
clients, US + EU regions, prod + self-hosted) without re-authenticating:

```bash
lumo login --profile acme --org acme-corp        # first client
lumo login --profile clientb --org clientb-corp  # second client

lumo profile list                 # see all profiles (* marks the active one)
lumo profile use acme             # switch the default
lumo users list --profile clientb # one-off command against another profile
LUMO_PROFILE=clientb lumo users list   # env-var selection (CI, direnv)

lumo profile show [name]          # inspect a profile (tokens masked)
lumo profile create staging --org acme-staging      # token-less profile for API-key use
lumo profile delete clientb       # remove a profile and its tokens
```

Each profile stores its own `org_id`, `base_url`, and login tokens in
`~/.lumoauth/credentials.yaml`. Profile selection precedence: `--profile`
flag > `LUMO_PROFILE` env var > `current_profile` in the credentials file.
The `--org` flag still overrides the profile's org for a single command.

Existing single-profile credential files are migrated automatically on
first use: your old credentials become the `default` profile and the
original file is preserved at `~/.lumoauth/credentials.yaml.bak`.

`lumo logout` clears only the active profile; other profiles keep their
sessions.

### Configuration precedence

Settings (organization, base URL, output format) resolve flags > environment > config file > active profile:

| Priority | Method | Example |
|----------|--------|---------|
| 1 (highest) | CLI flags | `--org acme-corp --base-url https://eu.app.lumoauth.dev` |
| 2 | Environment variables | `LUMO_ORG`, `LUMO_BASE_URL`, `LUMO_API_KEY` |
| 3 | Config file | `~/.lumoauth/config.yaml` |
| 4 (fallback) | Active profile | `~/.lumoauth/credentials.yaml` (from `lumo login`) |

The **credential** is chosen separately: the active profile's login token wins whenever it targets the organization the command addresses (so a stale key for another org is never sent by accident); otherwise the API key is used; otherwise the command exits with code 2 and a hint. `lumo whoami` shows the outcome.

### Config file format

```yaml
# ~/.lumoauth/config.yaml — manual settings (no secrets)
org_id: acme-corp
base_url: https://app.lumoauth.dev   # or https://eu.app.lumoauth.dev
format: table
insecure: false
```

```yaml
# ~/.lumoauth/credentials.yaml — managed by `lumo login`/`logout`/`lumo profile` (0600)
version: 2
current_profile: acme
profiles:
  acme:
    base_url: https://app.lumoauth.dev
    org_id: acme-corp
    access_token: eyJhbGciOi...
    refresh_token: ...
    expires_at: 2026-05-08T17:00:00Z
    token_type: Bearer
    scopes: [openid, profile, email, admin]
  clientb:
    base_url: https://eu.app.lumoauth.dev
    org_id: clientb-corp
    access_token: ...
```

Manage settings via:

```bash
lumo config init          # Interactive setup wizard
lumo config show          # Display current configuration
lumo config set org_id acme-corp
```

---

## Commands

Everything is `lumo <resource> <verb>`. Run `lumo --help` or `lumo <resource> --help` for the authoritative list.

| Area | Commands |
|---|---|
| Session | [`login`](#login--logout--whoami) · `logout` · `whoami` · [`doctor`](#doctor) · [`profile`](#profile--named-credential-profiles) · [`upgrade`](#upgrade) · `config` |
| Developer loop | [`dev`](#dev--ephemeral-sandbox-tenants) · [`tunnel`](#tunnel--forward-webhooks-to-localhost) · [`init`](#init--scaffold-a-starter-project) |
| Organization | [`org`](#org--organization-profile-and-settings) · [`settings`](#settings) · [`mfa`](#mfa-policy-and-coverage) · [`social`](#social-login-providers) |
| People & access | [`users`](#users) · [`identities`](#federated-identities-saml--ldap-relink) · [`roles`](#roles) · [`groups`](#groups) · [`permissions`](#permissions) |
| Clients | [`apps`](#oauth-applications) · [`agents`](#ai-agents) · [`sessions` / `tokens`](#sessions--tokens) |
| Events | [`webhooks`](#webhooks) · [`logs`](#audit-logs) |
| Escape hatch | [`api`](#raw-api-access) |

### Global flags

```
--api-key string    Organization API key lmk_… (overrides LUMO_API_KEY)
--org string        Organization slug (overrides LUMO_ORG and the active profile; --org-id still accepted)
--base-url string   Base URL (overrides LUMO_BASE_URL)
--profile string    Named credentials profile (overrides LUMO_PROFILE and current_profile)
-o, --output string Output format: table, json, yaml (default: table; json when piped)
--insecure          Skip TLS verification (local dev only)
-q, --quiet         Suppress non-essential output
-v, --verbose       Enable verbose output
```

### `login` / `logout` / `whoami`

```bash
lumo login [--org acme-corp] [--scope openid,admin:users:read] [--profile name] [--no-browser] [--insecure]
lumo logout
lumo whoami [-o json]
```

`--scope` narrows the requested scopes (default `openid,profile,email,admin`). `--no-browser` prints the verification URL instead of opening a browser — useful in headless or remote environments. `--profile` logs in to (or creates) a named profile; `logout` clears only the active profile. `whoami` shows which credential the next command will send and, for a login token, its scopes.

### `doctor`

```bash
lumo doctor            # ✓/✗ per check, with the fix for each failure
lumo doctor -o json    # {"ok": bool, "checks": [...]} — exit 1 when a check fails
```

Checks the active profile, organization, server reachability (OIDC discovery), the credential and its scopes, and a real Admin API call. Run it after `lumo login`, after creating an API key, and in CI before the real work.

### `org` — organization profile and settings

```bash
lumo org get
lumo org update --name "ACME Corporation"
lumo org update --set timezone=Europe/Berlin --set allow_root_admin_login=false
lumo org update --set security.dpop_require_nonce=true
```

Only the documented, non-security settings keys are writable; anything else is refused with the offending keys listed and nothing is saved. Dotted keys nest, and object-valued keys merge key by key.

### `profile` — Named credential profiles

```bash
lumo profile list                # all profiles; * marks the active one
lumo profile use <name>          # set the current profile
lumo profile show [name]         # profile details (tokens masked)
lumo profile create <name> [--org slug] [--base-url url] [--use]
lumo profile delete <name> [-f]
```

See [Named profiles](#named-profiles-multi-org) for the full workflow.

### `upgrade`

```bash
lumo upgrade [--check] [--exec]
```

Channel-aware upgrade — see [Upgrading](#upgrading).

### `dev` — Ephemeral sandbox tenants

Spawn throwaway tenants for branch previews, PR environments, or scratch experiments. Each sandbox is a separate, fully-isolated tenant with its own slug; the daily cleanup cron deletes any sandbox past its TTL (default 24h, max 7 days).

```bash
lumo dev start [--name pr-1234] [--ttl-hours 24]
lumo dev list
lumo dev stop sandbox-pr-1234-7a3b21
```

`lumo dev start` prints the sandbox slug. Use it like any other org by passing `--org` or by exporting `LUMO_ORG`. The sandbox can be destroyed early with `lumo dev stop`; only the sandbox's owner (the user who created it) can destroy it.

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
lumo init --framework next [--dir ./my-app] [--org acme-corp] [--client-id lumo_...] [--sdk-path ~/src/lumoauth]
lumo init --framework express
lumo init --framework fastapi
lumo init --framework go
```

Generates a minimal-but-working project wired to your org. `.env` files are pre-filled with the org slug and base URL from your current credentials, plus the OAuth client ID when `--client-id` is given.

Until the SDK packages are published, pass `--sdk-path` (or set `LUMO_SDK_PATH`) pointing at a LumoAuth source checkout (the directory containing `sdk-js`, `sdk-python`, `sdk-go`); the starter then installs the SDK from those local sources.

| Framework | What you get |
|---|---|
| `next` | Next.js 14 (App Router) + `@lumoauth/nextjs`, with `<SignIn />` and `<UserButton />` wired up. |
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

lumo users authenticators list <user-id-or-email>          # ID, type, name, state, tier, default, last used + status
lumo users authenticators remove <user-id> <authenticator-id> [--yes]   # e.g. totp:12 — a lost phone
lumo users tap <user-id-or-email> --reason "Lost phone; verified by video call" [--ttl 60] [--multi-use] [--mfa-challenge <id>]

lumo users roles get <user-id>
lumo users roles set <user-id> --roles admin,editor
lumo users groups get <user-id>
lumo users groups set <user-id> --groups team-a,team-b
```

Admins can no longer switch MFA off for a user: `lumo users mfa-reset` is deprecated and fails without calling the server. For a locked-out user, issue a **temporary access code** with `lumo users tap` — it stands in for the second factor once (or until it expires with `--multi-use`), only lets the user sign in and enroll a new factor, and is printed exactly once. The reason is required, audited and shown to the user. When you are signed in with `lumo login`, the server also wants a fresh MFA check of your own: pass an approved `step_up` challenge id with `--mfa-challenge` (API keys are exempt). `lumo users authenticators remove` revokes a single factor (asks for confirmation on a terminal; scripts must pass `--yes`); if MFA is required, the user enrolls again at next sign-in.

### Federated identities (SAML / LDAP relink)

```bash
lumo users identities list <user-id-or-email>                     # SAML IdP + NameID, LDAP directory + DN, social links
lumo users identities link <user> --saml-idp <idp-id> --name-id <nameid> [--mfa-challenge <id>]
lumo users identities link <user> --ldap-config <id> [--dn "uid=jane,ou=people,dc=acme,dc=com"] [--ldap-only] [--mfa-challenge <id>]
lumo users identities unlink <user> --type saml|ldap|social [--yes] [--mfa-challenge <id>]

lumo identities legacy-saml [--idp <idp-id>]                      # users with a legacy bare-NameID SAML link
lumo identities legacy-saml --idp <idp-id> --relink --dry-run     # preview
lumo identities legacy-saml --idp <idp-id> --relink [--user <id> ...] --yes
```

A link decides which external identity signs in as the user, so linking or unlinking signs the user out everywhere, emails them, and is audited (`identity.link.created` / `identity.link.removed` / `identity.link.bulk_relinked`). You cannot change the links of an account that holds permissions you lack. Without `--dn` the server looks the directory entry up by the user's email, then username; `--ldap-only` also disables the local password. A 409 (`identity_conflict`) means another user already holds that identity or the user is linked to a different federated source (unlink it first). Users whose SAML link predates per-IdP binding are refused at SAML sign-in while the organization has more than one SAML IdP — `lumo identities legacy-saml` lists them with the IdP whose allowed email domains claim them; `--relink` rebinds them (all users those domains claim, or the given `--user` ids). With `lumo login` tokens pass an approved `step_up` challenge with `--mfa-challenge` (API keys are exempt).

### MFA policy and coverage

```bash
lumo mfa policy get
lumo mfa policy set --requirement required --grace-days 14
lumo mfa policy set --allowed-factors passkey,push,totp --min-tier-login 4 --min-tier-step-up 3
lumo mfa policy set --trusted-device-days 30 --required-factors 1 \
  --email-otp-counts=false --passwordless-passkey --block-voip
lumo mfa policy set --data '{"sms_country_denylist":["XX"]}'   # fields without a flag
lumo mfa coverage                # enrolled %, by factor type and tier, grace-period counts
lumo mfa coverage -o json
```

`--requirement` is one of `off`, `optional`, `required`, `risk_based`; `--allowed-factors` takes `passkey,push,totp,sms_otp,email_otp`. `policy set` sends only the flags you pass (a partial update); tiers run from 1 (strongest, passkey) to 4 (basic, SMS/email).

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
lumo settings get general
lumo settings get auth
lumo settings get branding
lumo settings get all

lumo settings update org --set name="Acme Corp"
lumo settings update auth --set session_timeout=86400 --set password_min_length=12
lumo settings update branding --set primary_color="#5865F2" --set logo_url="https://example.com/logo.png"
lumo settings update scim --data '{"allow_user_deletion": false}'
```

Areas: `general`, `authentication` (`auth`), `security`, `email`, `branding`, `scim`, `organization` (`org`, `tenant`). `--set key=value` is repeatable and dotted keys nest. The MFA requirement and factors are not an `auth` setting any more — use `lumo mfa policy set`.

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

For endpoints not covered by named commands, or for use by AI agents. Paths are relative to the Admin API of the current organization; absolute API paths and URLs work too:

```bash
lumo api GET /users --query search=jane --query limit=5     # → /orgs/<org>/api/v1/admin/users?search=jane&limit=5
lumo api POST /roles --data '{"name": "Editor"}'
lumo api PATCH /organization --data '{"settings":{"allow_root_admin_login":false}}'
lumo api GET /orgs/<org>/api/v1/.well-known/openid-configuration   # absolute API path
lumo api GET /api/v1/authz/check                                   # global API
lumo api --raw GET /healthz                                        # relative to the base URL
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
| Structured errors | `{"error": true, "message", "hint", "status", "code", "details"}` — `hint` says how to fix it (e.g. the exact `lumo login --scope …`) |
| Typed exit codes | `0` success · `1` error · `2` auth/authorization · `3` not found · `4` invalid input · `5` rate limited |
| `--quiet` mode | Suppresses decorative output |
| Raw API | `lumo api` allows any endpoint without a dedicated command |
| No interactivity | All operations are non-interactive (no prompts during execution) |

### Example agent prompt

```
You have access to the LumoAuth CLI (`lumo`). Use it to manage users, roles,
and permissions for the organization. Authenticate ONE of these ways:

  - LUMO_API_KEY + LUMO_ORG env vars (CI / scripted), OR
  - `lumo login --org <slug>` once at the start (interactive).

Run `lumo doctor -o json` first; if "ok" is false, fix the failing check's hint.

To list users:     lumo users list -o json
To create a role:  lumo roles create --name "Editor" --permissions doc.edit,doc.view -o json
To check settings: lumo settings get authentication -o json
To check MFA:      lumo mfa policy get -o json   (and: lumo mfa coverage -o json)
To make raw calls: lumo api GET /users -o json      (paths are relative to the Admin API)

Always use -o json for parseable output. On a 403, read "hint" in the error JSON:
it names the missing scope and how to get it.
```

### Example: scripting

```bash
#!/bin/bash
# Spawn a sandbox tenant for a PR and seed it
SLUG=$(lumo dev start --name pr-1234 -o json | jq -r '.data.slug')
lumo --org "$SLUG" users create --email demo@example.com --name "Demo"
lumo --org "$SLUG" agents create --name "PR Bot"
echo "Sandbox $SLUG ready"
```

---

## Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LUMO_API_KEY` | Organization API key (`lmk_...`) | — |
| `LUMO_ORG` | Organization slug (`LUMO_ORG_ID` still accepted) | active profile's org |
| `LUMO_BASE_URL` | LumoAuth server URL | `https://app.lumoauth.dev` (US) |
| `LUMO_PROFILE` | Named credentials profile | `current_profile` from credentials.yaml |
| `LUMO_OUTPUT_FORMAT` | Default output format | `table` |
| `LUMO_INSECURE` | Skip TLS verification (`true`/`1`) | `false` |
| `LUMO_CONFIG_DIR` | Custom config directory | `~/.lumoauth` |

---

## API scopes

Scopes are enforced per resource and fail closed. An API key carries the scopes ticked when it was created; a login token carries what `lumo login --scope` requested (default: the blanket `admin`, which satisfies every row below within the user's own permissions). Read scopes never authorise writes, and a scope for one resource never unlocks another. `lumo settings get` on any area needs `admin:settings:read`; `lumo org update` needs `admin:settings:write`.

| Command | Required scope |
|---------|---------------|
| `users list/get` | `admin:users:read` |
| `users create/update/delete` | `admin:users:write` |
| `users authenticators list`, `mfa coverage` | `admin:users:read` |
| `users authenticators remove`, `users tap` | `admin:users:write` |
| `mfa policy get` / `mfa policy set` | `admin:settings:read` / `admin:settings:write` |
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
| `settings get`, `org get` | `admin:settings:read` |
| `settings update`, `org update` | `admin:settings:write` |
| `sessions/tokens` | `admin:sessions:read`, `admin:sessions:write` |
| `social list/get` | `admin:social:read` |
| `social create/delete` | `admin:social:write` |
| `dev start/list/stop` | `admin:settings:read` (list), `admin:settings:write` (start/stop) |
| `mcp servers/discovery/test` | `admin:agents:read`, `admin:agents:write` (test mints a token) |
| `doctor` | any scope that can read the organization (`admin`, `admin:settings:read`) |

---

## Troubleshooting

**`lumo --version` works but `lumo login` says "command not found"** — restart your shell so `~/.local/bin` is on `PATH`.

**Browser doesn't open during `lumo login`** — pass `--no-browser` and copy the verification URL by hand. The flow still works.

**`tls: failed to verify certificate` against a dev server** — run `lumo login --insecure` once. The choice is stored in the profile and applies only to that profile's server, so later commands need no flag and production URLs keep full TLS verification. Localhost, `.local`, `http://` and private-network addresses (`10.x`, `172.16–31.x`, `192.168.x`) are detected automatically when you pick "Other / Self-hosted".

**`stored credentials have expired`** — the CLI auto-refreshes with the stored refresh token; this error means the refresh token itself expired or was revoked. Run `lumo login` again (add `--profile <name>` if the expired credentials belong to a non-default profile).

**`403 forbidden` right after `lumo login`** — the token has no admin scope: either you narrowed `--scope`, or an administrator narrowed the `lumoauth-cli:<slug>` client. The error's hint names the missing scope; `lumo login` again with it, or use an API key. `lumo doctor` shows the scopes in effect.

**`invalid_scope` on `lumo login`** — the organization's CLI client does not allow a requested scope. Narrow `--scope`, or ask an administrator to allow it under Applications → LumoAuth CLI.

**`404` from `lumo api`** — paths are relative to `/orgs/<org>/api/v1/admin` unless they start with `/orgs`, `/api` or `/.well-known`; pass `--raw` for anything else relative to the base URL.

**Webhooks don't show up in `lumo tunnel`** — check `lumo webhooks list` for any *other* webhooks subscribed to the same events; the dispatcher fires every subscriber. The tunnel still receives them, but make sure you're reproducing the action that triggers the event.

**`lumo dev stop` says "does not own this sandbox"** — sandboxes are owned by the user who created them. If a teammate spawned it, ask them to destroy it (or wait for the cleanup cron to expire it).

---

## Releasing

Releases are cut with [GoReleaser](https://goreleaser.com) from `.goreleaser.yaml` (schema v2). Pushing a `vX.Y.Z` tag builds the archive matrix, publishes the GitHub release with `checksums.txt`, and pushes package-manager manifests:

| Channel | Where it lands |
|---|---|
| Homebrew | cask `Casks/lumo.rb` in [`LumoAuth/homebrew-tap`](https://github.com/LumoAuth/homebrew-tap), completions generated from the binary |
| Scoop | `lumo.json` in [`LumoAuth/scoop-bucket`](https://github.com/LumoAuth/scoop-bucket) |
| winget | manifests pushed to the [`LumoAuth/winget-pkgs`](https://github.com/LumoAuth/winget-pkgs) fork on a `lumo-<version>` branch, with an automatic PR to `microsoft/winget-pkgs` |

Pre-release tags (`-rc.*`, `-beta.*`) create the GitHub release but skip all three publishers (`skip_upload: auto`).

```bash
go test ./... && gofmt -l . && goreleaser check    # pre-flight (GoReleaser v2.x)
goreleaser release --snapshot --clean              # rehearse: builds everything into dist/, publishes nothing
git tag -a v1.2.0 -m v1.2.0 && git push origin v1.2.0
GITHUB_TOKEN=<cross-repo PAT> goreleaser release --clean
```

The token must be able to write to this repo, the tap, the bucket and the winget fork, and open PRs on `microsoft/winget-pkgs`; a classic PAT with the `repo` scope does. The version string is injected from the tag via ldflags, so there is nothing to bump in source.

<details>
<summary><strong>Project structure</strong></summary>

<a name="project-structure"></a>

```
cli/
├── main.go                            # Entry point
├── go.mod
├── .goreleaser.yaml                   # Build matrix, ldflags, brew/scoop/winget publishing
├── install.sh                         # One-line install script
├── cmd/
│   ├── root.go                        # Root command, global flags, exit codes
│   ├── version.go                     # `lumo version` / `--version`
│   ├── login.go                       # `lumo login/logout/whoami`
│   ├── profile.go                     # `lumo profile list/use/create/delete/show`
│   ├── upgrade.go                     # `lumo upgrade` (channel-aware self-update)
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
    ├── config/credentials.go          # Multi-profile tokens (credentials.yaml)
    ├── client/client.go               # HTTP client w/ Bearer + ApiKey fallback
    ├── upgrade/upgrade.go             # Install-channel detection, checksum verify, self-replace
    └── output/output.go               # Table/JSON/YAML output
```

</details>

## License

[MIT](LICENSE) © 2026 LumoAuth LLC
