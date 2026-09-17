package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lumoauth/cli/internal/config"
)

func isolatedConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LUMO_CONFIG_DIR", dir)
	t.Setenv(config.EnvProfile, "")
	config.SetProfileOverride("")
}

func TestResolveURL(t *testing.T) {
	isolatedConfig(t)
	c := New(&config.Config{BaseURL: "https://x.test/", OrgID: "acme"})
	cases := map[string]string{
		"/users":                          "https://x.test/orgs/acme/api/v1/admin/users",
		"users":                           "https://x.test/orgs/acme/api/v1/admin/users",
		"/admin/users":                    "https://x.test/orgs/acme/api/v1/admin/users",
		"/orgs/other/api/v1/me":           "https://x.test/orgs/other/api/v1/me",
		"/api/v1/authz/check":             "https://x.test/api/v1/authz/check",
		"/.well-known/agent.json":         "https://x.test/.well-known/agent.json",
		"https://elsewhere.test/anything": "https://elsewhere.test/anything",
	}
	for in, want := range cases {
		if got := c.ResolveURL(in); got != want {
			t.Errorf("ResolveURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAuthPrecedenceTokenBeatsAPIKeyForSameOrg(t *testing.T) {
	isolatedConfig(t)
	creds := &config.Credentials{BaseURL: "https://x.test", OrgID: "acme", AccessToken: "at", RefreshToken: "rt",
		ExpiresAt: time.Now().Add(time.Hour), Scopes: []string{"openid", "admin"}}
	if err := config.SaveProfile("default", creds, true); err != nil {
		t.Fatal(err)
	}

	c := New(&config.Config{BaseURL: config.DefaultBaseURL, APIKey: "lmk_key"})
	if c.AuthMethod() != AuthToken {
		t.Fatalf("expected token auth for the logged-in org, got %s", c.AuthMethod())
	}
	if c.Config().OrgID != "acme" || c.Config().BaseURL != "https://x.test" {
		t.Fatalf("org/base URL not inherited from profile: %+v", c.Config())
	}
	name, value, _ := c.AuthorizationHeader()
	if name != "Authorization" || value != "Bearer at" {
		t.Fatalf("unexpected header %s: %s", name, value)
	}

	// A different org falls back to the API key rather than sending the wrong token.
	other := New(&config.Config{BaseURL: "https://x.test", OrgID: "other", APIKey: "lmk_key"})
	if other.AuthMethod() != AuthAPIKey {
		t.Fatalf("expected api-key auth for a different org, got %s", other.AuthMethod())
	}
	name, value, _ = other.AuthorizationHeader()
	if name != "X-API-Key" || value != "lmk_key" {
		t.Fatalf("unexpected header %s: %s", name, value)
	}
}

func TestErrorHintsAndRefreshRetry(t *testing.T) {
	isolatedConfig(t)

	var tokenCalls, adminCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth/token"):
			tokenCalls++
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "rt" {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "at2", "refresh_token": "rt2", "expires_in": 3600, "scope": "openid admin"})
		case strings.HasSuffix(r.URL.Path, "/admin/users"):
			adminCalls++
			if r.Header.Get("Authorization") != "Bearer at2" {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":"unauthorized","message":"Authentication required","status":401}`))
				return
			}
			if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				t.Errorf("missing CSRF witness header")
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/admin/webhooks"):
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":"forbidden","message":"Insufficient scope","status":403,"details":{"required_scopes":["admin:webhooks:read"],"granted_scopes":["admin:users:read"]}}`))
		case strings.HasSuffix(r.URL.Path, "/admin/organization"):
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"invalid_request","message":"Unsupported settings key(s): sandbox","status":400,"details":{"rejected_keys":["sandbox"]}}`))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"not_found","message":"Resource not found","status":404}`))
		}
	}))
	defer srv.Close()

	creds := &config.Credentials{BaseURL: srv.URL, OrgID: "acme", AccessToken: "stale", RefreshToken: "rt",
		ExpiresAt: time.Now().Add(time.Hour)}
	if err := config.SaveProfile("default", creds, true); err != nil {
		t.Fatal(err)
	}

	c := New(&config.Config{BaseURL: srv.URL, OrgID: "acme"})
	if c.AuthMethod() != AuthToken {
		t.Fatalf("expected token auth, got %s", c.AuthMethod())
	}

	// 401 with a stale token → one refresh + one retry, then success.
	if _, err := c.Get("/users", nil); err != nil {
		t.Fatalf("expected retry after refresh to succeed, got %v", err)
	}
	if tokenCalls != 1 || adminCalls != 2 {
		t.Fatalf("expected 1 refresh and 2 admin calls, got %d/%d", tokenCalls, adminCalls)
	}
	stored, _ := config.LoadCredentials()
	if stored.AccessToken != "at2" || stored.RefreshToken != "rt2" || !stored.HasScope("admin") {
		t.Fatalf("rotated tokens/scopes not persisted: %+v", stored)
	}
	if b, _ := os.ReadFile(filepath.Join(os.Getenv("LUMO_CONFIG_DIR"), config.CredentialsFile)); strings.Contains(string(b), "stale") {
		t.Fatalf("stale token still on disk")
	}

	// 403 with required_scopes → hint names the scope and how to get it.
	_, err := c.Get("/webhooks", nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != 403 || apiErr.Code != "forbidden" {
		t.Fatalf("unexpected error %+v", apiErr)
	}
	if got := apiErr.RequiredScopes(); len(got) != 1 || got[0] != "admin:webhooks:read" {
		t.Fatalf("RequiredScopes = %v", got)
	}
	if h := apiErr.Hint(); !strings.Contains(h, "admin:webhooks:read") || !strings.Contains(h, "lumo login --scope") {
		t.Fatalf("hint not actionable: %q", h)
	}

	// 400 rejected settings keys → hint lists them.
	_, err = c.Patch("/organization", map[string]interface{}{"settings": map[string]interface{}{"sandbox": 1}})
	apiErr, _ = err.(*APIError)
	if apiErr == nil || apiErr.StatusCode != 400 || !strings.Contains(apiErr.Hint(), "sandbox") {
		t.Fatalf("expected rejected_keys hint, got %v", err)
	}

	// 404 mentions the org and path resolution.
	_, err = c.Get("/nope", nil)
	apiErr, _ = err.(*APIError)
	if apiErr == nil || apiErr.StatusCode != 404 || !strings.Contains(apiErr.Hint(), "acme") {
		t.Fatalf("expected 404 hint naming the org, got %v", err)
	}
}

func TestAPIKeyHintsWithoutToken(t *testing.T) {
	isolatedConfig(t)
	c := New(&config.Config{BaseURL: "https://x.test", OrgID: "acme", APIKey: "lmk_abc"})
	e := c.apiError(401, "GET", "https://x.test/orgs/acme/api/v1/admin/users", []byte(`{"error":"unauthorized","message":"Invalid API key","status":401}`))
	if !strings.Contains(e.Hint(), "portal/settings/api-keys") {
		t.Fatalf("api-key 401 hint should link to the portal, got %q", e.Hint())
	}
	e = c.apiError(403, "POST", "https://x.test/orgs/acme/api/v1/admin/users", []byte(`{"error":"forbidden","message":"Access denied: this API key has no admin write scope for this operation","status":403}`))
	if !strings.Contains(e.Hint(), "admin:<resource>:<read|write>") {
		t.Fatalf("api-key 403 hint should explain resource scopes, got %q", e.Hint())
	}
	// OAuth-style envelope and legacy boolean `error` are both tolerated.
	e = c.apiError(400, "POST", "https://x.test/orgs/acme/api/v1/oauth/token", []byte(`{"error":"invalid_grant","error_description":"Code expired"}`))
	if e.Error() != "Code expired (HTTP 400 invalid_grant)" {
		t.Fatalf("unexpected OAuth error text %q", e.Error())
	}
	e = c.apiError(500, "GET", "https://x.test/x", []byte(`{"error":true,"message":"boom"}`))
	if e.Error() != "boom (HTTP 500)" {
		t.Fatalf("unexpected legacy error text %q", e.Error())
	}
}
