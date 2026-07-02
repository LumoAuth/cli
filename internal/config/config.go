package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir  = ".lumoauth"
	DefaultConfigFile = "config.yaml"
)

// Config holds all CLI configuration.
type Config struct {
	APIKey   string `yaml:"api_key" json:"api_key"`
	OrgID    string `yaml:"org_id" json:"org_id"`
	BaseURL  string `yaml:"base_url" json:"base_url"`
	Format   string `yaml:"format" json:"format"`
	Insecure bool   `yaml:"insecure" json:"insecure"`
}

// Load resolves configuration with precedence: flags > env > file.
// Flag values are passed in as overrides (empty string = not set).
func Load(flagAPIKey, flagOrgID, flagBaseURL, flagFormat string, flagInsecure bool) (*Config, error) {
	cfg := &Config{}

	// 1. Load from config file (lowest priority)
	if err := cfg.loadFile(); err != nil {
		// Config file is optional; only error on parse failures
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("config file error: %w", err)
		}
	}

	// 2. Override with environment variables
	if v := os.Getenv("LUMO_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("LUMO_ORG_ID"); v != "" {
		cfg.OrgID = v
	}
	if v := os.Getenv("LUMO_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("LUMO_OUTPUT_FORMAT"); v != "" {
		cfg.Format = v
	}
	if os.Getenv("LUMO_INSECURE") == "true" || os.Getenv("LUMO_INSECURE") == "1" {
		cfg.Insecure = true
	}

	// 3. Override with CLI flags (highest priority)
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if flagOrgID != "" {
		cfg.OrgID = flagOrgID
	}
	if flagBaseURL != "" {
		cfg.BaseURL = flagBaseURL
	}
	if flagFormat != "" {
		cfg.Format = flagFormat
	}
	if flagInsecure {
		cfg.Insecure = true
	}

	// Defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://app.lumoauth.dev"
	}
	if cfg.Format == "" {
		cfg.Format = "table"
	}

	return cfg, nil
}

// Validate checks that required fields are set.
//
// Auth is satisfied EITHER by stored device-flow credentials (`lumo login`)
// or by an API key (legacy / scriptable path). The credentials file is the
// preferred source whenever it's present and unexpired — see `client.New`
// for the matching request-time precedence.
//
// We still allow a configured API key to satisfy auth on its own, because
// users in scripts may not have run `lumo login` at all. But we always
// consult the credentials file to fill in OrgID / BaseURL when the user
// hasn't pinned them, so freshly-logged-in users don't have to repeat
// `--org-id` on every command.
func (c *Config) Validate() error {
	creds, _ := LoadCredentials()
	// Silently refresh when the access token has expired but a refresh
	// token is on disk. The CLI's device-flow access tokens are only valid
	// for an hour, so without this every command after the first hour fails
	// even though the user's 30-day refresh token is perfectly usable.
	// Failure here is not fatal — we fall through to the API-key path or
	// the helpful error below.
	if creds.HasToken() && creds.IsExpired() {
		_ = creds.EnsureFresh(c.Insecure)
	}
	// Inherit org/base URL from the active profile whenever it pins them —
	// even token-less profiles (created via `lumo profile create`) provide
	// this context so an API key + profile combination works without --org-id.
	if creds != nil {
		if c.OrgID == "" {
			c.OrgID = creds.OrgID
		}
		if c.BaseURL == "" || c.BaseURL == "https://app.lumoauth.dev" {
			if creds.BaseURL != "" {
				c.BaseURL = creds.BaseURL
			}
		}
	}
	if !(creds.HasToken() && !creds.IsExpired()) && c.APIKey == "" {
		baseURL := c.BaseURL
		orgID := c.OrgID
		if creds != nil {
			if baseURL == "" {
				baseURL = creds.BaseURL
			}
			if orgID == "" {
				orgID = creds.OrgID
			}
		}
		apiKeysURL := apiKeysSettingsURL(baseURL, orgID)
		if creds.HasToken() && creds.IsExpired() {
			return fmt.Errorf(
				"stored credentials have expired and refresh failed.\n"+
					"  Re-authenticate with 'lumo login', or switch to a long-lived API key:\n"+
					"    1. Generate one at %s\n"+
					"    2. lumo config set api_key <key>   (or export LUMO_API_KEY=<key>)",
				apiKeysURL,
			)
		}
		return fmt.Errorf(
			"not authenticated.\n"+
				"  Run 'lumo login' for interactive use, or use a long-lived API key for scripts:\n"+
				"    1. Generate one at %s\n"+
				"    2. lumo config set api_key <key>   (or export LUMO_API_KEY=<key>)",
			apiKeysURL,
		)
	}

	if c.OrgID == "" {
		return fmt.Errorf("organization ID is required. Set via --org-id flag, LUMO_ORG_ID env var, or 'lumo config init'")
	}
	return nil
}

// apiKeysSettingsURL builds the dashboard URL the user can visit to mint
// a long-lived API key. Uses placeholders when the parts aren't known yet
// so the message is still actionable.
func apiKeysSettingsURL(baseURL, orgID string) string {
	if baseURL == "" {
		baseURL = "<base-url>"
	} else {
		baseURL = strings.TrimRight(baseURL, "/")
	}
	if orgID == "" {
		orgID = "<org>"
	}
	return fmt.Sprintf("%s/orgs/%s/portal/settings/api-keys", baseURL, orgID)
}

// ConfigDir returns the config directory path.
func ConfigDir() string {
	if v := os.Getenv("LUMO_CONFIG_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfigDir
	}
	return filepath.Join(home, DefaultConfigDir)
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), DefaultConfigFile)
}

func (c *Config) loadFile() error {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

// LoadFromFile reads the config file without applying defaults or env/flag
// overrides. Callers (e.g. `lumo login`) use this to detect whether the user
// has explicitly configured a base URL, vs. relying on the built-in default.
// Returns nil, nil when no file exists.
func LoadFromFile() (*Config, error) {
	cfg := &Config{}
	if err := cfg.loadFile(); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return cfg, nil
}

// Save writes the config to the config file.
func (c *Config) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := ConfigPath()
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
