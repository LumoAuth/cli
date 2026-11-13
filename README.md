<div align="center">

<a href="https://lumoauth.dev">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://docs.lumoauth.dev/img/lumoauth-logo-horizontal-reversed.svg">
    <img src="https://docs.lumoauth.dev/img/lumoauth-logo-horizontal.svg" alt="LumoAuth" height="36">
  </picture>
</a>

# LumoAuth CLI

**Run your LumoAuth organization from the terminal with `lumo`.**<br>
Users, roles, OAuth apps, AI agents, MCP servers, webhooks, audit logs and MFA policy, with JSON output for scripts and coding agents.

[![Release](https://img.shields.io/github/v/release/LumoAuth/cli?label=release&color=111)](https://github.com/LumoAuth/cli/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/LumoAuth/cli?color=111)](go.mod)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%C2%B7%20Linux%20%C2%B7%20Windows-111)](#install)
[![License: MIT](https://img.shields.io/badge/license-MIT-111)](LICENSE)

[Quick start](#quick-start) · [What you can do](#what-you-can-do) · [Install](#install) · [Sign in](#sign-in) · [Commands](#commands) · [Coding agents](#use-lumo-from-a-coding-agent) · [Docs](https://docs.lumoauth.dev/cli)

</div>

---

## Quick start

**1. Install `lumo`:**

```bash
brew install lumoauth/tap/lumo
```

On Windows, or on a computer without Homebrew, see [Install](#install).

**2. Sign in** to your organization:

```bash
lumo login --org acme-corp
```

Choose your region, United States or European Union. Then approve the sign-in in the browser window that opens.

**3. Check the setup:**

```console
$ lumo doctor
✓ profile       default (~/.lumoauth/credentials.yaml)
✓ organization  acme-corp
✓ server        https://app.lumoauth.dev
✓ credential    jane@acme.com, login token, expires 2026-11-10T23:12:09Z
✓ scopes        openid profile email admin
✓ discovery     issuer https://app.lumoauth.dev/orgs/acme-corp
✓ admin api     GET /admin/organization succeeded

lumo 1.1.0
```

**4. Run a command:**

```bash
lumo users list
```

No organization yet? [Sign up at app.lumoauth.dev](https://app.lumoauth.dev/register). The slug that you choose there is the one that `lumo login --org` takes.

In CI and scripts, use an [API key](#use-an-api-key-in-ci-and-scripts) instead of the browser:

```bash
LUMO_API_KEY=lmk_… LUMO_ORG=acme-corp lumo users list -o json
```

## What you can do

Every command has the form `lumo <resource> <verb>`. Some common tasks:

| To | Run |
|---|---|
| Find a user | `lumo users list --search jane` |
| Let a user who lost their phone sign in and enroll a new factor | `lumo users tap jane@acme.com --reason "Lost phone; verified by video call"` |
| Require MFA, with 14 days for people to enroll | `lumo mfa policy set --requirement required --grace-days 14` |
| See how many people use each MFA factor | `lumo mfa coverage` |
| Register an AI agent with a key that expires in 90 days | `lumo agents create --name "Support bot" --key-expires-in-days 90` |
| Call an MCP server with an agent's token | `lumo mcp test <server-id> <agent-id>` |
| Send live webhook events to your laptop | `lumo tunnel --to http://localhost:3000/webhooks` |
| Send a failed webhook delivery again | `lumo webhooks deliveries replay <webhook-id> <delivery-id>` |
| Start a throwaway organization for a pull request | `lumo dev start --name pr-1234 --ttl-hours 24` |
| Start a Next.js app that signs in with your organization | `lumo init --framework next` |
| Check an Auth0 export before you import it | `lumo migrate auth0 --file users.json --dry-run` |
| Export the October audit log as CSV | `lumo logs export --export-format csv --from 2026-10-01 --to 2026-10-31` |
| Call an Admin API endpoint that has no command | `lumo api GET /users --query search=jane` |

> [!TIP]
> Do you use a coding agent? Install the [`lumoauth-cli`](https://github.com/lumoauth/agent-skills/tree/main/skills/lumoauth-cli) skill with `npx skills add lumoauth/agent-skills --skill lumoauth-cli`. It teaches Claude Code, Cursor, Codex and other agents every `lumo` command, the JSON output and the exit codes.

## How `lumo` works with you

- **It checks the setup before you start.** `lumo doctor` tests the profile, the organization, the server, the credential and its scopes, and one real Admin API call. It prints the fix for each check that fails and exits with status 1, so you can run it in CI.
- **It keeps sessions short and narrow.** `lumo login` saves the session in a file that only you can read. Access tokens last one hour, and `lumo` refreshes them for you. Pass `--scope` to sign in with fewer permissions.
- **It asks before it removes access.** Commands that remove a factor or unlink an identity ask for confirmation in a terminal. Scripts must pass `--yes`.
- **Each error includes the fix.** An error comes with a hint, such as the exact `lumo login --scope …` command for a `403`. Each kind of error has its own [exit code](#use-lumo-from-a-coding-agent).
- **Its output fits scripts.** When the output goes to a pipe or a file, `lumo` prints JSON instead of a table.
- **Nothing is out of reach.** `lumo api` calls any Admin API endpoint, including endpoints that have no command yet.

## Install

| To | Run |
|---|---|
| Install on macOS or Linux with Homebrew | `brew install lumoauth/tap/lumo` |
| Install on Windows with Scoop | `scoop bucket add lumoauth https://github.com/lumoauth/scoop-bucket`, then `scoop install lumo` |
| Install on Windows with winget | `winget install --id LumoAuth.lumo` |
| Install on Linux, macOS or WSL without a package manager | `curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh \| sh` |
| Build with Go on any platform | `go install github.com/lumoauth/cli@latest` |
| Upgrade an installed `lumo` | `lumo upgrade` |

Each channel installs the same statically linked binary. Release archives exist for macOS (Apple silicon, Intel), Linux (x86_64, arm64, i386) and Windows (x86_64, i386). For Windows on Arm, use `go install`.

### Homebrew (macOS / Linux)

```bash
brew install lumoauth/tap/lumo
```

The tap is [`LumoAuth/homebrew-tap`](https://github.com/LumoAuth/homebrew-tap), and `lumo` is a cask. On macOS the cask removes the Gatekeeper quarantine attribute, so the unsigned binary runs without a warning. Homebrew also installs the shell completions for bash, zsh and fish. To upgrade, run `brew upgrade lumoauth/tap/lumo` or `lumo upgrade`.

### Scoop (Windows)

```powershell
scoop bucket add lumoauth https://github.com/lumoauth/scoop-bucket
scoop install lumo
```

The bucket is [`LumoAuth/scoop-bucket`](https://github.com/LumoAuth/scoop-bucket). To upgrade, run `scoop update lumo` or `lumo upgrade`.

### winget (Windows)

```powershell
winget install --id LumoAuth.lumo
```

Each release goes to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs/pulls?q=LumoAuth.lumo) as a pull request. You can install the release after Microsoft merges that pull request, usually within a few days. Until then, use Scoop or the direct download. To upgrade, run `winget upgrade --id LumoAuth.lumo` or `lumo upgrade`.

### Install script (Linux / macOS / WSL)

```bash
curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
```

The script needs no `sudo`. It does these steps:

1. finds your operating system and architecture
2. downloads the latest release
3. compares its SHA-256 with the release's `checksums.txt`, and stops on a mismatch
4. installs `lumo` in `~/.local/bin`
5. adds that folder to `PATH` for bash, zsh, fish or sh, if it is missing

Restart your shell, then run `lumo --version`. `LUMO_SKIP_CHECKSUM=1` turns off the checksum test. Do not use it.

macOS quarantines a binary that you install this way, because the binary is not notarized. Remove the attribute once, or use Homebrew:

```bash
xattr -d com.apple.quarantine ~/.local/bin/lumo
```

<details>
<summary><b>Direct download</b>: get an archive from the releases page and verify it yourself</summary>

<a name="direct-download"></a>

<br>

Each release on the [releases page](https://github.com/LumoAuth/cli/releases/latest) has one archive per platform and a `checksums.txt` file. The archive names have the form `lumo_<OS>_<Arch>`:

- `OS` is `Darwin`, `Linux` or `Windows`
- `Arch` is `x86_64`, `arm64` or `i386`
- Windows archives are `.zip` files, and the others are `.tar.gz` files

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
# then add that folder to your PATH
```

`lumo upgrade` also works for a binary that you placed by hand. It downloads and verifies the next release in the same way.

</details>

<details>
<summary><b><code>go install</code></b>: build from the module on any platform that Go supports</summary>

<a name="go-install"></a>

<br>

You need Go 1.22 or later. Go builds `lumo` for your platform, including platforms without a release archive, such as Windows on Arm.

```bash
go install github.com/lumoauth/cli@latest          # latest release
go install github.com/lumoauth/cli@v1.1.0          # a specific version
```

The module root is the main package, so the binary in `$(go env GOPATH)/bin` is named `cli`. Rename it:

```bash
mv "$(go env GOPATH)/bin/cli" "$(go env GOPATH)/bin/lumo"
```

`lumo --version` prints `dev` for a `go install` build. Only the release pipeline adds the version, the commit and the date.

</details>

<details>
<summary><b>Build from source</b></summary>

<a name="build-from-source"></a>

<br>

```bash
git clone https://github.com/LumoAuth/cli.git && cd cli
go build -o lumo .

# Optional: install into $GOPATH/bin
go install .
```

</details>

### Shell completions

Homebrew installs the completions for you. For the other channels, generate them with `lumo`:

```bash
lumo completion bash | sudo tee /etc/bash_completion.d/lumo > /dev/null   # bash
lumo completion zsh > "${fpath[1]}/_lumo"                                 # zsh
lumo completion fish > ~/.config/fish/completions/lumo.fish               # fish
lumo completion powershell | Out-String | Invoke-Expression               # PowerShell
```

`lumo completion --help` has the full steps for each shell.

### Upgrading

```bash
lumo upgrade            # find the install channel and upgrade in place
lumo upgrade --check    # only report whether a newer version exists
lumo upgrade --exec     # upgrade without a confirmation prompt
```

`lumo upgrade` finds out how you installed `lumo` and upgrades through the same channel:

- **Homebrew, Scoop or winget:** `lumo` runs the package manager, so the package manager's records stay correct.
- **Install script or a binary that you placed by hand:** `lumo` downloads the release archive for your system, verifies it against `checksums.txt` and replaces the binary in one step.

On Windows, a running `lumo.exe` sometimes cannot be replaced. Then `lumo` saves the new version as `lumo.exe.new` and prints the steps to finish the upgrade.

## Sign in

`lumo` accepts two kinds of credentials. Both work with every command.

### Sign in with your browser

Use this method on your own computer.

```bash
lumo login --org acme-corp
```

What happens:

1. `lumo` asks for your region: United States (`app.lumoauth.dev`) or European Union (`eu.app.lumoauth.dev`). Pass `--base-url` or set `LUMO_BASE_URL` to skip this question.
2. `lumo` starts the OAuth device flow with `POST /orgs/{org}/api/v1/oauth/device_authorization`. It uses the first-party client `lumoauth-cli` and asks for the scopes `openid profile email admin`.
3. `lumo` prints a user code and opens the verification page in your browser.
4. You approve the sign-in in the portal.
5. `lumo` saves the tokens and the granted scopes in `~/.lumoauth/credentials.yaml`, with file mode 0600.

Later commands use the saved token. An access token lasts one hour. `lumo` gets a new one with the 30-day refresh token before it expires, and once more when the server returns `401`.

```bash
lumo whoami    # profile, organization, credential, scopes and expiry (-o json for scripts)
lumo doctor    # checks the credential against the server
lumo logout    # removes the tokens of the active profile
```

**Scopes.** Every resource command needs the `admin` scope or a resource scope such as `admin:users:read`. The Admin API accepts a login token only when the token has one of these scopes *and* the signed-in user has the `settings.manage` permission. For a session with fewer permissions, ask for resource scopes:

```bash
lumo login --org acme-corp --scope openid,admin:users:read,admin:audit:read
```

A command that needs another scope fails with `403`. The error names the missing scope and the `lumo login --scope …` command that adds it.

On your first sign-in to an organization, LumoAuth creates a public OAuth client named `lumoauth-cli:<slug>`. The client allows the device code and refresh token grants and the default scopes. The device flow already binds the approval to the user code, so the client does not use PKCE. An administrator can remove scopes from this client to require narrower sessions. A request for a scope that the client does not allow fails with `invalid_scope` and a hint.

### Use an API key in CI and scripts

```bash
export LUMO_API_KEY=lmk_your_key_here
export LUMO_ORG=acme-corp
lumo users list -o json
```

Create API keys in the portal at `https://app.lumoauth.dev/orgs/acme-corp/portal/settings/api-keys`. LumoAuth checks the scopes of a key for each resource:

- A key with `admin:users:read` can list users and do nothing else.
- A `:read` scope never allows a write.
- The key stops working when its creator is deactivated or loses `settings.manage`.

Create one key for each job. Give it only the [scopes](#api-scopes) that the job's commands need.

### Named profiles (multi-org)

With profiles, you can work in several organizations or regions without signing in again. Examples are two customer organizations, or one organization in the US and one in the EU.

```bash
lumo login --profile acme --org acme-corp        # first organization
lumo login --profile clientb --org clientb-corp  # second organization

lumo profile list                 # all profiles (* marks the active one)
lumo profile use acme             # change the active profile
lumo users list --profile clientb # run one command with another profile
LUMO_PROFILE=clientb lumo users list   # choose the profile with an environment variable (CI, direnv)

lumo profile show [name]          # show a profile (tokens masked)
lumo profile create staging --org acme-staging      # profile without tokens, for an API key
lumo profile delete clientb       # remove a profile and its tokens
```

Each profile keeps its own `org_id`, `base_url` and login tokens in `~/.lumoauth/credentials.yaml`. The `--profile` flag comes first, then `LUMO_PROFILE`, then `current_profile` in the credentials file. The `--org` flag still overrides the organization of the profile for one command.

`lumo` moves an older single-profile credentials file to the new format on first use. Your old credentials become the `default` profile, and `lumo` keeps the original file as `~/.lumoauth/credentials.yaml.bak`.

`lumo logout` signs out of the active profile only. Other profiles stay signed in.

### Configuration precedence

`lumo` takes the organization and the base URL from the first place that sets them:

| Priority | Source | Example |
|----------|--------|---------|
| 1 (highest) | Flags | `--org acme-corp --base-url https://eu.app.lumoauth.dev` |
| 2 | Environment variables | `LUMO_ORG`, `LUMO_BASE_URL`, `LUMO_API_KEY` |
| 3 | Config file | `~/.lumoauth/config.yaml` |
| 4 (fallback) | Active profile | `~/.lumoauth/credentials.yaml` (from `lumo login`) |

The output format comes from `-o`, then `LUMO_OUTPUT_FORMAT`, then `format` in the config file. See [Output formats](#output-formats).

`lumo` chooses the **credential** separately:

1. the login token of the active profile, when that profile is for the organization that the command uses
2. otherwise, the API key
3. otherwise, the command stops with exit code 2 and a hint

So `lumo` never sends a login token to another organization by mistake. `lumo whoami` shows which credential the next command uses.

### Config file format

```yaml
# ~/.lumoauth/config.yaml: settings that you edit (no secrets)
org_id: acme-corp
base_url: https://app.lumoauth.dev   # or https://eu.app.lumoauth.dev
format: table                        # table, json or yaml
insecure: false
```

```yaml
# ~/.lumoauth/credentials.yaml: written by lumo login, logout and profile (mode 0600)
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

Change the settings with these commands:

```bash
lumo config init          # answer questions to create the file
lumo config show          # print the current configuration
lumo config set org_id acme-corp
```

## Commands

Every command has the form `lumo <resource> <verb>`. For the full list, run `lumo --help` or `lumo <resource> --help`.

[Session and setup](#session-and-setup) · [Developer loop](#developer-loop) · [Organization](#organization) · [People and access](#people-and-access) · [Apps and agents](#apps-and-agents) · [Events](#events) · [Migration](#migration) · [Raw API access](#raw-api-access)

| Area | Commands |
|---|---|
| Session and setup | [`login`](#login--logout--whoami) · `logout` · `whoami` · [`doctor`](#doctor) · [`profile`](#profile--named-credential-profiles) · [`config`](#config-file-format) · [`upgrade`](#upgrade) · [`completion`](#shell-completions) |
| Developer loop | [`dev`](#dev--sandbox-organizations) · [`tunnel`](#tunnel--forward-webhooks-to-localhost) · [`init`](#init--create-a-starter-project) |
| Organization | [`org`](#org--organization-profile-and-settings) · [`settings`](#settings) · [`mfa`](#mfa--mfa-policy-and-coverage) · [`social`](#social--social-login-providers) |
| People and access | [`users`](#users) · [`identities`](#federated-identities-saml--ldap-relink) · [`roles`](#roles) · [`groups`](#groups) · [`permissions`](#permissions) |
| Apps and agents | [`apps`](#apps--oauth-applications) · [`agents`](#agents--ai-agents) · [`mcp`](#mcp--mcp-servers) · [`sessions` / `tokens`](#sessions--tokens) |
| Events | [`webhooks`](#webhooks) · [`logs`](#logs--audit-logs) |
| Migration | [`migrate`](#migrate--import-from-auth0) |
| Raw API access | [`api`](#api) |

### Global flags

```
--api-key string    Organization API key lmk_… (overrides LUMO_API_KEY)
--org string        Organization slug (overrides LUMO_ORG and the active profile; --org-id still accepted)
--base-url string   Base URL (overrides LUMO_BASE_URL)
--profile string    Named credentials profile (overrides LUMO_PROFILE and current_profile)
-o, --output string Output format: table, json, yaml (default: table in a terminal, json when piped)
--insecure          Skip TLS verification (local dev only)
-q, --quiet         Suppress non-essential output
-v, --verbose       Enable verbose output
```

### Session and setup

Sign in, check the setup, switch between organizations and keep `lumo` up to date.

#### `login` / `logout` / `whoami`

```bash
lumo login [--org acme-corp] [--base-url https://eu.app.lumoauth.dev] [--scope openid,admin:users:read] [--profile name] [--no-browser] [--insecure]
lumo logout
lumo whoami [-o json]
```

- `--scope` asks for fewer scopes. The default is `openid,profile,email,admin`.
- `--no-browser` prints the verification URL and does not open a browser. Use it on a remote or headless machine.
- `--profile` signs in to a named profile, and creates the profile if it is new.
- `logout` signs out of the active profile only.
- `whoami` shows the credential that the next command sends and, for a login token, its scopes.

#### `doctor`

```bash
lumo doctor            # ✓ or ✗ for each check, with the fix for each failure
lumo doctor -o json    # {"ok": bool, "checks": [...]}, exit code 1 when a check fails
```

`lumo doctor` checks the active profile, the organization, the server (with OIDC discovery), the credential and its scopes, and one real Admin API call. Run it after `lumo login`, after you create an API key, and in CI before the real work.

#### `profile` — Named credential profiles

```bash
lumo profile list                # all profiles; * marks the active one
lumo profile use <name>          # set the active profile
lumo profile show [name]         # profile details (tokens masked)
lumo profile create <name> [--org slug] [--base-url url] [--use]
lumo profile delete <name> [-f]
```

For the full workflow, see [Named profiles](#named-profiles-multi-org).

#### `upgrade`

```bash
lumo upgrade [--check] [--exec]
```

`lumo upgrade` uses the channel that installed `lumo`. See [Upgrading](#upgrading).

### Developer loop

Try a change in a throwaway organization, receive webhooks on your laptop and start a new app.

#### `dev` — Sandbox organizations

```bash
lumo dev start [--name pr-1234] [--ttl-hours 24]
lumo dev list
lumo dev stop sandbox-pr-1234-7a3b21
```

A sandbox is a separate organization with its own slug. Use it for branch previews, pull request environments or experiments. Each sandbox has a time to live: 24 hours by default, 7 days at most. A daily cleanup job deletes each sandbox after its time to live ends.

`lumo dev start` prints the slug of the sandbox. A new sandbox holds an administrator role, a signing key, a Starter App client and one API key that can administer it. It holds no user who can sign in, and your own login or API key does not reach into it. `lumo dev start` saves the sandbox's API key as a profile that is named after the sandbox, so every command works when you pass `--org <sandbox-slug>` (for example `lumo --org <sandbox-slug> users create --email dev@example.com`). With `-o json` the command prints the server's answer, which contains the key once in `data.api_key`. To delete a sandbox early, run `lumo dev stop`, which also removes the saved key. Only its owner, the user who created it, can delete it.

A sandbox uses the plan of the organization that created it and cannot create sandboxes itself.

To start sandboxes, your organization needs the **Developer sandboxes** add-on ($199/month, Business plan only, up to 10 live sandboxes for each owner). Without the add-on, `dev start` fails with `403 feature_not_in_plan`, and `lumo` prints the billing page where an administrator can add it. `dev list` and `dev stop` work without the add-on, so you can always delete existing sandboxes.

#### `tunnel` — Forward webhooks to localhost

```bash
lumo tunnel --to http://localhost:3000/webhooks
lumo tunnel --to http://localhost:8000/hooks --filter user.created --filter user.deleted
```

`lumo tunnel` works like `stripe listen --forward-to`:

1. `lumo` asks the server to register a temporary `tunnel://<session>` webhook for all events (`*`).
2. `lumo` opens an SSE stream of the deliveries for that webhook.
3. `lumo` sends each event as a POST to the `--to` URL, with the `X-LumoAuth-Event` and `X-LumoAuth-Delivery` headers.
4. When you press Ctrl+C, the server deletes the tunnel webhook.

The server ends each stream after a maximum time, and `lumo` then connects again.

#### `init` — Create a starter project

```bash
lumo init --framework next [--dir ./my-app] [--org acme-corp] [--client-id lumo_...] [--sdk-path ~/src/lumoauth]
lumo init --framework express
lumo init --framework fastapi
lumo init --framework go
```

`lumo init` creates a small project that already works with your organization. Its `.env` file has the organization slug and the base URL from your current credentials. With `--client-id`, it also has the OAuth client ID.

| Framework | What you get |
|---|---|
| `next` | Next.js 14 (App Router) and `@lumoauth/nextjs`, with `<SignIn />` and `<UserButton />` in place |
| `express` | Express and the `@lumoauth/express` middleware, with a session cookie and a `requireAuth` example |
| `fastapi` | FastAPI and the `lumoauth.fastapi` router, with a `get_current_user` dependency |
| `go` | Go (chi) and the `lumo-auth-go` middleware, with routes that need a session cookie |

The project installs the SDK from its public registry: npm, PyPI or the Go proxy. To work on the SDK itself, pass `--sdk-path` or set `LUMO_SDK_PATH`. Point it at a LumoAuth source checkout, the folder that holds `sdk-js`, `sdk-python` and `sdk-go`. The project then installs the SDK from those local sources.

`lumo init` does not change existing files unless you pass `--force`.

### Organization

The organization profile, its settings, the MFA policy and social login.

#### `org` — Organization profile and settings

```bash
lumo org get
lumo org update --name "ACME Corporation"
lumo org update --set timezone=Europe/Berlin --set allow_root_admin_login=false
lumo org update --set security.dpop_require_nonce=true
```

You can change only the documented settings keys that are not security-sensitive. For any other key, `lumo` lists the rejected keys and saves nothing. Dotted keys nest, and keys with object values merge one key at a time.

#### Settings

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

The areas are `general`, `authentication` (`auth`), `security`, `email`, `branding`, `scim` and `organization` (`org`, `tenant`). You can repeat `--set key=value`, and dotted keys nest. The MFA requirement and factors are no longer `auth` settings. Use `lumo mfa policy set`.

#### `mfa` — MFA policy and coverage

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

- `--requirement` is `off`, `optional`, `required` or `risk_based`.
- `--allowed-factors` takes `passkey,push,totp,sms_otp,email_otp`.
- `policy set` sends only the flags that you pass, as a partial update.
- Tiers go from 1 (strongest, passkey) to 4 (basic, SMS or email).

#### `social` — Social login providers

```bash
lumo social list
lumo social get <id>
lumo social create --provider google --client-id <id> --client-secret <secret> [--name "Google"] [--scopes openid,profile,email] [--enable]
lumo social delete <id>
```

`social create` adds the provider in the disabled state. Pass `--enable` to show it on the sign-in page at once. Without `--name`, the display name is the provider type with a capital letter, such as `Google`. Without `--scopes`, LumoAuth asks the provider for `openid`, `profile` and `email`.

### People and access

Users, their sign-in factors and external identities, and the roles, groups and permissions that control access.

#### Users

```bash
lumo users list [--search "john"] [--role admin] [--blocked] [--page 1] [--limit 25]
lumo users get <user-id-or-email>
lumo users create --email user@example.com [--name "John Doe"] [--password pw] [--roles admin,editor]
lumo users update <id> [--name "New Name"] [--email new@email.com]
lumo users delete <id>
lumo users block <id>
lumo users unblock <id>
lumo users set-password <id> --password "newpassword"

lumo users authenticators list <user-id-or-email>          # ID, type, name, state, tier, default, last used, status
lumo users authenticators remove <user-id> <authenticator-id> [--yes]   # for example totp:12, a lost phone
lumo users tap <user-id-or-email> --reason "Lost phone; verified by video call" [--ttl 60] [--multi-use] [--mfa-challenge <id>]

lumo users roles get <user-id>
lumo users roles set <user-id> --roles admin,editor
lumo users groups get <user-id>
lumo users groups set <user-id> --groups team-a,team-b
```

Administrators cannot turn MFA off for a user. `lumo users mfa-reset` is deprecated and fails without a call to the server.

For a user who is locked out, issue a **temporary access code** with `lumo users tap`:

- The code replaces the second factor once. With `--multi-use`, it works until it expires.
- With the code, the user can only sign in and enroll a new factor.
- `lumo` prints the code exactly once.
- The reason is required. LumoAuth writes it to the audit log and shows it to the user.

When you use a `lumo login` session, the server also asks for a new MFA check of your own. Pass the ID of an approved `step_up` challenge with `--mfa-challenge`. API keys do not need it.

`lumo users authenticators remove` revokes one factor. In a terminal it asks for confirmation; scripts must pass `--yes`. If MFA is required, the user enrolls again at the next sign-in.

#### Federated identities (SAML / LDAP relink)

```bash
lumo users identities list <user-id-or-email>                     # SAML IdP + NameID, LDAP directory + DN, social links
lumo users identities link <user> --saml-idp <idp-id> --name-id <nameid> [--mfa-challenge <id>]
lumo users identities link <user> --ldap-config <id> [--dn "uid=jane,ou=people,dc=acme,dc=com"] [--ldap-only] [--mfa-challenge <id>]
lumo users identities unlink <user> --type saml|ldap|social [--yes] [--mfa-challenge <id>]

lumo identities legacy-saml [--idp <idp-id>]                      # users with a legacy bare-NameID SAML link
lumo identities legacy-saml --idp <idp-id> --relink --dry-run     # preview
lumo identities legacy-saml --idp <idp-id> --relink [--user <id> ...] --yes
```

A link sets which external identity signs in as the user. So when you link or unlink an identity, LumoAuth signs the user out everywhere, sends them an email and writes an audit event (`identity.link.created`, `identity.link.removed` or `identity.link.bulk_relinked`). You cannot change the links of an account that has permissions that you do not have.

- Without `--dn`, the server looks up the directory entry by the user's email, then by the username.
- `--ldap-only` also turns off the local password.
- A `409` (`identity_conflict`) means that another user already has that identity, or that the user has a link to another federated source. Unlink that source first.
- With a `lumo login` session, pass an approved `step_up` challenge with `--mfa-challenge`. API keys do not need it.

Some SAML links are older than the binding to one IdP. While the organization has more than one SAML IdP, LumoAuth refuses these users at SAML sign-in. `lumo identities legacy-saml` lists these users, each with the IdP whose allowed email domains match them. `--relink` binds them to that IdP: all the users that the domains match, or only the users that you name with `--user`.

#### Roles

```bash
lumo roles list
lumo roles get <id-or-slug>
lumo roles create --name "Editor" [--description "..."] [--permissions doc.edit,doc.view]
lumo roles update <id> [--name "New Name"] [--permissions p1,p2]
lumo roles delete <id>
```

`--permissions` replaces all the permissions of the role. LumoAuth keeps each permission that you cannot grant yourself. It skips a slug that does not exist or that you cannot grant, and `lumo` prints a warning for each skipped slug.

#### Groups

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

#### Permissions

```bash
lumo permissions list
lumo permissions get <id-or-slug>
lumo permissions create --slug "document.edit" [--name "Edit Documents"]
lumo permissions update <id> [--name "New Name"]
lumo permissions delete <id>
```

### Apps and agents

OAuth applications, AI agents, MCP servers, and the sessions and tokens that they hold.

#### `apps` — OAuth applications

```bash
lumo apps list [--search "my-app"]
lumo apps get <client-id>
lumo apps create --name "My App" [--type web|spa|native|m2m] [--redirect-uris http://localhost:3000/callback]
lumo apps update <id> [--name "Updated App"]
lumo apps delete <id>
lumo apps rotate-secret <id>
```

#### `agents` — AI agents

```bash
lumo agents list [--search "assistant"]
lumo agents get <agent-id>
lumo agents create --name "My Agent" [--type api_key] [--capabilities read_data,write_data] [--key-expires-in-days 90]
lumo agents update <id> [--name "Updated Agent"] [--key-expires-in-days 90]
lumo agents delete <id>
lumo agents enable <id>
lumo agents disable <id>
lumo agents rotate-credentials <id> [--key-expires-in-days 90 | --key-expires-at 2027-03-31 | --key-never-expires]
lumo agents token <id> [--ttl 3600] [--scopes a,b] [--resource https://mcp.example.com/mcp]   # get a bearer token for a test
```

`--type` is the identity type: `api_key` (the default), `client_secret`, `workload`, `aauth`, `private_key_jwt` or `custom`.

Set the lifetime of the agent's API key with one of three flags:

- `--key-expires-in-days`: 1 to 3650 days; 180 is the recommended value
- `--key-expires-at`: a date, which lasts to the end of that day in UTC, or an RFC 3339 time
- `--key-never-expires`

The effect of the flag depends on the command:

- On `create`, it sets the lifetime of the new agent's key. Without a flag, the key expires after 180 days.
- On `update`, it changes the lifetime of the current key.
- On `rotate-credentials`, it sets the lifetime of the new key. Without a flag, the new key keeps the current expiry date. If that date is in the past, the new key gets 180 days.

#### `mcp` — MCP servers

```bash
lumo mcp servers                       # list registered MCP servers (transport, scopes, basic scopes)
lumo mcp discovery <server-id>         # print the server's protected resource metadata
lumo mcp test <server-id> <agent-id>   # get an agent token for the server and call it
lumo mcp check-tool <server-id> <tool> --subject agent:<agent-id> --client-id <server-client>
```

`mcp test` gets a token with the server's Resource URI as its audience. Then it sends one stateless MCP 2026-07-28 request, `tools/list` by default. The request has the `MCP-Protocol-Version` and `Mcp-Method` headers, and the protocol version in `_meta`. You cannot test a server that requires DPoP this way.

`mcp check-tool` makes the same check for each tool call as an MCP server does:

1. It gets a client credentials token for the server's own client, with the scope `mcp:authorize`.
2. It asks LumoAuth whether the subject may call the tool.

The client secret comes from `--client-secret` or `LUMO_MCP_CLIENT_SECRET`. The command exits with status 0 when LumoAuth allows the call, and with 1 when LumoAuth denies it.

#### Sessions & tokens

```bash
lumo sessions list [--user <user-id>]
lumo sessions revoke <session-id>
lumo sessions revoke-all --user <user-id>

lumo tokens list [--user <user-id>] [--client <client-id>]
lumo tokens revoke <token-id>
```

### Events

Webhooks for other systems and the audit log of everything that happens in the organization.

#### Webhooks

```bash
lumo webhooks list
lumo webhooks get <id>
lumo webhooks create --url https://example.com/hook --events user.created,user.login
lumo webhooks update <id> [--url "..." --rotate-secret] [--events e1,e2]
lumo webhooks delete <id>

lumo webhooks deliveries list <webhook-id> [--status pending|success|failed|dead_lettered] [--limit 50]
lumo webhooks deliveries get <webhook-id> <delivery-id>
lumo webhooks deliveries replay <webhook-id> <delivery-id>
```

LumoAuth generates the signing secret of a new webhook. `webhooks create` prints it once, so store it at once to verify signatures.

`deliveries list` shows each delivery with its status, the number of attempts and the time of the last attempt. `deliveries replay` sends a failed or dead-lettered delivery again.

#### `logs` — Audit logs

```bash
lumo logs list [--action login] [--user <user-id>] [--from 2025-01-01] [--to 2025-12-31] [--status failure]
lumo logs get <log-id>
lumo logs stats [--from 2025-01-01] [--to 2025-12-31]
lumo logs export [--export-format csv|json] [--from ...] [--to ...]
```

`--status` is the result of the event: `success`, `failure`, `pending` or `partial`. `--user` takes the user ID of the actor.

`logs export` prints the file to standard output. Redirect it to save it: `lumo logs export --export-format csv > audit.csv`.

### Migration

Move users from another identity provider.

#### `migrate` — Import from Auth0

```bash
lumo migrate auth0 --file users.json --dry-run     # read and validate only
lumo migrate auth0 --file users.json [--concurrent 4] [--limit 100]
lumo migrate auth0 --file users.json --send-reset  # also send each new user a password-reset email
```

`lumo migrate auth0` reads an Auth0 bulk user export and creates each user through the Admin API. It skips users whose email already exists, so you can run it again safely. `--concurrent` sets the number of parallel requests (1 to 16). `--limit` stops after that many users, for a migration in stages. `--send-reset` requests a password-reset email for each user that the run creates (one more request per user; the link in the email is short-lived). When the API answers `429`, the importer waits for the time in `Retry-After` and sends the request again, so a large import runs at the rate that the API allows.

`lumo` never copies Auth0 password hashes. Migrated users have no password, and they set one with your organization's first sign-in flow, such as a magic link or a password reset.

### Raw API access

#### `api`

Use `lumo api` for endpoints that have no command, or from an AI agent. Paths are relative to the Admin API of the current organization. Absolute API paths and URLs also work:

```bash
lumo api GET /users --query search=jane --query limit=5     # → /orgs/<org>/api/v1/admin/users?search=jane&limit=5
lumo api POST /roles --data '{"name": "Editor"}'
lumo api PATCH /organization --data '{"settings":{"allow_root_admin_login":false}}'
lumo api GET /orgs/<org>/api/v1/.well-known/openid-configuration   # absolute API path
lumo api GET /api/v1/authz/check                                   # global API
lumo api --raw GET /healthz                                        # relative to the base URL
```

For every route, see the [API reference](https://docs.lumoauth.dev/api-reference).

## Output formats

```bash
lumo users list                # a table in a terminal
lumo users list -o json        # JSON
lumo users list -o yaml        # YAML
lumo users list | jq '.data[].email'    # JSON, because the output goes to a pipe
lumo users list -o table | less        # -o table keeps the table in a pipe
```

Without `-o`, `lumo` prints a table in a terminal and JSON when the output goes to a pipe or a file. `LUMO_OUTPUT_FORMAT` and `format` in the config file set another default. A default of `table` still gives JSON in a pipe; only `-o table` forces a table there.

## Use lumo from a coding agent

`lumo` works well for AI coding agents such as Claude Code, Cursor, Codex and GitHub Copilot.

> [!TIP]
> Install the [`lumoauth-cli`](https://github.com/lumoauth/agent-skills/tree/main/skills/lumoauth-cli) skill so that your agent knows every command:
>
> ```bash
> npx skills add lumoauth/agent-skills --skill lumoauth-cli
> ```
>
> The [LumoAuth agent skills](https://github.com/lumoauth/agent-skills) also cover sign-in for apps, access control, agent identity and MCP servers.

| Feature | Details |
|---------|---------|
| Automatic JSON | Output is JSON when it goes to a pipe or a file. Pass `-o json` to be explicit. |
| Structured errors | In JSON mode, an error prints `{"error": true, "message", "hint", "status", "code", "details"}` on standard output. The `hint` gives the fix, such as the exact `lumo login --scope …` command. |
| Exit codes | `0` success · `1` error · `2` sign-in or permission · `3` not found · `4` invalid input · `5` rate limited |
| Quiet mode | `-q` leaves out decorative output |
| Raw API | `lumo api` reaches any endpoint that has no command |
| No questions | In a script, commands that ask for confirmation in a terminal stop with an error unless you pass `--yes`. `lumo login` asks for the region and the organization only when you leave out `--base-url` and `--org`. |

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

### Example script

```bash
#!/bin/bash
# Start a sandbox organization for a pull request and add test data
SLUG=$(lumo dev start --name pr-1234 -o json | jq -r '.data.slug')
lumo --org "$SLUG" users create --email demo@example.com --name "Demo"
lumo --org "$SLUG" agents create --name "PR Bot"
echo "Sandbox $SLUG ready"
```

## Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LUMO_API_KEY` | Organization API key (`lmk_...`) | — |
| `LUMO_ORG` | Organization slug (`LUMO_ORG_ID` still accepted) | organization of the active profile |
| `LUMO_BASE_URL` | LumoAuth server URL | `https://app.lumoauth.dev` (US) |
| `LUMO_PROFILE` | Named credentials profile | `current_profile` from credentials.yaml |
| `LUMO_OUTPUT_FORMAT` | Default output format (`table`, `json` or `yaml`) | table in a terminal, JSON in a pipe |
| `LUMO_INSECURE` | Skip TLS verification (`true`/`1`) | `false` |
| `LUMO_CONFIG_DIR` | Custom config folder | `~/.lumoauth` |
| `LUMO_MCP_CLIENT_SECRET` | Client secret of the MCP server for `lumo mcp check-tool` | — |
| `LUMO_SDK_PATH` | Local SDK sources for `lumo init` | — |

## API scopes

LumoAuth checks scopes for each resource and refuses a call when a scope is missing:

- An API key has the scopes that you chose when you created it.
- A login token has the scopes that `lumo login --scope` asked for. The default is the `admin` scope, which covers every row below, within the permissions of the user.
- A read scope never allows a write, and a scope for one resource never allows another resource.

`lumo settings get` needs `admin:settings:read` for every area, and `lumo org update` needs `admin:settings:write`.

| Command | Required scope |
|---------|---------------|
| `users list/get` | `admin:users:read` |
| `users create/update/delete` | `admin:users:write` |
| `users authenticators list`, `mfa coverage` | `admin:users:read` |
| `users authenticators remove`, `users tap` | `admin:users:write` |
| `users identities list`, `identities legacy-saml` (list) | `admin:users:read` |
| `users identities link/unlink`, `identities legacy-saml --relink` | `admin:users:write` |
| `mfa policy get` / `mfa policy set` | `admin:settings:read` / `admin:settings:write` |
| `roles list/get` | `admin:roles:read` |
| `roles create/update/delete` | `admin:roles:write` |
| `groups list/get` | `admin:groups:read` |
| `groups create/update/delete` | `admin:groups:write` |
| `apps list/get` | `admin:clients:read` |
| `apps create/update/delete` | `admin:clients:write` |
| `agents list/get` | `admin:agents:read` |
| `agents create/update/delete` | `admin:agents:write` |
| `webhooks list/get`, `webhooks deliveries list/get` | `admin:webhooks:read` |
| `webhooks create/update/delete`, `webhooks deliveries replay` | `admin:webhooks:write` |
| `tunnel` | `admin:webhooks:write` |
| `logs list/get/stats/export` | `admin:audit:read` |
| `permissions list/get` | `admin:permissions:read` |
| `permissions create/update/delete` | `admin:permissions:write` |
| `settings get`, `org get` | `admin:settings:read` |
| `settings update`, `org update` | `admin:settings:write` |
| `sessions/tokens` | `admin:sessions:read`, `admin:sessions:write` |
| `social list/get` | `admin:social:read` |
| `social create/delete` | `admin:social:write` |
| `dev start/list/stop` | `admin:settings:read` (list), `admin:settings:write` (start/stop) |
| `migrate auth0` | `admin:users:write` |
| `mcp servers/discovery/test` | `admin:agents:read`, `admin:agents:write` (`test` gets a token) |
| `mcp check-tool` | None: it uses the MCP server's own client credentials (`mcp:authorize`) |
| `doctor` | any scope that can read the organization (`admin`, `admin:settings:read`) |

## Troubleshooting

**`lumo --version` works, but `lumo login` says "command not found".** Restart your shell, so that `~/.local/bin` is on your `PATH`.

**The browser does not open during `lumo login`.** Pass `--no-browser` and copy the verification URL by hand. The sign-in still works.

**A script gets a table instead of JSON.** Look for `-o table` in the command. Only that flag keeps a table in a pipe.

**`tls: failed to verify certificate` with a local development server.** Run `lumo login --insecure` once. `lumo` saves this choice in the profile, for that profile's server only. Later commands need no flag, and production URLs keep full TLS verification. When you enter a custom server URL at the region question, `lumo` turns off TLS verification by itself for localhost, `.local`, `http://` and private network addresses (`10.x`, `172.16–31.x`, `192.168.x`).

**`stored credentials have expired`.** `lumo` refreshes the access token with the saved refresh token. You see this error when the refresh token expired or was revoked. Run `lumo login` again. Add `--profile <name>` if the credentials belong to a profile other than the active one.

**`403 forbidden` right after `lumo login`.** The token has no admin scope. Either you asked for fewer scopes with `--scope`, or an administrator removed scopes from the `lumoauth-cli:<slug>` client. The hint in the error names the missing scope. Run `lumo login` again with that scope, or use an API key. `lumo doctor` shows the scopes that you have.

**`invalid_scope` on `lumo login`.** The CLI client of the organization does not allow a scope that you asked for. Ask for fewer scopes with `--scope`, or ask an administrator to allow the scope under **Applications → LumoAuth CLI**.

**`404` from `lumo api`.** Paths are relative to `/orgs/<org>/api/v1/admin`, unless they start with `/orgs`, `/api` or `/.well-known`. For any other path relative to the base URL, pass `--raw`.

**Webhooks do not show up in `lumo tunnel`.** Make sure that you repeat the action that sends the event. Also run `lumo webhooks list` to find *other* webhooks for the same events. LumoAuth delivers each event to every webhook that subscribes to it, so the tunnel and the other webhooks all get it.

**`lumo dev stop` says "does not own this sandbox".** Only the user who created a sandbox can delete it. Ask that teammate to delete it, or wait for the cleanup job to remove it after its time to live.

## Contributing

You need Go 1.22 or later. Run the checks before you open a pull request:

```bash
go build -o lumo .
go test ./...
gofmt -l .           # prints nothing when the formatting is correct
```

The tests include a route check: each Admin API path in `cmd/` must exist in the LumoAuth OpenAPI spec. The test reads the spec from `LUMO_OPENAPI_PATH`, or from a checkout of [`LumoAuth/api-clients`](https://github.com/LumoAuth/api-clients) next to `cli/`. Without a spec, the test skips.

<details>
<summary><b>Releasing</b></summary>

<a name="releasing"></a>

<br>

[GoReleaser](https://goreleaser.com) builds each release from `.goreleaser.yaml` (schema v2). When you push a `vX.Y.Z` tag, it builds the archives for each platform, publishes the GitHub release with `checksums.txt` and pushes the package manager manifests:

| Channel | Destination |
|---|---|
| Homebrew | cask `Casks/lumo.rb` in [`LumoAuth/homebrew-tap`](https://github.com/LumoAuth/homebrew-tap), with completions generated from the binary |
| Scoop | `lumo.json` in [`LumoAuth/scoop-bucket`](https://github.com/LumoAuth/scoop-bucket) |
| winget | manifests on a `lumo-<version>` branch of the [`LumoAuth/winget-pkgs`](https://github.com/LumoAuth/winget-pkgs) fork, with an automatic pull request to `microsoft/winget-pkgs` |

Pre-release tags (`-rc.*`, `-beta.*`) create the GitHub release but skip all three package managers (`skip_upload: auto`).

```bash
go test ./... && gofmt -l . && goreleaser check    # pre-flight (GoReleaser v2.x)
goreleaser release --snapshot --clean              # rehearsal: builds everything into dist/, publishes nothing
git tag -a v1.2.0 -m v1.2.0 && git push origin v1.2.0
GITHUB_TOKEN=<cross-repo PAT> goreleaser release --clean
```

The token must be able to write to this repository, the tap, the bucket and the winget fork, and to open pull requests on `microsoft/winget-pkgs`. A classic PAT with the `repo` scope can do all of this. The release pipeline takes the version from the tag (through ldflags), so there is nothing to change in the source.

</details>

<details>
<summary><b>Project structure</b></summary>

<a name="project-structure"></a>

<br>

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
│   ├── doctor.go                      # `lumo doctor`
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
│   ├── users.go, roles.go, groups.go, apps.go, agents.go, mcp.go, …
│   ├── migrate.go                     # `lumo migrate auth0`
│   ├── route_drift_test.go            # Admin API paths vs the OpenAPI spec
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

## Resources

- [`lumo` documentation](https://docs.lumoauth.dev/cli)
- [LumoAuth documentation](https://docs.lumoauth.dev)
- [API reference](https://docs.lumoauth.dev/api-reference)
- [SDKs and libraries](https://docs.lumoauth.dev/developer/sdks)
- [LumoAuth agent skills](https://github.com/lumoauth/agent-skills)

## License

[MIT](LICENSE) © 2026 LumoAuth LLC
