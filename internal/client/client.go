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
func New(cfg *config.Config) *Client {
	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// adminURL builds the full URL for an admin API endpoint.
func (c *Client) adminURL(path string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	tenant := c.cfg.Tenant
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s/t/%s/api/v1/admin%s", base, tenant, path)
}

// tenantURL builds the full URL for a tenant-scoped API endpoint (non-admin).
func (c *Client) tenantURL(path string) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	tenant := c.cfg.Tenant
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s/t/%s/api/v1%s", base, tenant, path)
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

	// Set headers
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

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
