// Package client is the single HTTP path every `lumo` command uses to talk
// to LumoAuth. It owns credential precedence, silent token refresh, the
// headers the Admin API expects, and the translation of API errors into
// actionable messages.
//
// Credential precedence (see New):
//  1. `lumo login` device-flow token for the org being addressed.
//  2. A configured API key (`lmk_…`) for scripted use.
//  3. Nothing — the request fails at config validation with a clear hint.
//
// Both credential classes are stateless on the server: no session cookie is
// ever issued, so the credential is sent on every request and scopes are
// enforced per resource. When the server answers 403 with the scopes it
// wanted, the error carries a hint that says exactly how to get them.
package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/config"
)

// AuthMethod names the credential class a Client will send.
type AuthMethod string

const (
	AuthNone   AuthMethod = "none"
	AuthToken  AuthMethod = "token"   // device-flow OAuth access token (lumo login)
	AuthAPIKey AuthMethod = "api-key" // organization API key (lmk_…)
)

// Client wraps HTTP interactions with the LumoAuth APIs.
type Client struct {
	cfg        *config.Config
	creds      *config.Credentials // set when a live token for this org is in use
	httpClient *http.Client
	method     AuthMethod
	// extraHeaders are sent on every request from this client (see WithHeader).
	extraHeaders map[string]string
}

// APIError is a structured error from the API, enriched with a hint that
// tells the user what to do next.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    map[string]interface{}
	// OAuth-style envelope ({error, error_description}) used by the OAuth and
	// agent endpoints.
	ErrorDescription string
	// Method/path of the failed request, for hints.
	Method string
	Path   string
	// Auth the client sent, for hints.
	Auth AuthMethod
	// BaseURL/OrgID for building portal links in hints.
	BaseURL string
	OrgID   string
	// Billing gates: the feature key or limit key the plan lacks, and the
	// cheapest unlock ({"addon": id} and/or {"plan": id}) the server suggests.
	Feature string
	Limit   string
	Upgrade map[string]string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.ErrorDescription
	}
	if msg == "" && e.Code != "" && e.Code != "true" {
		msg = e.Code
	}
	if msg == "" {
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
	if e.Code != "" && e.Code != "true" && !strings.EqualFold(e.Code, msg) {
		return fmt.Sprintf("%s (HTTP %d %s)", msg, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("%s (HTTP %d)", msg, e.StatusCode)
}

// RequiredScopes returns the scopes the server said were missing, if any.
func (e *APIError) RequiredScopes() []string {
	return e.detailStrings("required_scopes", "required_scope")
}

// GrantedScopes returns the scopes the server saw on the credential, if any.
func (e *APIError) GrantedScopes() []string {
	return e.detailStrings("granted_scopes", "key_scopes")
}

// RejectedKeys returns `details.rejected_keys` for a 400 on the organization
// settings endpoint.
func (e *APIError) RejectedKeys() []string {
	return e.detailStrings("rejected_keys")
}

func (e *APIError) detailStrings(keys ...string) []string {
	if e.Details == nil {
		return nil
	}
	for _, k := range keys {
		switch v := e.Details[k].(type) {
		case []interface{}:
			out := make([]string, 0, len(v))
			for _, s := range v {
				if str, ok := s.(string); ok {
					out = append(out, str)
				}
			}
			sort.Strings(out)
			return out
		case string:
			return []string{v}
		}
	}
	return nil
}

// Hint returns a one-or-two-line, actionable next step for this error, or
// "" when there is nothing useful to add.
func (e *APIError) Hint() string {
	portalKeys := apiKeysURL(e.BaseURL, e.OrgID)
	switch e.Code {
	case "step_up_required":
		return "This action needs a fresh MFA check of your own (within 10 minutes). Approve a step_up challenge (POST /orgs/" + orDefault(e.OrgID, "<org>") + "/api/v1/mfa/challenges) and pass its id with --mfa-challenge, or use an organization API key."
	case "mfa_reset_removed":
		return "Admins can no longer switch MFA off for a user. Use 'lumo users tap <user> --reason ...' for a one-time recovery code, or 'lumo users authenticators remove <user> <authenticator-id>' for a lost device."
	case "identity_conflict":
		return "That identity is already linked to another user, or this user is linked to a different federated source. Check 'lumo users identities list <user>' and unlink the conflicting link first."
	case "directory_lookup_failed":
		return "The directory entry could not be found automatically. Pass the user's distinguished name with --dn."
	case "last_factor_required":
		return "This is the user's last sign-in method. Issue a temporary access code ('lumo users tap') so they can enroll a replacement first."
	case "feature_not_in_plan":
		return e.billingHint("The organization's plan does not include " + featureLabel(e.Feature) + ".")
	case "plan_limit_reached":
		return e.billingHint("The organization is at its plan limit for " + orDefault(strings.ReplaceAll(e.Limit, "_", " "), "this resource") + ".")
	}
	switch e.StatusCode {
	case 401:
		switch e.Auth {
		case AuthAPIKey:
			return "The API key was rejected. Check LUMO_API_KEY / --api-key, that it belongs to organization '" + orDefault(e.OrgID, "<org>") + "', and that it has not been revoked. Create a new one at " + portalKeys
		case AuthToken:
			return "Your login has expired or was revoked. Run 'lumo login' again."
		default:
			return "Not authenticated. Run 'lumo login' for interactive use, or set LUMO_API_KEY for scripts."
		}
	case 403:
		if req := e.RequiredScopes(); len(req) > 0 {
			granted := e.GrantedScopes()
			g := "none"
			if len(granted) > 0 {
				g = strings.Join(granted, " ")
			}
			switch e.Auth {
			case AuthAPIKey:
				return fmt.Sprintf("This API key lacks the scope(s) %s (it has: %s). Create a key with those scopes at %s — read scopes never authorise writes.", strings.Join(req, ", "), g, portalKeys)
			case AuthToken:
				return fmt.Sprintf("Your login token lacks the scope(s) %s (it has: %s). Re-authenticate with 'lumo login --scope %s' (or the blanket 'admin' scope). If the server refuses the scope, an administrator must allow it on the 'lumoauth-cli' OAuth client.", strings.Join(req, ", "), g, strings.Join(req, ","))
			}
			return "Missing scope(s): " + strings.Join(req, ", ")
		}
		lower := strings.ToLower(e.Message)
		switch {
		case strings.Contains(lower, "settings.manage") || strings.Contains(lower, "admin privileges"):
			return "The signed-in user is not an administrator of this organization (needs the settings.manage permission). Ask an admin to grant it, or use an organization API key."
		case strings.Contains(lower, "admin") && strings.Contains(lower, "scope"):
			if e.Auth == AuthToken {
				return "This token has no admin scope. Run 'lumo login' again (the CLI requests the 'admin' scope by default), or use an organization API key for scripts."
			}
			return "This API key has no admin scope for this operation. Create a key with the right admin:<resource>:<read|write> scopes at " + portalKeys
		case strings.Contains(lower, "cross-tenant") || strings.Contains(lower, "tenant"):
			return "The credential belongs to a different organization than --org '" + orDefault(e.OrgID, "<org>") + "'. Check 'lumo whoami' and 'lumo profile list'."
		case strings.Contains(lower, "csrf"):
			return "The server expected a same-origin witness; this is a CLI bug — please report it with 'lumo --version'."
		}
		return "Permission denied. Run 'lumo doctor' to see what this credential can reach."
	case 404:
		return "Nothing at that path for organization '" + orDefault(e.OrgID, "<org>") + "'. Check the id and --org; 'lumo api' paths are relative to /orgs/<org>/api/v1/admin unless they start with /orgs or /api."
	case 400:
		if keys := e.RejectedKeys(); len(keys) > 0 {
			return "Rejected settings key(s): " + strings.Join(keys, ", ") + ". Only the documented, non-security keys are writable through the API; nothing was saved."
		}
	case 429:
		return "Rate limited. Wait a moment and retry; scripts should back off exponentially."
	}
	if e.StatusCode >= 500 {
		return "The server had a problem. Retry shortly; if it persists, check the server logs or status page."
	}
	return ""
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// billingHint turns the server's `upgrade` suggestion into a next step that
// ends at the organization's billing page, where the add-on or plan is bought.
func (e *APIError) billingHint(what string) string {
	billing := billingURL(e.BaseURL, e.OrgID, orDefault(e.Feature, e.Limit))
	var unlock string
	switch {
	case e.Upgrade["plan"] != "" && e.Upgrade["addon"] != "":
		unlock = fmt.Sprintf("Upgrade to the %s plan and add the %s add-on", planLabel(e.Upgrade["plan"]), addonLabel(e.Upgrade["addon"]))
	case e.Upgrade["addon"] != "":
		unlock = fmt.Sprintf("Add the %s add-on", addonLabel(e.Upgrade["addon"]))
	case e.Upgrade["plan"] != "":
		unlock = fmt.Sprintf("Upgrade to the %s plan", planLabel(e.Upgrade["plan"]))
	default:
		return what + " Contact sales, or ask an organization admin to check " + billing
	}
	return fmt.Sprintf("%s %s at %s (an organization admin can do this; it takes effect immediately).", what, unlock, billing)
}

func billingURL(baseURL, orgID, unlock string) string {
	if baseURL == "" {
		baseURL = "<base-url>"
	}
	u := strings.TrimRight(baseURL, "/") + "/orgs/" + orDefault(orgID, "<org>") + "/portal/billing"
	if unlock != "" {
		u += "?unlock=" + url.QueryEscape(unlock)
	}
	return u
}

// Customer-facing names for the catalog ids that reach the CLI. Anything
// else falls back to the id so a new add-on never prints as an empty string.
func addonLabel(id string) string {
	switch id {
	case "dev_sandboxes":
		return "Developer sandboxes ($199/month)"
	case "fraud_shield":
		return "Fraud Shield"
	case "agent_governance":
		return "Agent Governance"
	case "audit_compliance":
		return "Audit & Compliance"
	case "fine_grained_authz":
		return "Fine-grained authorization"
	case "threat_detection":
		return "Threat Detection"
	case "byok_keys":
		return "Bring-your-own signing keys"
	}
	return strings.ReplaceAll(id, "_", " ")
}

func planLabel(id string) string {
	switch id {
	case "free":
		return "Developer"
	case "pro":
		return "Pro"
	case "business":
		return "Business"
	}
	return id
}

func featureLabel(id string) string {
	switch id {
	case "dev_sandboxes":
		return "ephemeral sandbox organizations (lumo dev start)"
	case "":
		return "this feature"
	}
	return strings.ReplaceAll(id, "_", " ")
}

func apiKeysURL(baseURL, orgID string) string {
	if baseURL == "" {
		baseURL = "<base-url>"
	}
	return strings.TrimRight(baseURL, "/") + "/orgs/" + orDefault(orgID, "<org>") + "/portal/settings/api-keys"
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

// New creates an API client from config.
//
// Auth precedence:
//  1. If `lumo login` credentials exist, are unexpired (refreshed silently
//     when possible), and target the same org as this request, use the bearer
//     token. This is the per-user, auditable, revocable session — the right
//     default whenever we have it.
//  2. Otherwise fall back to a configured API key (lmk_…) for scripted use.
//  3. Otherwise the request is unauthenticated and will fail at validate().
func New(cfg *config.Config) *Client {
	creds, _ := config.LoadCredentials()
	// Inherit org / base URL / TLS policy from the active profile when not
	// pinned, so commands work without --org or --insecure right after
	// `lumo login`. Must happen before the transport is built.
	cfg.InheritFromProfile(creds)
	c := &Client{cfg: cfg, httpClient: NewHTTPClient(cfg.Insecure, 30*time.Second), method: AuthNone}

	if creds.HasToken() && creds.IsExpired() && creds.OrgID == cfg.OrgID {
		// Best-effort silent refresh. If it fails we fall through to the
		// API-key path; Validate() surfaces a clear error when there is neither.
		_ = creds.EnsureFresh(cfg.Insecure)
	}
	if creds != nil {
		if creds.HasToken() && !creds.IsExpired() && creds.OrgID == c.cfg.OrgID {
			c.creds = creds
			c.method = AuthToken
		}
	}
	if c.method == AuthNone && c.cfg.APIKey != "" {
		c.method = AuthAPIKey
	}
	return c
}

// NewHTTPClient builds the http.Client every command shares (TLS policy,
// timeout). Streaming commands pass a zero timeout.
func NewHTTPClient(insecure bool, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- explicit opt-in for local dev
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// WithHeader returns a copy of the client that also sends name: value on
// every request, e.g. X-MFA-Challenge for step-up protected endpoints.
func (c *Client) WithHeader(name, value string) *Client {
	cp := *c
	cp.extraHeaders = make(map[string]string, len(c.extraHeaders)+1)
	for k, v := range c.extraHeaders {
		cp.extraHeaders[k] = v
	}
	cp.extraHeaders[name] = value
	return &cp
}

// Config returns the effective configuration (org/base URL after profile
// inheritance).
func (c *Client) Config() *config.Config { return c.cfg }

// AuthMethod reports which credential class requests will carry.
func (c *Client) AuthMethod() AuthMethod { return c.method }

// Credentials returns the device-flow credentials in use, or nil.
func (c *Client) Credentials() *config.Credentials { return c.creds }

// HTTPClient exposes the underlying http.Client for streaming callers.
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// AuthorizationHeader returns the header name and value to authenticate a
// hand-built request with the same precedence every other command uses.
func (c *Client) AuthorizationHeader() (name, value string, err error) {
	switch c.method {
	case AuthToken:
		return "Authorization", "Bearer " + c.creds.AccessToken, nil
	case AuthAPIKey:
		return "X-API-Key", c.cfg.APIKey, nil
	}
	return "", "", fmt.Errorf("not authenticated; run 'lumo login' or set LUMO_API_KEY")
}

// AdminURL builds the full URL for an Admin API endpoint.
func (c *Client) AdminURL(path string) string {
	return c.OrgURL("/admin" + ensureLeadingSlash(path))
}

// OrgURL builds the full URL for an org-scoped API endpoint (non-admin).
func (c *Client) OrgURL(path string) string {
	return fmt.Sprintf("%s/orgs/%s/api/v1%s", strings.TrimRight(c.cfg.BaseURL, "/"), c.cfg.OrgID, ensureLeadingSlash(path))
}

// RawURL builds a URL from the base URL and an arbitrary path.
func (c *Client) RawURL(path string) string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + ensureLeadingSlash(path)
}

// ResolveURL turns the paths people type into a full URL:
//
//	/users                  → {base}/orgs/{org}/api/v1/admin/users   (Admin API, the common case)
//	admin/users             → same
//	/orgs/x/api/v1/...      → {base}/orgs/x/api/v1/...               (absolute API path)
//	/api/v1/authz/check     → {base}/api/v1/authz/check              (global API)
//	/.well-known/...        → {base}/.well-known/...
//	https://host/anything   → as given
func (c *Client) ResolveURL(path string) string {
	p := strings.TrimSpace(path)
	switch {
	case strings.HasPrefix(p, "http://"), strings.HasPrefix(p, "https://"):
		return p
	case strings.HasPrefix(p, "/orgs/"), strings.HasPrefix(p, "/api/"), strings.HasPrefix(p, "/.well-known/"),
		strings.HasPrefix(p, "/healthz"):
		return c.RawURL(p)
	case strings.HasPrefix(ensureLeadingSlash(p), "/admin/"):
		return c.OrgURL(ensureLeadingSlash(p))
	}
	return c.AdminURL(p)
}

func ensureLeadingSlash(p string) string {
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// Get performs a GET request to an Admin API endpoint.
func (c *Client) Get(path string, query url.Values) (json.RawMessage, error) {
	return c.do("GET", withQuery(c.AdminURL(path), query), nil)
}

// Post performs a POST request to an Admin API endpoint.
func (c *Client) Post(path string, body interface{}) (json.RawMessage, error) {
	return c.do("POST", c.AdminURL(path), body)
}

// Put performs a PUT request to an Admin API endpoint.
func (c *Client) Put(path string, body interface{}) (json.RawMessage, error) {
	return c.do("PUT", c.AdminURL(path), body)
}

// Patch performs a PATCH request to an Admin API endpoint.
func (c *Client) Patch(path string, body interface{}) (json.RawMessage, error) {
	return c.do("PATCH", c.AdminURL(path), body)
}

// Delete performs a DELETE request to an Admin API endpoint.
func (c *Client) Delete(path string) (json.RawMessage, error) {
	return c.do("DELETE", c.AdminURL(path), nil)
}

// Request performs a request to a path resolved with ResolveURL.
func (c *Client) Request(method, path string, query url.Values, body interface{}) (json.RawMessage, error) {
	return c.do(strings.ToUpper(method), withQuery(c.ResolveURL(path), query), body)
}

// RawRequest performs a request to an arbitrary path relative to the base URL.
func (c *Client) RawRequest(method, path string, body interface{}) (json.RawMessage, error) {
	return c.do(strings.ToUpper(method), c.RawURL(path), body)
}

// NewRequest builds an authenticated *http.Request for callers that need
// the raw response (streams, large downloads). Headers match do().
func (c *Client) NewRequest(method, fullURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req, body != nil)
	return req, nil
}

func (c *Client) applyHeaders(req *http.Request, hasBody bool) {
	if name, value, err := c.AuthorizationHeader(); err == nil {
		req.Header.Set(name, value)
	}
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	// The Admin API's CSRF witness for cookie callers; harmless for
	// credential callers and keeps every path identical.
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", userAgent)
	for k, v := range c.extraHeaders {
		req.Header.Set(k, v)
	}
}

// userAgent is stamped by the command layer with the build version.
var userAgent = "lumo-cli"

// SetUserAgent lets the command layer stamp the build version on requests.
func SetUserAgent(ua string) {
	if ua != "" {
		userAgent = ua
	}
}

func withQuery(fullURL string, query url.Values) string {
	if len(query) == 0 {
		return fullURL
	}
	sep := "?"
	if strings.Contains(fullURL, "?") {
		sep = "&"
	}
	return fullURL + sep + query.Encode()
}

func (c *Client) do(method, fullURL string, body interface{}) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		payload = data
	}

	resp, respBody, err := c.send(method, fullURL, payload)
	if err != nil {
		return nil, err
	}

	// One transparent retry: a token the server no longer accepts (revoked
	// server-side, or expired by clock skew) gets refreshed and re-sent.
	if resp.StatusCode == 401 && c.method == AuthToken && c.creds != nil && c.creds.RefreshToken != "" {
		if refreshErr := c.creds.ForceRefresh(c.cfg.Insecure); refreshErr == nil {
			resp, respBody, err = c.send(method, fullURL, payload)
			if err != nil {
				return nil, err
			}
		}
	}

	if resp.StatusCode >= 400 {
		return nil, c.apiError(resp.StatusCode, method, fullURL, respBody)
	}
	return json.RawMessage(respBody), nil
}

func (c *Client) send(method, fullURL string, payload []byte) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if payload != nil {
		reqBody = bytes.NewReader(payload)
	}
	req, err := c.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}
	return resp, respBody, nil
}

func (c *Client) apiError(status int, method, fullURL string, body []byte) *APIError {
	apiErr := &APIError{StatusCode: status, Method: method, Auth: c.method, BaseURL: c.cfg.BaseURL, OrgID: c.cfg.OrgID}
	if u, err := url.Parse(fullURL); err == nil {
		apiErr.Path = u.Path
	}
	// The generic envelope is {error, message, status, details}; the OAuth
	// envelope is {error, error_description}. `error` may also be a boolean
	// in a few legacy responses — tolerate all of them.
	var probe struct {
		Error            json.RawMessage        `json:"error"`
		Message          string                 `json:"message"`
		ErrorDescription string                 `json:"error_description"`
		Details          map[string]interface{} `json:"details"`
		// Billing gates ({error: feature_not_in_plan | plan_limit_reached})
		// carry what was refused and the cheapest way to unlock it.
		Feature string            `json:"feature"`
		Limit   string            `json:"limit"`
		Upgrade map[string]string `json:"upgrade"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		apiErr.Message = strings.TrimSpace(string(body))
		if len(apiErr.Message) > 300 {
			apiErr.Message = apiErr.Message[:300] + "…"
		}
		return apiErr
	}
	var code string
	if len(probe.Error) > 0 {
		if err := json.Unmarshal(probe.Error, &code); err != nil {
			code = strings.Trim(string(probe.Error), `"`)
		}
	}
	apiErr.Code = code
	apiErr.Message = probe.Message
	apiErr.ErrorDescription = probe.ErrorDescription
	apiErr.Details = probe.Details
	apiErr.Feature = probe.Feature
	apiErr.Limit = probe.Limit
	apiErr.Upgrade = probe.Upgrade
	return apiErr
}
