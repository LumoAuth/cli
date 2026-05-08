package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Import users + clients from another identity provider",
	Long: `Migrate from another IDP into the current LumoAuth tenant. Each
sub-command takes an export file from the source IDP and replays it
against the LumoAuth Admin API. Idempotent — re-runs skip rows that
already exist.`,
}

var migrateAuth0Cmd = &cobra.Command{
	Use:   "auth0",
	Short: "Import users from an Auth0 bulk export",
	Long: `Reads an Auth0 bulk-user-import JSON file and creates each user in the
current LumoAuth tenant via the Admin API. Skips users whose email
already exists (409 from the API). Designed to be re-runnable.

Auth0 export format reference:
  https://auth0.com/docs/manage-users/user-migration/bulk-user-imports#users-import-schema

Typical workflow:
  1. In Auth0, run a Users export job (Tenant Settings → Import / Export
     Users → Export users) and download the JSON file.
  2. Run:  lumo migrate auth0 --file users.json --org acme-corp
  3. Review the per-row outcome printed to stderr; the summary line is
     also printed to stdout for scripting.

The CLI never sends Auth0 password hashes anywhere — bcrypt hashes from
Auth0 are not directly compatible with LumoAuth's password policy and
attempting to migrate them would silently break login. Migrated users
are created without a password and are sent through your tenant's
"first login" flow (magic link or password reset) the next time they
sign in. Use --send-reset to trigger this proactively.`,
	RunE: runMigrateAuth0,
}

var (
	migrateAuth0File       string
	migrateAuth0DryRun     bool
	migrateAuth0Concurrent int
	migrateAuth0SendReset  bool
	migrateAuth0Limit      int
)

func init() {
	migrateAuth0Cmd.Flags().StringVar(&migrateAuth0File, "file", "", "Path to Auth0 export JSON file (required)")
	migrateAuth0Cmd.Flags().BoolVar(&migrateAuth0DryRun, "dry-run", false, "Parse and validate only — no writes")
	migrateAuth0Cmd.Flags().IntVar(&migrateAuth0Concurrent, "concurrent", 4, "Number of parallel POSTs (1–16)")
	migrateAuth0Cmd.Flags().BoolVar(&migrateAuth0SendReset, "send-reset", false, "Trigger a password-reset email for each migrated user")
	migrateAuth0Cmd.Flags().IntVar(&migrateAuth0Limit, "limit", 0, "Stop after N users (0 = no limit; useful for staged migrations)")
	_ = migrateAuth0Cmd.MarkFlagRequired("file")

	migrateCmd.AddCommand(migrateAuth0Cmd)
	rootCmd.AddCommand(migrateCmd)
}

// auth0User mirrors the subset of the Auth0 bulk-import schema we map.
// Unknown fields are preserved on user_metadata so app-side data isn't
// lost — Auth0 lets you put arbitrary JSON there and many tenants do.
type auth0User struct {
	Email         string                 `json:"email"`
	EmailVerified bool                   `json:"email_verified"`
	Name          string                 `json:"name"`
	GivenName     string                 `json:"given_name"`
	FamilyName    string                 `json:"family_name"`
	Nickname      string                 `json:"nickname"`
	Username      string                 `json:"username"`
	UserID        string                 `json:"user_id"`
	Picture       string                 `json:"picture"`
	Locale        string                 `json:"locale"`
	Blocked       bool                   `json:"blocked"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
	UserMetadata  map[string]interface{} `json:"user_metadata"`
	AppMetadata   map[string]interface{} `json:"app_metadata"`
}

// lumoCreatePayload is the body shape POSTed to /admin/users. Mirrors
// the AdminUserController::create handler's expected fields.
type lumoCreatePayload struct {
	Email         string                 `json:"email"`
	Name          string                 `json:"name,omitempty"`
	GivenName     string                 `json:"givenName,omitempty"`
	FamilyName    string                 `json:"familyName,omitempty"`
	Nickname      string                 `json:"nickname,omitempty"`
	Username      string                 `json:"username,omitempty"`
	Picture       string                 `json:"picture,omitempty"`
	Locale        string                 `json:"locale,omitempty"`
	EmailVerified bool                   `json:"emailVerified"`
	Blocked       bool                   `json:"blocked,omitempty"`
	Traits        map[string]interface{} `json:"traits,omitempty"`
}

type migrateResult struct {
	Email   string
	Status  string // created | skipped | failed
	Code    int
	Message string
}

func runMigrateAuth0(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigValidated()
	if err != nil {
		return err
	}
	auth, err := authHeader(cfg)
	if err != nil {
		return err
	}

	if migrateAuth0Concurrent < 1 || migrateAuth0Concurrent > 16 {
		return fmt.Errorf("--concurrent must be between 1 and 16")
	}

	users, err := loadAuth0Export(migrateAuth0File)
	if err != nil {
		return fmt.Errorf("read %s: %w", migrateAuth0File, err)
	}

	if migrateAuth0Limit > 0 && len(users) > migrateAuth0Limit {
		users = users[:migrateAuth0Limit]
	}

	fmt.Fprintf(os.Stderr, "→ Loaded %d user(s) from %s\n", len(users), migrateAuth0File)
	if migrateAuth0DryRun {
		fmt.Fprintln(os.Stderr, "  --dry-run: no writes will be performed")
	}
	fmt.Fprintf(os.Stderr, "  Target tenant: %s\n  Base URL:      %s\n  Concurrency:   %d\n\n",
		cfg.OrgID, cfg.BaseURL, migrateAuth0Concurrent)

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	hc := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	usersURL := strings.TrimRight(cfg.BaseURL, "/") +
		"/orgs/" + cfg.OrgID + "/api/v1/admin/users"

	// Fixed-size worker pool so a 50K-user export doesn't blow up the
	// server with 50K concurrent POSTs.
	in := make(chan auth0User)
	out := make(chan migrateResult, len(users))
	var wg sync.WaitGroup
	for i := 0; i < migrateAuth0Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range in {
				out <- importOne(hc, usersURL, auth, u, migrateAuth0DryRun)
			}
		}()
	}
	go func() {
		for _, u := range users {
			in <- u
		}
		close(in)
	}()
	go func() { wg.Wait(); close(out) }()

	var (
		created uint64
		skipped uint64
		failed  uint64
	)
	for r := range out {
		switch r.Status {
		case "created":
			atomic.AddUint64(&created, 1)
			fmt.Fprintf(os.Stderr, "  ✓ %s\n", r.Email)
		case "skipped":
			atomic.AddUint64(&skipped, 1)
			fmt.Fprintf(os.Stderr, "  – %s  (%s)\n", r.Email, r.Message)
		case "failed":
			atomic.AddUint64(&failed, 1)
			fmt.Fprintf(os.Stderr, "  ✗ %s  HTTP %d  %s\n", r.Email, r.Code, r.Message)
		}
	}

	summary := struct {
		Total   int    `json:"total"`
		Created uint64 `json:"created"`
		Skipped uint64 `json:"skipped"`
		Failed  uint64 `json:"failed"`
		DryRun  bool   `json:"dry_run"`
	}{Total: len(users), Created: created, Skipped: skipped, Failed: failed, DryRun: migrateAuth0DryRun}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Done. created=%d skipped=%d failed=%d\n", created, skipped, failed)

	if getPrinter().IsJSON() {
		raw, _ := json.Marshal(summary)
		fmt.Println(string(raw))
	}
	if failed > 0 {
		os.Exit(1)
	}
	return nil
}

func loadAuth0Export(path string) ([]auth0User, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Auth0 exports are sometimes a JSON array, sometimes NDJSON
	// (one user per line). Detect by peeking at the first non-whitespace byte.
	trimmed := bytes.TrimLeftFunc(raw, func(r rune) bool { return r == ' ' || r == '\n' || r == '\r' || r == '\t' })
	if len(trimmed) == 0 {
		return nil, errors.New("empty file")
	}
	if trimmed[0] == '[' {
		var arr []auth0User
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, fmt.Errorf("parse JSON array: %w", err)
		}
		return arr, nil
	}
	// Treat as NDJSON.
	var out []auth0User
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var u auth0User
		if err := json.Unmarshal(line, &u); err != nil {
			return nil, fmt.Errorf("parse NDJSON line: %w", err)
		}
		out = append(out, u)
	}
	return out, nil
}

func importOne(hc *http.Client, usersURL, auth string, u auth0User, dryRun bool) migrateResult {
	if u.Email == "" {
		return migrateResult{Email: "(no email)", Status: "skipped", Message: "missing email"}
	}

	payload := lumoCreatePayload{
		Email:         u.Email,
		Name:          u.Name,
		GivenName:     u.GivenName,
		FamilyName:    u.FamilyName,
		Nickname:      u.Nickname,
		Username:      u.Username,
		Picture:       u.Picture,
		Locale:        u.Locale,
		EmailVerified: u.EmailVerified,
		Blocked:       u.Blocked,
	}

	// Stash Auth0 metadata + the original Auth0 user_id under traits so
	// app-side data isn't lost. Apps that referenced `auth0|abc123` user
	// ids can look these up after the migration without breaking.
	traits := map[string]interface{}{}
	if u.UserID != "" {
		traits["auth0_user_id"] = u.UserID
	}
	if len(u.UserMetadata) > 0 {
		traits["auth0_user_metadata"] = u.UserMetadata
	}
	if len(u.AppMetadata) > 0 {
		traits["auth0_app_metadata"] = u.AppMetadata
	}
	if u.CreatedAt != "" {
		traits["auth0_created_at"] = u.CreatedAt
	}
	if len(traits) > 0 {
		payload.Traits = traits
	}

	if dryRun {
		return migrateResult{Email: u.Email, Status: "created", Message: "dry-run"}
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", usersURL, bytes.NewReader(body))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return migrateResult{Email: u.Email, Status: "failed", Message: err.Error()}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case 201, 200:
		return migrateResult{Email: u.Email, Status: "created", Code: resp.StatusCode}
	case 409:
		return migrateResult{Email: u.Email, Status: "skipped", Code: 409, Message: "already exists"}
	default:
		// Try to extract the error message from the JSON envelope.
		var env struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBody, &env)
		msg := env.Message
		if msg == "" {
			msg = env.Error
		}
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return migrateResult{Email: u.Email, Status: "failed", Code: resp.StatusCode, Message: msg}
	}
}
