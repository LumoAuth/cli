package cmd

import (
	"bufio"
	"fmt"
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
)

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)

	loginCmd.Flags().StringVar(&loginOrgID, "org", "", "Organization slug (prompted if not set)")
	loginCmd.Flags().StringVar(&loginBaseURL, "base-url", "", "Override the LumoAuth base URL")
	loginCmd.Flags().BoolVar(&loginInsecure, "insecure", false, "Skip TLS verification (dev only)")
	loginCmd.Flags().BoolVar(&loginNoBrowse, "no-browser", false, "Do not auto-open the verification URL")
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to LumoAuth via browser (OAuth 2.0 device flow)",
	Long: `Authenticate this CLI against a LumoAuth tenant using the OAuth 2.0
Device Authorization Grant (RFC 8628). No API key required — the CLI
opens a browser to the verification URL, you confirm in the dashboard,
and credentials are stored at ~/.lumoauth/credentials.yaml.

The well-known first-party client 'lumoauth-cli' is auto-provisioned
on the server the first time you log in to a tenant.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.ClearCredentials(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Logged out. ~/.lumoauth/credentials.yaml removed.")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Print the currently authenticated principal",
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := config.LoadCredentials()
		if err != nil {
			return err
		}
		if creds == nil {
			fmt.Fprintln(os.Stderr, "Not logged in. Run 'lumo login' to sign in.")
			os.Exit(2)
		}
		if creds.IsExpired() {
			fmt.Fprintln(os.Stderr, "Credentials expired. Run 'lumo login' to renew.")
			os.Exit(2)
		}
		fmt.Printf("Org:        %s\n", creds.OrgID)
		fmt.Printf("Base URL:   %s\n", creds.BaseURL)
		if creds.UserEmail != "" {
			fmt.Printf("User:       %s\n", creds.UserEmail)
		}
		fmt.Printf("Expires at: %s\n", creds.ExpiresAt.Format(time.RFC3339))
		return nil
	},
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

	dev, err := client.Start()
	if err != nil {
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

		// Success — persist tokens.
		expiresAt := time.Now().Add(time.Duration(result.Token.ExpiresIn) * time.Second)
		creds := &config.Credentials{
			BaseURL:      baseURL,
			OrgID:        orgID,
			AccessToken:  result.Token.AccessToken,
			RefreshToken: result.Token.RefreshToken,
			ExpiresAt:    expiresAt,
			TokenType:    result.Token.TokenType,
		}
		if err := creds.Save(); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}

		fmt.Fprintf(os.Stderr, "✓ Logged in to %s. Credentials saved at %s\n", orgID, config.CredentialsPath())
		return nil
	}
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
		fmt.Fprintln(os.Stderr, "  (local URL detected — skipping TLS verification)")
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
