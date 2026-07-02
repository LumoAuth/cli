package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/config"
)

// Client wraps HTTP interactions with the LumoAuth Admin API.
type Client struct {
	cfg        *config.Config
	creds      *config.Credentials // optional; used when no API key configured
	httpClient *http.Client
}

// APIError represents a structured error from the API.
type APIError struct {
	StatusCode int
	Message    string      `json:"message"`
	Error_     string      `json:"error"`
	Details    interface{} `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Error_
	}
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	return msg
}

// PaginatedResponse wraps a paginated API response.
type PaginatedResponse struct {
	Data         json.RawMessage `json:"data"`
	Meta         PaginationMeta  `json:"meta"`
	ResourceType string          `json:"resourceType"`
}

// PaginationMeta holds pagination info.
type PaginationMeta struct {
	Total           int  `json:"total"`
	Page            int  `json:"page"`
	Limit           int  `json:"limit"`
	TotalPages      int  `json:"totalPages"`
	HasNextPage     bool `json:"hasNextPage"`
	HasPreviousPage bool `json:"hasPreviousPage"`
}

// New creates a new API client from config.
//
// Auth precedence:
//  1. If `lumo login` credentials exist, are unexpired, and target the same
//     org as this request, use the bearer token. This is the per-user,
//     auditable, revocable session — the right default whenever we have it.
//  2. Otherwise fall back to a configured API key (lmk_…) for scripted use.
//  3. Otherwise the request is unauthenticated and will fail at validate().
//
// The earlier "API key wins unconditionally" precedence caused a confusing
// failure mode: a user who'd previously set up a key for org A and then ran
// `lumo login` against org B would silently keep sending the org-A key to
// org-B routes, and the server would reject with "Invalid API key" — even
// though their device-flow credentials were perfectly valid for the route.
func New(cfg *config.Config) *Client {
	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	c := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}

	creds, _ := config.LoadCredentials()
	if creds.HasToken() && creds.IsExpired() {
		// Best-effort silent refresh. If it fails we fall through to the
		// API-key path; Validate() will have surfaced a clear error already
		// when the user has neither.
		_ = creds.EnsureFresh(cfg.Insecure)
	}
	if creds == nil {
		return c
	}

	// Inherit org/base from the active profile when not pinned, so commands
	// work without requiring --org-id when the user just ran `lumo login`
	// (or created a token-less profile pinning the org for API-key use).
	if c.cfg.OrgID == "" {
		c.cfg.OrgID = creds.OrgID
	}
	if c.cfg.BaseURL == "" || c.cfg.BaseURL == "https://app.lumoauth.dev" {
		if creds.BaseURL != "" {
			c.cfg.BaseURL = creds.BaseURL
		}
	}

	// Only attach the bearer when the profile holds a live token AND the
	// credentials actually belong to the org being addressed; otherwise the
	// API key path stays in effect (and the request will succeed if the key
	// is for the right tenant, or fail with a clear server-side
	// tenant-mismatch otherwise).
	if creds.HasToken() && !creds.IsExpired() && creds.OrgID == c.cfg.OrgID {
		c.creds = creds
	}

	return c
}

// adminURL builds the full URL for an admin API endpoint.
func (c *Client) adminURL(path string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	orgID := c.cfg.OrgID
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s/orgs/%s/api/v1/admin%s", base, orgID, path)
}

// orgURL builds the full URL for an org-scoped API endpoint (non-admin).
func (c *Client) orgURL(path string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	orgID := c.cfg.OrgID
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s/orgs/%s/api/v1%s", base, orgID, path)
}

// rawURL builds a URL from the base URL and arbitrary path.
func (c *Client) rawURL(path string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// Get performs a GET request to an admin API endpoint.
func (c *Client) Get(path string, query url.Values) (json.RawMessage, error) {
	fullURL := c.adminURL(path)
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	return c.doRequest("GET", fullURL, nil)
}

// Post performs a POST request to an admin API endpoint.
func (c *Client) Post(path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest("POST", c.adminURL(path), body)
}

// Put performs a PUT request to an admin API endpoint.
func (c *Client) Put(path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest("PUT", c.adminURL(path), body)
}

// Patch performs a PATCH request to an admin API endpoint.
func (c *Client) Patch(path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest("PATCH", c.adminURL(path), body)
}

// Delete performs a DELETE request to an admin API endpoint.
func (c *Client) Delete(path string) (json.RawMessage, error) {
	return c.doRequest("DELETE", c.adminURL(path), nil)
}

// RawRequest performs a request to an arbitrary path relative to the base URL.
func (c *Client) RawRequest(method, path string, body interface{}) (json.RawMessage, error) {
	return c.doRequest(method, c.rawURL(path), body)
}

func (c *Client) doRequest(method, fullURL string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers — prefer device-flow Bearer (when it matches this org),
	// otherwise fall back to the API key. See `New` for the rationale.
	if c.creds != nil {
		req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	} else if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// CSRF defence-in-depth: admin API rejects cookie/Bearer state-changing
	// requests without a custom header. Sending it unconditionally is safe.
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, apiErr); err != nil {
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	return json.RawMessage(respBody), nil
}
