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
	Tenant  string `yaml:"tenant" json:"tenant"`
	BaseURL string `yaml:"base_url" json:"base_url"`
	Format  string `yaml:"format" json:"format"`
	Insecure bool  `yaml:"insecure" json:"insecure"`
}

// Load resolves configuration with precedence: flags > env > file.
// Flag values are passed in as overrides (empty string = not set).
func Load(flagAPIKey, flagTenant, flagBaseURL, flagFormat string, flagInsecure bool) (*Config, error) {
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
	if v := os.Getenv("LUMO_TENANT"); v != "" {
		cfg.Tenant = v
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
	if flagTenant != "" {
		cfg.Tenant = flagTenant
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
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required. Set via --api-key flag, LUMO_API_KEY env var, or 'lumo config init'")
	}
	if c.Tenant == "" {
		return fmt.Errorf("tenant slug is required. Set via --tenant flag, LUMO_TENANT env var, or 'lumo config init'")
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
