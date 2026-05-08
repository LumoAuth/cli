package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// CredentialsFile is the device-flow credential cache, separate from
// config.yaml so that:
//   - users editing config.yaml don't accidentally leak tokens to git.
//   - `lumo logout` can wipe credentials without touching settings.
//   - the access token's expiry is tracked even when a refresh token is in use.
const CredentialsFile = "credentials.yaml"

// Credentials is the persisted device-flow auth state.
type Credentials struct {
	BaseURL      string    `yaml:"base_url"`
	OrgID        string    `yaml:"org_id"`
	UserEmail    string    `yaml:"user_email,omitempty"`
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token,omitempty"`
	ExpiresAt    time.Time `yaml:"expires_at"`
	TokenType    string    `yaml:"token_type,omitempty"` // usually "Bearer"
}

// CredentialsPath returns the absolute path to the credentials file.
func CredentialsPath() string {
	return filepath.Join(ConfigDir(), CredentialsFile)
}

// LoadCredentials reads the credentials file. Returns nil, nil when no
// credentials are stored — callers should fall back to API-key auth.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var c Credentials
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &c, nil
}

// SaveCredentials writes the credentials file with 0600 perms (rw-owner-only).
func (c *Credentials) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.WriteFile(CredentialsPath(), data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

// ClearCredentials removes the credentials file. Idempotent.
func ClearCredentials() error {
	if err := os.Remove(CredentialsPath()); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove credentials: %w", err)
	}
	return nil
}

// IsExpired reports whether the access token has passed its expiry — the
// caller should refresh (or re-login) before issuing API calls.
func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	// Consider expired 30s early to avoid mid-flight failures from clock skew.
	return time.Now().Add(30 * time.Second).After(c.ExpiresAt)
}
