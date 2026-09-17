// Package auth implements the OAuth 2.0 Device Authorization Grant (RFC 8628)
// for the `lumo login` command.
//
// Flow:
//  1. Start: POST /oauth/device_authorization with client_id=lumoauth-cli.
//  2. Display user_code + verification_uri to the user, optionally open browser.
//  3. Poll: POST /oauth/token at the server-provided interval until success.
//
// The well-known client id `lumoauth-cli` is auto-provisioned per tenant by
// the server's CliClientProvisioner — first-time login on a tenant creates
// the underlying OAuth client transparently.
package auth

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CliClientID is the well-known first-party client id understood by the
// server. The server resolves it to a per-tenant suffixed client transparently.
const CliClientID = "lumoauth-cli"

// AdminScope is the blanket Admin API scope. The Admin API accepts an OAuth
// access token only when it carries `admin` or a resource scope of the form
// `admin:<resource>:<read|write>`, in addition to the user's own permissions.
const AdminScope = "admin"

// DefaultScopes is what `lumo login` requests unless --scope narrows it:
// identity claims for `whoami`, plus the blanket admin scope so every `lumo`
// resource command works. Least-privilege sessions request e.g.
// `openid admin:users:read` instead.
var DefaultScopes = []string{"openid", "profile", "email", AdminScope}

// SplitScopes turns a space- or comma-separated scope string into a list.
func SplitScopes(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' }) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// DeviceCodeGrant is the standard RFC 8628 grant type identifier.
const DeviceCodeGrant = "urn:ietf:params:oauth:grant-type:device_code"

// DeviceAuthResponse is the payload returned by /oauth/device_authorization.
type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse is the success payload from /oauth/token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// errResponse is the OAuth error envelope per RFC 6749.
type errResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Client wraps an HTTP client targeting a specific (baseURL, org) pair.
type Client struct {
	BaseURL  string
	OrgID    string
	HTTP     *http.Client
	Insecure bool
}

// New returns a Client. `insecure` skips TLS verification — useful for local
// dev against self-signed certs.
func New(baseURL, orgID string, insecure bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		BaseURL:  strings.TrimSuffix(baseURL, "/"),
		OrgID:    orgID,
		HTTP:     &http.Client{Transport: tr, Timeout: 30 * time.Second},
		Insecure: insecure,
	}
}

// Start issues the device authorization request. The returned response
// contains the device_code (used for polling) and the user_code +
// verification_uri (shown to the user).
//
// Scopes are always sent explicitly (RFC 8628 §3.1) so the resulting token
// carries exactly what the user asked for; with no scope parameter the server
// would grant the client's whole allowed list.
func (c *Client) Start(scopes []string) (*DeviceAuthResponse, error) {
	form := url.Values{}
	form.Set("client_id", CliClientID)
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}
	form.Set("scope", strings.Join(scopes, " "))

	endpoint := fmt.Sprintf("%s/orgs/%s/api/v1/oauth/device_authorization", c.BaseURL, c.OrgID)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device authorization: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var oauthErr errResponse
		_ = json.NewDecoder(resp.Body).Decode(&oauthErr)
		if oauthErr.Error != "" {
			return nil, fmt.Errorf("%s: %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return nil, fmt.Errorf("device authorization returned %d", resp.StatusCode)
	}

	var out DeviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode device authorization response: %w", err)
	}
	return &out, nil
}

// Refresh exchanges a refresh_token for a fresh access_token at the CLI
// client's token endpoint. The server may rotate the refresh_token — when
// the response carries a new one, callers should persist it.
func (c *Client) Refresh(refreshToken string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", CliClientID)

	endpoint := fmt.Sprintf("%s/orgs/%s/api/v1/oauth/token", c.BaseURL, c.OrgID)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var tok TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
			return nil, fmt.Errorf("decode refresh response: %w", err)
		}
		return &tok, nil
	}

	var oauthErr errResponse
	_ = json.NewDecoder(resp.Body).Decode(&oauthErr)
	if oauthErr.Error != "" {
		return nil, fmt.Errorf("%s: %s", oauthErr.Error, oauthErr.ErrorDescription)
	}
	return nil, fmt.Errorf("refresh returned %d", resp.StatusCode)
}

// PollResult is what Poll returns at each iteration.
type PollResult struct {
	Token   *TokenResponse
	Pending bool   // user hasn't approved yet — keep polling
	Error   string // e.g. "access_denied", "expired_token"
}

// Poll exchanges the device code for tokens. Returns Pending=true while the
// user hasn't approved yet; returns Token on success; returns Error for
// terminal failures (denied, expired, slow_down).
func (c *Client) Poll(deviceCode string) (*PollResult, error) {
	form := url.Values{}
	form.Set("grant_type", DeviceCodeGrant)
	form.Set("device_code", deviceCode)
	form.Set("client_id", CliClientID)

	endpoint := fmt.Sprintf("%s/orgs/%s/api/v1/oauth/token", c.BaseURL, c.OrgID)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var tok TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
			return nil, fmt.Errorf("decode token response: %w", err)
		}
		return &PollResult{Token: &tok}, nil
	}

	var oauthErr errResponse
	_ = json.NewDecoder(resp.Body).Decode(&oauthErr)

	switch oauthErr.Error {
	case "authorization_pending":
		return &PollResult{Pending: true}, nil
	case "slow_down":
		// Server asks us to back off — caller should add to interval.
		return &PollResult{Pending: true, Error: "slow_down"}, nil
	case "access_denied", "expired_token":
		return &PollResult{Error: oauthErr.Error}, nil
	default:
		if oauthErr.Error != "" {
			return nil, fmt.Errorf("%s: %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return nil, errors.New("token endpoint returned " + resp.Status)
	}
}
