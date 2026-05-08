package cmd

import (
	"fmt"

	"github.com/lumoauth/cli/internal/config"
)

// authHeader returns the Authorization (or X-API-Key) header value the
// CLI uses for authenticated requests, mirroring client.New's logic.
//
// Used by commands that build raw http.Request values (dev, tunnel, migrate)
// rather than going through the wrapped HTTP client.
func authHeader(cfg *config.Config) (string, error) {
	if cfg.APIKey != "" {
		return "ApiKey " + cfg.APIKey, nil
	}
	creds, err := config.LoadCredentials()
	if err != nil {
		return "", err
	}
	if creds == nil {
		return "", fmt.Errorf("not authenticated; run 'lumo login'")
	}
	if creds.IsExpired() {
		return "", fmt.Errorf("credentials expired; run 'lumo login'")
	}
	return "Bearer " + creds.AccessToken, nil
}
