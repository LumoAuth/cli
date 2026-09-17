package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/client"
	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
)

// doctorCheck is one line of `lumo doctor` output.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail | skip
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose credentials, scopes and connectivity for the current org",
	Long: `Run the checks a new user usually has to discover by trial and error:

  1. Which profile, organization and server the CLI will talk to
  2. Which credential it will send (login token or API key) and whether it
     is still valid — expired tokens are refreshed on the spot
  3. Whether that credential can reach the Admin API (scopes, permissions)
  4. Whether the server is reachable and the organization exists

Every failing check comes with the command that fixes it. Exit code is 1
when any check fails, so it can gate CI jobs.`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	p := getPrinter()
	var checks []doctorCheck
	add := func(c doctorCheck) { checks = append(checks, c) }

	// 1. Configuration ------------------------------------------------------
	cfg, err := getConfig()
	if err != nil {
		add(doctorCheck{Name: "config", Status: "fail", Detail: err.Error(), Hint: "Fix or delete " + config.ConfigPath()})
		return finishDoctor(p, checks)
	}
	profile := config.ActiveProfileName()
	creds, _ := config.LoadCredentials()
	if creds != nil {
		if cfg.OrgID == "" {
			cfg.OrgID = creds.OrgID
		}
		if (cfg.BaseURL == "" || cfg.BaseURL == config.DefaultBaseURL) && creds.BaseURL != "" {
			cfg.BaseURL = creds.BaseURL
		}
	}
	add(doctorCheck{Name: "profile", Status: "ok", Detail: fmt.Sprintf("%s (%s)", profile, config.CredentialsPath())})
	if cfg.OrgID == "" {
		add(doctorCheck{Name: "organization", Status: "fail", Detail: "no organization selected",
			Hint: "Run 'lumo login --org <slug>' or pass --org / set LUMO_ORG"})
		return finishDoctor(p, checks)
	}
	add(doctorCheck{Name: "organization", Status: "ok", Detail: cfg.OrgID})
	add(doctorCheck{Name: "server", Status: "ok", Detail: cfg.BaseURL})

	// 2. Credential ---------------------------------------------------------
	c := client.New(cfg)
	switch c.AuthMethod() {
	case client.AuthToken:
		cr := c.Credentials()
		detail := fmt.Sprintf("login token, expires %s", cr.ExpiresAt.Format(time.RFC3339))
		if cr.UserEmail != "" {
			detail = cr.UserEmail + ", " + detail
		}
		add(doctorCheck{Name: "credential", Status: "ok", Detail: detail})
		if len(cr.Scopes) == 0 {
			add(doctorCheck{Name: "scopes", Status: "warn", Detail: "unknown (logged in with an older CLI)",
				Hint: "Run 'lumo login' again to record the granted scopes"})
		} else if cr.HasAdminAccess() {
			add(doctorCheck{Name: "scopes", Status: "ok", Detail: strings.Join(cr.Scopes, " ")})
		} else {
			add(doctorCheck{Name: "scopes", Status: "fail", Detail: strings.Join(cr.Scopes, " "),
				Hint: "No admin scope: 'lumo' resource commands will get 403. Run 'lumo login' (requests 'admin' by default) or use an API key"})
		}
	case client.AuthAPIKey:
		add(doctorCheck{Name: "credential", Status: "ok", Detail: "API key " + maskSecret(cfg.APIKey) + " (scopes enforced per resource)"})
	default:
		hint := "Run 'lumo login --org " + cfg.OrgID + "', or set LUMO_API_KEY"
		detail := "none"
		if creds.HasToken() && creds.IsExpired() {
			detail = "login token expired and could not be refreshed"
		} else if creds != nil && creds.OrgID != "" && creds.OrgID != cfg.OrgID {
			detail = fmt.Sprintf("profile %q is logged in to %q, not %q", profile, creds.OrgID, cfg.OrgID)
			hint = "Run 'lumo login --org " + cfg.OrgID + "' or switch profiles with 'lumo profile use'"
		}
		add(doctorCheck{Name: "credential", Status: "fail", Detail: detail, Hint: hint})
	}

	// 3. Server reachability + organization ---------------------------------
	disc, err := c.Request("GET", "/orgs/"+cfg.OrgID+"/api/v1/.well-known/openid-configuration", nil, nil)
	if err != nil {
		var apiErr *client.APIError
		hint := "Check --base-url / LUMO_BASE_URL and that the server is up"
		detail := err.Error()
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			hint = "Organization slug not found on this server; check --org"
		} else if strings.Contains(strings.ToLower(detail), "certificate") || strings.Contains(strings.ToLower(detail), "tls") {
			hint = "TLS verification failed; for a local dev server pass --insecure"
		}
		add(doctorCheck{Name: "discovery", Status: "fail", Detail: detail, Hint: hint})
		return finishDoctor(p, checks)
	}
	var discovery struct {
		Issuer string `json:"issuer"`
	}
	_ = json.Unmarshal(disc, &discovery)
	add(doctorCheck{Name: "discovery", Status: "ok", Detail: "issuer " + discovery.Issuer})

	// 4. Admin API reach -----------------------------------------------------
	if c.AuthMethod() == client.AuthNone {
		add(doctorCheck{Name: "admin api", Status: "skip", Detail: "no credential"})
		return finishDoctor(p, checks)
	}
	if _, err := c.Get("/organization", nil); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			add(doctorCheck{Name: "admin api", Status: "fail", Detail: apiErr.Error(), Hint: apiErr.Hint()})
		} else {
			add(doctorCheck{Name: "admin api", Status: "fail", Detail: err.Error()})
		}
		return finishDoctor(p, checks)
	}
	add(doctorCheck{Name: "admin api", Status: "ok", Detail: "GET /admin/organization succeeded"})

	if c.AuthMethod() == client.AuthAPIKey {
		// A key that can read the organization has admin:settings:read; other
		// resources may still be refused — say so instead of guessing.
		add(doctorCheck{Name: "note", Status: "ok", Detail: "API keys are checked per resource; a 403 on another command names the missing scope"})
	}
	return finishDoctor(p, checks)
}

func finishDoctor(p interface {
	IsJSON() bool
	PrintResult(interface{})
}, checks []doctorCheck) error {
	failed := false
	for _, c := range checks {
		if c.Status == "fail" {
			failed = true
		}
	}
	if p.IsJSON() {
		p.PrintResult(map[string]interface{}{"ok": !failed, "checks": checks, "version": Version})
	} else {
		for _, c := range checks {
			icon := map[string]string{"ok": "✓", "warn": "!", "fail": "✗", "skip": "-"}[c.Status]
			fmt.Printf("%s %-13s %s\n", icon, c.Name, c.Detail)
			if c.Hint != "" {
				fmt.Printf("  → %s\n", c.Hint)
			}
		}
		fmt.Printf("\nlumo %s\n", Version)
	}
	if failed {
		os.Exit(ExitGeneral)
	}
	return nil
}
