package cmd

import (
	"fmt"

	"github.com/lumoauth/cli/internal/client"
	"github.com/lumoauth/cli/internal/config"
)

// authHeader returns the Authorization header value for commands that build
// raw http.Request values (tunnel streaming, bulk migrate). It goes through
// the shared client so precedence (login token for this org, else API key)
// and silent refresh are identical everywhere.
func authHeader(cfg *config.Config) (string, error) {
	c := client.New(cfg)
	switch c.AuthMethod() {
	case client.AuthToken:
		return "Bearer " + c.Credentials().AccessToken, nil
	case client.AuthAPIKey:
		return "ApiKey " + cfg.APIKey, nil
	}
	return "", fmt.Errorf("not authenticated; run 'lumo login' or set LUMO_API_KEY")
}
