package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/auth"
	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	loginOrgID    string
	loginBaseURL  string
	loginInsecure bool
	loginNoBrowse bool
	loginScopes   []string
)

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)

	loginCmd.Flags().StringVar(&loginOrgID, "org", "", "Organization slug (prompted if not set)")
	loginCmd.Flags().StringVar(&loginBaseURL, "base-url", "", "Override the LumoAuth base URL")
	loginCmd.Flags().BoolVar(&loginInsecure, "insecure", false, "Skip TLS verification (dev only)")
	loginCmd.Flags().BoolVar(&loginNoBrowse, "no-browser", false, "Do not auto-open the verification URL")
	loginCmd.Flags().StringSliceVar(&loginScopes, "scope", nil,
		"Scopes to request (comma-separated or repeated). Default: "+strings.Join(auth.DefaultScopes, ",")+
			". Narrow for least privilege, e.g. --scope openid,admin:users:read")
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to LumoAuth via browser (OAuth 2.0 device flow)",
	Long: `Authenticate this CLI against a LumoAuth tenant using the OAuth 2.0
Device Authorization Grant (RFC 8628). No API key required — the CLI
opens a browser to the verification URL, you confirm in the dashboard,
and credentials are stored at ~/.lumoauth/credentials.yaml.

Credentials are written into the active profile (default: "default").
Log in to a second org under a separate name with --profile:

  lumo login --profile clientb --org clientb-corp

and switch between them with 'lumo profile use <name>' (or per-command
via --profile / LUMO_PROFILE).

Scopes: the token carries exactly what you ask for. By default that is
openid profile email + the blanket 'admin' scope, which is what every
'lumo' resource command needs. For a least-privilege session request
resource scopes instead, for example:

  lumo login --org acme-corp --scope openid,admin:users:read,admin:audit:read

The well-known first-party client 'lumoauth-cli' is auto-provisioned
on the server the first time you log in to an organization. If your
administrator narrowed that client's allowed scopes, requesting more
fails with invalid_scope.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear the active profile's stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := config.ClearCredentials()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Logged out. Profile %q removed from %s.\n", name, config.CredentialsPath())
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show which credential the CLI will use, and what it can reach",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getConfig()
		if err != nil {
			return err
		}
		p := getPrinter()
		profile := config.ActiveProfileName()
		creds, err := config.LoadCredentials()
		if err != nil {
			return err
		}

		info := map[string]interface{}{
			"profile":  profile,
			"org":      cfg.OrgID,
			"base_url": cfg.BaseURL,
		}
		cfg.InheritFromProfile(creds)
		info["org"] = cfg.OrgID
		info["base_url"] = cfg.BaseURL
		info["tls_verify"] = !cfg.Insecure

		switch {
		case creds.HasToken() && !creds.IsExpired():
			info["auth"] = "token"
			info["expires_at"] = creds.ExpiresAt.Format(time.RFC3339)
			if creds.UserEmail != "" {
				info["user"] = creds.UserEmail
			}
			info["scopes"] = creds.Scopes
			info["admin_api"] = creds.HasAdminAccess()
		case cfg.APIKey != "":
			info["auth"] = "api-key"
			info["api_key"] = maskSecret(cfg.APIKey)
			info["admin_api"] = true
		case creds.HasToken():
			info["auth"] = "expired"
			info["expires_at"] = creds.ExpiresAt.Format(time.RFC3339)
		default:
			info["auth"] = "none"
		}

		if p.IsJSON() {
			p.PrintResult(info)
		} else {
			fmt.Printf("Profile:    %s\n", profile)
			fmt.Printf("Org:        %s\n", info["org"])
			fmt.Printf("Base URL:   %s\n", info["base_url"])
			if cfg.Insecure {
				fmt.Println("TLS:        verification off (local dev)")
			}
			switch info["auth"] {
			case "token":
				fmt.Printf("Auth:       login token (expires %s)\n", info["expires_at"])
				if creds.UserEmail != "" {
					fmt.Printf("User:       %s\n", creds.UserEmail)
				}
				if len(creds.Scopes) > 0 {
					fmt.Printf("Scopes:     %s\n", strings.Join(creds.Scopes, " "))
				}
				if !creds.HasAdminAccess() {
					fmt.Println("Admin API:  no — the token has no admin scope; run 'lumo login' again or use an API key")
				}
			case "api-key":
				fmt.Printf("Auth:       API key %s (scopes are enforced per resource by the server)\n", info["api_key"])
			case "expired":
				fmt.Printf("Auth:       login expired at %s — run 'lumo login'\n", info["expires_at"])
			default:
				fmt.Println("Auth:       none — run 'lumo login' or set LUMO_API_KEY")
			}
		}
		if info["auth"] == "expired" || info["auth"] == "none" {
			os.Exit(ExitAuth)
		}
		return nil
	},
}

// maskSecret keeps a recognisable prefix/suffix of a credential for display.
func maskSecret(v string) string {
	if len(v) <= 12 {
		return "••••"
	}
	return v[:8] + "…" + v[len(v)-4:]
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Resolve the deployment URL. Mirrors the mobile app's region selector
	// (US / EU / Custom) when the user hasn't pinned a URL via --base-url,
	// LUMO_BASE_URL, or ~/.lumoauth/config.yaml. Custom covers self-hosted
	// and local-dev instances.
	baseURL, err := resolveLoginBaseURL()
	if err != nil {
		return err
	}

	orgID := loginOrgID
	if orgID == "" {
		orgID = strings.TrimSpace(promptString("Organization slug (e.g. acme-corp): "))
		if orgID == "" {
			return fmt.Errorf("organization slug is required")
		}
	}

	client := auth.New(baseURL, orgID, loginInsecure)

	fmt.Fprintf(os.Stderr, "→ Starting login on %s for org %s\n", baseURL, orgID)

	scopes := auth.DefaultScopes
	if len(loginScopes) > 0 {
		scopes = auth.SplitScopes(strings.Join(loginScopes, ","))
	}
	fmt.Fprintf(os.Stderr, "  Requesting scopes: %s\n", strings.Join(scopes, " "))

	dev, err := client.Start(scopes)
	if err != nil {
		if strings.Contains(err.Error(), "invalid_scope") {
			return fmt.Errorf("start device flow: %w\n  The organization's 'lumoauth-cli' client does not allow one of the requested scopes.\n  Ask an administrator to allow it (Applications → LumoAuth CLI → Scopes), or narrow the request, e.g.\n    lumo login --org %s --scope openid,profile,email", err, orgID)
		}
		return fmt.Errorf("start device flow: %w", err)
	}

	verifyURL := dev.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = dev.VerificationURI
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  ┌─ Open this URL in any browser to approve the login ─")
	fmt.Fprintf(os.Stderr, "  │  %s\n", verifyURL)
	fmt.Fprintf(os.Stderr, "  │  User code: %s\n", dev.UserCode)
	fmt.Fprintln(os.Stderr, "  └─")
	fmt.Fprintln(os.Stderr)

	// Auto-open is skipped for headless / SSH / no-DISPLAY sessions where
	// xdg-open would either fail silently or fire into the void. The URL is
	// already printed above, so the user can copy it into a local browser.
	switch {
	case loginNoBrowse:
		// Explicit opt-out — nothing to do.
	case isHeadlessSession():
		fmt.Fprintln(os.Stderr, "  (headless session detected — copy the URL above into a browser on your local machine)")
	default:
		if err := openBrowser(verifyURL); err == nil {
			fmt.Fprintln(os.Stderr, "  (opened in your browser; or copy the URL above)")
		} else {
			fmt.Fprintln(os.Stderr, "  (couldn't open browser automatically — copy the URL above)")
		}
	}

	fmt.Fprintln(os.Stderr, "Waiting for approval (Ctrl+C to cancel)...")

	interval := time.Duration(dev.Interval) * time.Second
	if interval < 1*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dev.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("device code expired before user approved")
		}
		time.Sleep(interval)

		result, err := client.Poll(dev.DeviceCode)
		if err != nil {
			return fmt.Errorf("poll: %w", err)
		}
		if result.Error == "slow_down" {
			interval += 5 * time.Second
			continue
		}
		if result.Error != "" {
			return fmt.Errorf("login failed: %s", result.Error)
		}
		if result.Pending {
			continue
		}

		// Success — persist tokens into the active profile (selectable via
		// --profile / LUMO_PROFILE) and make it the current profile so the
		// org just logged in to is what subsequent commands target.
		profile := config.ActiveProfileName()
		if err := config.ValidateProfileName(profile); err != nil {
			return err
		}
		creds := &config.Credentials{BaseURL: baseURL, OrgID: orgID, Insecure: loginInsecure}
		creds.ApplyToken(result.Token)
		// Best effort: record who signed in so whoami/doctor can say so.
		if email := fetchIdentityEmail(client, creds.AccessToken); email != "" {
			creds.UserEmail = email
		}
		if len(creds.Scopes) == 0 {
			// Servers that omit `scope` on success granted what was requested.
			creds.Scopes = scopes
		}
		if err := config.SaveProfile(profile, creds, true); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}

		fmt.Fprintf(os.Stderr, "✓ Logged in to %s. Credentials saved to profile %q in %s\n", orgID, profile, config.CredentialsPath())
		if creds.UserEmail != "" {
			fmt.Fprintf(os.Stderr, "  Signed in as:   %s\n", creds.UserEmail)
		}
		fmt.Fprintf(os.Stderr, "  Granted scopes: %s\n", strings.Join(creds.Scopes, " "))
		if creds.Insecure {
			fmt.Fprintf(os.Stderr, "  TLS verification is off for %s in this profile (remembered; other servers are unaffected).\n", baseURL)
		}
		if !creds.HasAdminAccess() {
			fmt.Fprintln(os.Stderr, "  ! No admin scope was granted, so 'lumo' resource commands will be refused (403).")
			fmt.Fprintln(os.Stderr, "    Ask an administrator to allow the 'admin' scope on the 'lumoauth-cli' client, or use an API key.")
		}
		fmt.Fprintln(os.Stderr, "  Next: lumo doctor")
		return nil
	}
}

// fetchIdentityEmail asks /me who the token belongs to. Errors are ignored:
// the login already succeeded and the email is only for display.
func fetchIdentityEmail(c *auth.Client, accessToken string) string {
	req, err := http.NewRequest("GET", c.BaseURL+"/orgs/"+c.OrgID+"/api/v1/me", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var me struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(resp.Body).Decode(&me) != nil {
		return ""
	}
	return me.Email
}

// stdinReader is shared across promptString calls so bufio's read-ahead
// doesn't drop input queued behind the line currently being consumed.
var stdinReader = bufio.NewReader(os.Stdin)

// promptString reads a single line from stdin with a prompt.
func promptString(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	line, _ := stdinReader.ReadString('\n')
	return line
}

// resolveLoginBaseURL decides which LumoAuth deployment to log in against.
//
// Precedence (highest first):
//  1. --base-url flag
//  2. LUMO_BASE_URL env var
//  3. base_url in ~/.lumoauth/config.yaml
//  4. Interactive region selector (US / EU / Custom)
//
// The selector is what end-users on production binaries see on first run;
// developers running against localhost typically pass --base-url or pre-seed
// the config file via `lumo config init`.
func resolveLoginBaseURL() (string, error) {
	if loginBaseURL != "" {
		return strings.TrimSuffix(loginBaseURL, "/"), nil
	}
	if v := strings.TrimSpace(os.Getenv("LUMO_BASE_URL")); v != "" {
		return strings.TrimSuffix(v, "/"), nil
	}
	if cfg, _ := config.LoadFromFile(); cfg != nil && strings.TrimSpace(cfg.BaseURL) != "" {
		return strings.TrimSuffix(cfg.BaseURL, "/"), nil
	}
	return promptRegion()
}

// promptRegion runs the interactive deployment-selection prompt and returns
// the chosen base URL. Sets loginInsecure when the user enters a custom
// http:// or localhost URL — those almost always mean a self-signed local
// dev cert and we'd otherwise fail the TLS handshake.
func promptRegion() (string, error) {
	fmt.Fprintln(os.Stderr, "Select region:")
	for i, r := range config.Regions {
		marker := " "
		if i == 0 {
			marker = "*"
		}
		fmt.Fprintf(os.Stderr, "  %s %d) %-20s  %s\n", marker, i+1, r.Label, r.Description)
	}
	choice := strings.TrimSpace(promptString("Region [1]: "))
	if choice == "" {
		choice = "1"
	}

	var picked config.Region
	switch choice {
	case "1", "us", "US":
		picked = config.RegionUS
	case "2", "eu", "EU":
		picked = config.RegionEU
	case "3", "custom", "other":
		picked = config.RegionCustom
	default:
		return "", fmt.Errorf("invalid selection %q — pick 1, 2, or 3", choice)
	}

	if picked.ID != "custom" {
		return picked.ServerURL, nil
	}

	url := strings.TrimSpace(promptString("Server URL (e.g. https://localhost:8000): "))
	if url == "" {
		return "", fmt.Errorf("server URL is required for the Custom option")
	}
	url = strings.TrimSuffix(url, "/")
	if config.IsLocalURL(url) && !loginInsecure {
		fmt.Fprintln(os.Stderr, "  (local or private-network URL detected — skipping TLS verification for this server)")
		loginInsecure = true
	}
	return url, nil
}

// openBrowser tries to open url in the user's default browser. Best-effort —
// if it can't, the caller falls back to printing the URL.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// isHeadlessSession reports whether the CLI is running somewhere a browser
// can't be opened — typically a remote SSH session or a Linux box with no
// graphical display. Without this guard, `xdg-open` would `Start()` cleanly
// and silently send the URL to a browser the user can't see.
func isHeadlessSession() bool {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return true
	}
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return true
	}
	return false
}
