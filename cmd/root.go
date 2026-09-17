package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/lumoauth/cli/internal/client"
	"github.com/lumoauth/cli/internal/config"
	"github.com/lumoauth/cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagAPIKey   string
	flagOrgID    string
	flagBaseURL  string
	flagFormat   string
	flagProfile  string
	flagInsecure bool
	flagQuiet    bool
	flagVerbose  bool
)

// Exit codes, stable for scripts and CI:
//
//	0 success · 1 general failure · 2 authentication/authorization
//	3 not found · 4 invalid input (400/409/422) · 5 rate limited
const (
	ExitOK        = 0
	ExitGeneral   = 1
	ExitAuth      = 2
	ExitNotFound  = 3
	ExitInput     = 4
	ExitRateLimit = 5
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "lumo",
	Short: "LumoAuth CLI — manage your organization's identity infrastructure",
	Long: `LumoAuth CLI manages one organization at a time: users, roles, groups,
OAuth apps, AI agents, webhooks, audit logs, permissions, settings, sessions.

Get started:
  lumo login --org acme-corp      browser sign-in (OAuth 2.0 device flow)
  lumo doctor                     check credentials, scopes and connectivity
  lumo users list                 any resource command; -o json for scripts

Credentials (both are stateless: no cookie, sent on every call):
  • lumo login      a per-user token that carries the 'admin' scope by default;
                    narrow it with --scope for least privilege.
  • API key         LUMO_API_KEY / --api-key / 'lumo config set api_key' for
                    CI and scripts. Keys are checked per resource: a key with
                    admin:users:read can list users and nothing else.

Raw access:
  lumo api GET /users             paths are relative to the Admin API
  lumo api GET /orgs/acme/api/v1/me   absolute API paths work too

Exit codes: 0 ok · 1 error · 2 auth · 3 not found · 4 invalid input · 5 rate limited`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// PersistentPreRun runs after flag parsing for every subcommand — the
	// earliest point where the --profile flag value is known. The config
	// package resolves the active profile as flag > LUMO_PROFILE env >
	// current_profile in ~/.lumoauth/credentials.yaml.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.SetProfileOverride(flagProfile)
		client.SetUserAgent("lumo-cli/" + Version)
	},
}

// Execute runs the root command.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}
	p := getPrinter()
	hint := errorHint(err)
	if p.IsJSON() {
		p.PrintFailure(failureFromError(err, hint))
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		if hint != "" {
			fmt.Fprintf(os.Stderr, "Hint:  %s\n", hint)
		}
	}
	os.Exit(exitCode(err))
}

// errorHint returns the actionable next step for an error, if any.
func errorHint(err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Hint()
	}
	return ""
}

func failureFromError(err error, hint string) output.Failure {
	f := output.Failure{Message: err.Error(), Hint: hint}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		f.Status = apiErr.StatusCode
		f.Code = apiErr.Code
		f.Details = apiErr.Details
	}
	return f
}

func exitCode(err error) int {
	if errors.Is(err, config.ErrNotAuthenticated) {
		return ExitAuth
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return ExitGeneral
	}
	switch {
	case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
		return ExitAuth
	case apiErr.StatusCode == 404:
		return ExitNotFound
	case apiErr.StatusCode == 400 || apiErr.StatusCode == 409 || apiErr.StatusCode == 422:
		return ExitInput
	case apiErr.StatusCode == 429:
		return ExitRateLimit
	}
	return ExitGeneral
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagAPIKey, "api-key", "", "Organization API key lmk_… (overrides LUMO_API_KEY)")
	pf.StringVar(&flagOrgID, "org", "", "Organization slug (overrides LUMO_ORG and the active profile)")
	// Kept for existing scripts; --org is the documented spelling.
	pf.StringVar(&flagOrgID, "org-id", "", "Alias of --org")
	_ = pf.MarkHidden("org-id")
	pf.StringVar(&flagBaseURL, "base-url", "", "Base URL (overrides LUMO_BASE_URL)")
	pf.StringVar(&flagProfile, "profile", "", "Named credentials profile (overrides LUMO_PROFILE and current_profile)")
	pf.StringVarP(&flagFormat, "output", "o", "", "Output format: table, json, yaml (default: table; json when piped)")
	pf.BoolVar(&flagInsecure, "insecure", false, "Skip TLS certificate verification (local dev only)")
	pf.BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
}

// getConfig loads and validates the configuration.
func getConfig() (*config.Config, error) {
	return config.Load(flagAPIKey, flagOrgID, flagBaseURL, flagFormat, flagInsecure)
}

// getConfigValidated loads config and validates required fields.
func getConfigValidated() (*config.Config, error) {
	cfg, err := getConfig()
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// getClient creates an authenticated API client.
func getClient() (*client.Client, error) {
	cfg, err := getConfigValidated()
	if err != nil {
		return nil, err
	}
	return client.New(cfg), nil
}

// getPrinter returns an output printer for the current format settings.
func getPrinter() *output.Printer {
	format := flagFormat
	if format == "" {
		if v := os.Getenv("LUMO_OUTPUT_FORMAT"); v != "" {
			format = v
		}
	}
	return output.NewPrinter(format, flagQuiet)
}
