package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir  = ".lumoauth"
	DefaultConfigFile = "config.yaml"
)

// Config holds all CLI configuration.
type Config struct {
	APIKey  string `yaml:"api_key" json:"api_key"`
	OrgID   string `yaml:"org_id" json:"org_id"`
	BaseURL string `yaml:"base_url" json:"base_url"`
	Format  string `yaml:"format" json:"format"`
	Insecure bool  `yaml:"insecure" json:"insecure"`
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
// Auth is satisfied EITHER by an API key (legacy / scriptable path) or by
// stored device-flow credentials (`lumo login`). When the API key is empty,
// we treat a non-expired credentials file at ~/.lumoauth/credentials.yaml as
// equivalent — the api client will use its bearer token instead.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		creds, _ := LoadCredentials()
		if creds == nil {
			return fmt.Errorf("not authenticated. Run 'lumo login', or set --api-key / LUMO_API_KEY for scripted use")
		}
		if creds.IsExpired() {
			return fmt.Errorf("stored credentials have expired. Run 'lumo login' to renew")
		}
		// Inherit org/base from credentials when not overridden — saves
		// callers from passing --org-id every time.
		if c.OrgID == "" {
			c.OrgID = creds.OrgID
		}
		if c.BaseURL == "" || c.BaseURL == "https://app.lumoauth.dev" {
			if creds.BaseURL != "" {
				c.BaseURL = creds.BaseURL
			}
		}
	}
	if c.OrgID == "" {
		return fmt.Errorf("organization ID is required. Set via --org-id flag, LUMO_ORG_ID env var, or 'lumo config init'")
	}
	return nil
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
