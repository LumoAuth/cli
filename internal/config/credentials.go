package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/auth"
	"gopkg.in/yaml.v3"
)

// CredentialsFile is the device-flow credential cache, separate from
// config.yaml so that:
//   - users editing config.yaml don't accidentally leak tokens to git.
//   - `lumo logout` can wipe credentials without touching settings.
//   - the access token's expiry is tracked even when a refresh token is in use.
const CredentialsFile = "credentials.yaml"

// CredentialsBackupFile is where the pre-migration single-profile file is
// preserved the first time the multi-profile layout is written.
const CredentialsBackupFile = "credentials.yaml.bak"

// DefaultProfile is the profile name legacy single-profile credentials are
// migrated into, and the fallback when nothing selects a profile.
const DefaultProfile = "default"

// EnvProfile selects the active profile, overriding current_profile in the
// credentials file. The --profile flag overrides this in turn.
const EnvProfile = "LUMO_PROFILE"

// storeVersion is the on-disk schema version of the multi-profile layout.
const storeVersion = 2

// profileOverride is set from the global --profile flag (highest precedence).
var profileOverride string

// SetProfileOverride records the --profile flag value. Empty means unset.
func SetProfileOverride(name string) { profileOverride = name }

// profileNameRe restricts profile names to something safe for YAML keys,
// shell completion, and log lines.
var profileNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateProfileName rejects names that would be awkward on disk or in flags.
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if !profileNameRe.MatchString(name) {
		return fmt.Errorf("invalid profile name %q — use letters, digits, '.', '_' or '-' (must start with a letter or digit)", name)
	}
	return nil
}

// Credentials is the persisted device-flow auth state for one profile.
type Credentials struct {
	BaseURL      string    `yaml:"base_url"`
	OrgID        string    `yaml:"org_id"`
	UserEmail    string    `yaml:"user_email,omitempty"`
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token,omitempty"`
	ExpiresAt    time.Time `yaml:"expires_at"`
	TokenType    string    `yaml:"token_type,omitempty"` // usually "Bearer"
	// Scopes granted to the access token (from the token response). Used by
	// `lumo whoami` / `lumo doctor` to explain a 403 before it happens.
	Scopes []string `yaml:"scopes,omitempty"`
	// Insecure records that this profile was logged in with TLS verification
	// disabled (--insecure, or a local/private dev URL). It applies only to
	// requests to this profile's own BaseURL — see InsecureFor.
	Insecure bool `yaml:"insecure,omitempty"`

	// profile records which named profile these credentials were loaded
	// from, so Save() writes back to the right slot. Not serialized.
	profile string `yaml:"-"`
}

// Profile returns the profile name these credentials were loaded from
// (empty for credentials constructed in memory and not yet saved).
func (c *Credentials) Profile() string { return c.profile }

// CredentialsStore is the multi-profile on-disk shape of credentials.yaml:
//
//	version: 2
//	current_profile: default
//	profiles:
//	  default:
//	    base_url: ...
//	    org_id: ...
//	    access_token: ...
type CredentialsStore struct {
	Version        int                     `yaml:"version"`
	CurrentProfile string                  `yaml:"current_profile,omitempty"`
	Profiles       map[string]*Credentials `yaml:"profiles"`
}

// ProfileNames returns the store's profile names, sorted.
func (s *CredentialsStore) ProfileNames() []string {
	names := make([]string, 0, len(s.Profiles))
	for n := range s.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// CredentialsPath returns the absolute path to the credentials file.
func CredentialsPath() string {
	return filepath.Join(ConfigDir(), CredentialsFile)
}

// CredentialsBackupPath returns the path of the pre-migration backup.
func CredentialsBackupPath() string {
	return filepath.Join(ConfigDir(), CredentialsBackupFile)
}

// LoadStore reads credentials.yaml and returns the multi-profile store.
// A missing file yields an empty store (not an error).
//
// Legacy single-profile files (the pre-profiles shape: top-level base_url /
// org_id / access_token keys) are migrated in place on first read: the
// original bytes are preserved at credentials.yaml.bak and the file is
// rewritten as a store with the old credentials under the "default" profile.
// The migration runs at most once — after it, the file has a `profiles` key
// and is loaded as-is.
func LoadStore() (*CredentialsStore, error) {
	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &CredentialsStore{Version: storeVersion, Profiles: map[string]*Credentials{}}, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	// Shape detection: a multi-profile file has a top-level `profiles` key.
	var probe map[string]interface{}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	if _, isStore := probe["profiles"]; isStore {
		var store CredentialsStore
		if err := yaml.Unmarshal(data, &store); err != nil {
			return nil, fmt.Errorf("parse credentials: %w", err)
		}
		if store.Profiles == nil {
			store.Profiles = map[string]*Credentials{}
		}
		if store.Version == 0 {
			store.Version = storeVersion
		}
		return &store, nil
	}

	// Empty file → empty store, nothing to migrate.
	if len(probe) == 0 {
		return &CredentialsStore{Version: storeVersion, Profiles: map[string]*Credentials{}}, nil
	}

	// Legacy single-profile shape — migrate in place.
	var legacy Credentials
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	store := &CredentialsStore{
		Version:        storeVersion,
		CurrentProfile: DefaultProfile,
		Profiles:       map[string]*Credentials{DefaultProfile: &legacy},
	}

	// Preserve the original file before rewriting. Don't clobber an existing
	// backup (e.g. from a previous migration the user rolled back from).
	backup := CredentialsBackupPath()
	if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
		if err := os.WriteFile(backup, data, 0o600); err != nil {
			return nil, fmt.Errorf("back up credentials before migration: %w", err)
		}
	}
	if err := SaveStore(store); err != nil {
		return nil, fmt.Errorf("migrate credentials to profiles: %w", err)
	}
	return store, nil
}

// SaveStore writes the multi-profile credentials file with 0600 perms.
func SaveStore(s *CredentialsStore) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if s.Version == 0 {
		s.Version = storeVersion
	}
	if s.Profiles == nil {
		s.Profiles = map[string]*Credentials{}
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.WriteFile(CredentialsPath(), data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

// activeProfileName resolves which profile is in effect for the given store.
// Precedence: --profile flag > LUMO_PROFILE env > current_profile > "default".
func activeProfileName(s *CredentialsStore) string {
	if profileOverride != "" {
		return profileOverride
	}
	if v := os.Getenv(EnvProfile); v != "" {
		return v
	}
	if s != nil && s.CurrentProfile != "" {
		return s.CurrentProfile
	}
	return DefaultProfile
}

// ActiveProfileName resolves the active profile name, loading the store to
// consult current_profile. Errors reading the store fall back to defaults —
// callers that need the error should use LoadStore directly.
func ActiveProfileName() string {
	store, err := LoadStore()
	if err != nil {
		store = nil
	}
	return activeProfileName(store)
}

// LoadCredentials returns the credentials of the active profile. Returns
// nil, nil when the profile doesn't exist or stores no credentials — callers
// should fall back to API-key auth.
func LoadCredentials() (*Credentials, error) {
	store, err := LoadStore()
	if err != nil {
		return nil, err
	}
	name := activeProfileName(store)
	c := store.Profiles[name]
	if c == nil {
		return nil, nil
	}
	c.profile = name
	return c, nil
}

// Save writes the credentials back into the profile they were loaded from
// (or the active profile for in-memory credentials), preserving all other
// profiles in the file.
func (c *Credentials) Save() error {
	name := c.profile
	if name == "" {
		name = ActiveProfileName()
	}
	return SaveProfile(name, c, false)
}

// SaveProfile stores credentials under the named profile. When makeCurrent
// is true (e.g. after `lumo login`), current_profile is switched to it.
func SaveProfile(name string, c *Credentials, makeCurrent bool) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	store, err := LoadStore()
	if err != nil {
		return err
	}
	c.profile = name
	store.Profiles[name] = c
	if makeCurrent || store.CurrentProfile == "" {
		store.CurrentProfile = name
	}
	return SaveStore(store)
}

// ClearCredentials removes the active profile's credentials. Idempotent.
// Returns the name of the profile that was cleared.
func ClearCredentials() (string, error) {
	store, err := LoadStore()
	if err != nil {
		return "", err
	}
	name := activeProfileName(store)
	if _, ok := store.Profiles[name]; !ok {
		return name, nil
	}
	delete(store.Profiles, name)
	if store.CurrentProfile == name {
		store.CurrentProfile = ""
		if names := store.ProfileNames(); len(names) > 0 {
			store.CurrentProfile = names[0]
		}
	}
	return name, SaveStore(store)
}

// UseProfile switches current_profile. The profile must exist.
func UseProfile(name string) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	if _, ok := store.Profiles[name]; !ok {
		return fmt.Errorf("profile %q does not exist (have: %v). Create it with 'lumo profile create %s' or 'lumo login --profile %s'",
			name, store.ProfileNames(), name, name)
	}
	store.CurrentProfile = name
	return SaveStore(store)
}

// CreateProfile adds a new, token-less profile (org/base URL context only;
// authenticate it later with `lumo login --profile <name>`).
func CreateProfile(name, orgID, baseURL string, makeCurrent bool) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	store, err := LoadStore()
	if err != nil {
		return err
	}
	if _, ok := store.Profiles[name]; ok {
		return fmt.Errorf("profile %q already exists", name)
	}
	store.Profiles[name] = &Credentials{OrgID: orgID, BaseURL: baseURL, profile: name}
	if makeCurrent || store.CurrentProfile == "" {
		store.CurrentProfile = name
	}
	return SaveStore(store)
}

// DeleteProfile removes a profile. If it was current, current_profile moves
// to the first remaining profile (alphabetically) or is unset.
func DeleteProfile(name string) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	if _, ok := store.Profiles[name]; !ok {
		return fmt.Errorf("profile %q does not exist (have: %v)", name, store.ProfileNames())
	}
	delete(store.Profiles, name)
	if store.CurrentProfile == name {
		store.CurrentProfile = ""
		if names := store.ProfileNames(); len(names) > 0 {
			store.CurrentProfile = names[0]
		}
	}
	return SaveStore(store)
}

// HasToken reports whether the profile actually holds usable credentials
// (as opposed to a token-less profile created by `lumo profile create`).
func (c *Credentials) HasToken() bool {
	return c != nil && (c.AccessToken != "" || c.RefreshToken != "")
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

// EnsureFresh refreshes the access token in place when expired and a refresh
// token is available, persisting the rotated tokens to disk. No-op when the
// token is still valid. Returns an error when there is no refresh token or
// the refresh call fails — callers should treat that as "user must re-login
// or fall back to API key".
func (c *Credentials) EnsureFresh(insecure bool) error {
	if !c.IsExpired() {
		return nil
	}
	return c.ForceRefresh(insecure)
}

// ForceRefresh exchanges the refresh token for a new access token even when
// the current one has not expired locally — used after the server answers
// 401 (revoked or clock skew). Persists the rotated tokens.
func (c *Credentials) ForceRefresh(insecure bool) error {
	if c.RefreshToken == "" {
		return fmt.Errorf("access token expired and no refresh token stored")
	}

	client := auth.New(c.BaseURL, c.OrgID, insecure || c.Insecure)
	tok, err := client.Refresh(c.RefreshToken)
	if err != nil {
		return err
	}
	c.ApplyToken(tok)
	return c.Save()
}

// ApplyToken copies a token response into the credentials (without saving).
func (c *Credentials) ApplyToken(tok *auth.TokenResponse) {
	c.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		c.RefreshToken = tok.RefreshToken
	}
	c.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if tok.TokenType != "" {
		c.TokenType = tok.TokenType
	}
	if tok.Scope != "" {
		c.Scopes = auth.SplitScopes(tok.Scope)
	}
}

// InsecureFor reports whether TLS verification should be skipped for
// requests to baseURL because this profile was set up that way. It never
// applies to a different server, so a local dev profile can't weaken TLS for
// a production URL passed with --base-url.
func (c *Credentials) InsecureFor(baseURL string) bool {
	if c == nil || !c.Insecure || c.BaseURL == "" {
		return false
	}
	return strings.TrimRight(c.BaseURL, "/") == strings.TrimRight(baseURL, "/")
}

// HasScope reports whether the token was granted the exact scope.
func (c *Credentials) HasScope(scope string) bool {
	if c == nil {
		return false
	}
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAdminAccess reports whether the token can reach the Admin API at all:
// the blanket `admin` scope or at least one `admin:<resource>:<read|write>`.
func (c *Credentials) HasAdminAccess() bool {
	if c == nil {
		return false
	}
	for _, s := range c.Scopes {
		if s == "admin" || strings.HasPrefix(s, "admin:") {
			return true
		}
	}
	return false
}
