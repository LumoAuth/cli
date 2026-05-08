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
	cfg, _ := config.Load("", "", loginBaseURL, "", loginInsecure)
	baseURL := cfg.BaseURL

	orgID := loginOrgID
	if orgID == "" {
		orgID = strings.TrimSpace(promptString(
			fmt.Sprintf("Organization slug (e.g. acme-corp): "),
		))
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
	fmt.Fprintf(os.Stderr, "  Verification URL:  %s\n", verifyURL)
	fmt.Fprintf(os.Stderr, "  User code:         %s\n", dev.UserCode)
	fmt.Fprintln(os.Stderr)

	if !loginNoBrowse {
		if err := openBrowser(verifyURL); err == nil {
			fmt.Fprintln(os.Stderr, "  (opened in your browser; or visit the URL above)")
		} else {
			fmt.Fprintln(os.Stderr, "  (couldn't open browser automatically — visit the URL above)")
		}
	}

	fmt.Fprintln(os.Stderr, "Waiting for approval...")

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

// promptString reads a single line from stdin with a prompt.
func promptString(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return line
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
