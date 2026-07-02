package cmd

import (
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

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "lumo",
	Short: "LumoAuth CLI — manage your organization's identity infrastructure",
	Long: `LumoAuth CLI provides comprehensive management of your LumoAuth organization.

Manage users, roles, groups, OAuth clients, agents, webhooks,
audit logs, permissions, settings, sessions, and more.

Authentication:
  Set your API key via --api-key, LUMO_API_KEY env var, or 'lumo config init'.
  API keys are created at /orgs/<orgId>/portal/settings/api-keys.

AI Agent Integration:
  Use -o json for structured output, or pipe to get auto-JSON.
  Use 'lumo api' for raw API access to any endpoint.

Examples:
  lumo users list --search "john"
  lumo roles create --name "Editor"
  lumo audit-logs list --limit 50
  lumo settings get auth
  lumo api GET /orgs/<orgId>/api/v1/admin/users`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// PersistentPreRun runs after flag parsing for every subcommand — the
	// earliest point where the --profile flag value is known. The config
	// package resolves the active profile as flag > LUMO_PROFILE env >
	// current_profile in ~/.lumoauth/credentials.yaml.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.SetProfileOverride(flagProfile)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		p := getPrinter()
		if p.IsJSON() {
			p.PrintError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}

		// Structured exit codes
		switch e := err.(type) {
		case *client.APIError:
			switch {
			case e.StatusCode == 401 || e.StatusCode == 403:
				os.Exit(2) // auth error
			case e.StatusCode == 404:
				os.Exit(3) // not found
			default:
				os.Exit(1)
			}
		default:
			os.Exit(1)
		}
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "LumoAuth API key (overrides LUMO_API_KEY)")
	rootCmd.PersistentFlags().StringVar(&flagOrgID, "org-id", "", "Organization ID (overrides LUMO_ORG_ID)")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "", "Base URL (overrides LUMO_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Named credentials profile (overrides LUMO_PROFILE and current_profile)")
	rootCmd.PersistentFlags().StringVarP(&flagFormat, "output", "o", "", "Output format: table, json, yaml (default: table)")
	rootCmd.PersistentFlags().BoolVar(&flagInsecure, "insecure", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
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
