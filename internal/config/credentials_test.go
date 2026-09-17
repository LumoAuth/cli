package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// setupDir points LUMO_CONFIG_DIR at a fresh temp dir and clears any
// profile override / env leakage between tests.
func setupDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LUMO_CONFIG_DIR", dir)
	t.Setenv(EnvProfile, "")
	SetProfileOverride("")
	t.Cleanup(func() { SetProfileOverride("") })
	return dir
}

const legacyYAML = `base_url: https://app.lumoauth.dev
org_id: acme-corp
user_email: jane@acme.example
access_token: at-legacy
refresh_token: rt-legacy
expires_at: 2027-01-02T15:04:05Z
token_type: Bearer
`

func writeLegacyFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, CredentialsFile), []byte(legacyYAML), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationFromLegacyShape(t *testing.T) {
	dir := setupDir(t)
	writeLegacyFile(t, dir)

	store, err := LoadStore()
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.CurrentProfile != DefaultProfile {
		t.Errorf("current_profile = %q, want %q", store.CurrentProfile, DefaultProfile)
	}
	c := store.Profiles[DefaultProfile]
	if c == nil {
		t.Fatal("default profile missing after migration")
	}
	if c.OrgID != "acme-corp" || c.AccessToken != "at-legacy" || c.RefreshToken != "rt-legacy" {
		t.Errorf("migrated credentials wrong: %+v", c)
	}
	if c.UserEmail != "jane@acme.example" || c.TokenType != "Bearer" {
		t.Errorf("migrated metadata wrong: %+v", c)
	}
	if want := time.Date(2027, 1, 2, 15, 4, 5, 0, time.UTC); !c.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %v, want %v", c.ExpiresAt, want)
	}

	// Backup preserves the original bytes.
	backup, err := os.ReadFile(filepath.Join(dir, CredentialsBackupFile))
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(backup) != legacyYAML {
		t.Errorf("backup content differs from original")
	}

	// The file on disk is now multi-profile.
	data, err := os.ReadFile(filepath.Join(dir, CredentialsFile))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]interface{}
	if err := yaml.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if _, ok := onDisk["profiles"]; !ok {
		t.Errorf("file was not rewritten in multi-profile shape: %s", data)
	}
}

func TestMigrationIsIdempotent(t *testing.T) {
	dir := setupDir(t)
	writeLegacyFile(t, dir)

	if _, err := LoadStore(); err != nil {
		t.Fatal(err)
	}
	afterFirst, err := os.ReadFile(filepath.Join(dir, CredentialsFile))
	if err != nil {
		t.Fatal(err)
	}
	backupAfterFirst, err := os.ReadFile(filepath.Join(dir, CredentialsBackupFile))
	if err != nil {
		t.Fatal(err)
	}

	// Second load: no re-migration, no backup clobbering, same content.
	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if store.Profiles[DefaultProfile].AccessToken != "at-legacy" {
		t.Error("credentials lost on second load")
	}
	afterSecond, err := os.ReadFile(filepath.Join(dir, CredentialsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterFirst) != string(afterSecond) {
		t.Error("file changed on second load — migration not idempotent")
	}
	backupAfterSecond, err := os.ReadFile(filepath.Join(dir, CredentialsBackupFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(backupAfterFirst) != string(backupAfterSecond) {
		t.Error("backup rewritten on second load")
	}
}

func TestMigrationLegacyLoadCredentialsKeepsWorking(t *testing.T) {
	dir := setupDir(t)
	writeLegacyFile(t, dir)

	// The old public API on a legacy file must return the same credentials.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil || creds.AccessToken != "at-legacy" || creds.OrgID != "acme-corp" {
		t.Fatalf("LoadCredentials after migration = %+v", creds)
	}
	if creds.Profile() != DefaultProfile {
		t.Errorf("profile = %q, want %q", creds.Profile(), DefaultProfile)
	}
}

func TestLoadStoreMissingAndEmptyFile(t *testing.T) {
	dir := setupDir(t)

	store, err := LoadStore()
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if len(store.Profiles) != 0 {
		t.Errorf("expected empty store, got %v", store.ProfileNames())
	}

	if err := os.WriteFile(filepath.Join(dir, CredentialsFile), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err = LoadStore()
	if err != nil {
		t.Fatalf("empty file: %v", err)
	}
	if len(store.Profiles) != 0 {
		t.Errorf("expected empty store from empty file, got %v", store.ProfileNames())
	}
	if _, err := os.Stat(filepath.Join(dir, CredentialsBackupFile)); !os.IsNotExist(err) {
		t.Error("empty file should not trigger a migration backup")
	}
}

func TestProfileCRUD(t *testing.T) {
	setupDir(t)

	// Create
	if err := CreateProfile("acme", "acme-corp", "https://app.lumoauth.dev", false); err != nil {
		t.Fatal(err)
	}
	if err := CreateProfile("clientb", "clientb-corp", "https://eu.app.lumoauth.dev", false); err != nil {
		t.Fatal(err)
	}
	if err := CreateProfile("acme", "", "", false); err == nil {
		t.Error("duplicate create should fail")
	}
	if err := CreateProfile("bad name!", "", "", false); err == nil {
		t.Error("invalid profile name should be rejected")
	}

	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if got := store.ProfileNames(); len(got) != 2 || got[0] != "acme" || got[1] != "clientb" {
		t.Errorf("ProfileNames = %v", got)
	}
	// First-created profile becomes current when none was set.
	if store.CurrentProfile != "acme" {
		t.Errorf("current = %q, want acme", store.CurrentProfile)
	}

	// Use
	if err := UseProfile("clientb"); err != nil {
		t.Fatal(err)
	}
	if err := UseProfile("nope"); err == nil {
		t.Error("using a missing profile should fail")
	}
	store, _ = LoadStore()
	if store.CurrentProfile != "clientb" {
		t.Errorf("current = %q, want clientb", store.CurrentProfile)
	}

	// SaveProfile writes tokens into a named slot without touching others.
	creds := &Credentials{OrgID: "clientb-corp", BaseURL: "https://eu.app.lumoauth.dev", AccessToken: "at-b"}
	if err := SaveProfile("clientb", creds, true); err != nil {
		t.Fatal(err)
	}
	store, _ = LoadStore()
	if store.Profiles["clientb"].AccessToken != "at-b" {
		t.Error("token not saved to clientb")
	}
	if store.Profiles["acme"].OrgID != "acme-corp" {
		t.Error("acme profile disturbed by clientb save")
	}

	// Delete current → current moves to remaining profile.
	if err := DeleteProfile("clientb"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteProfile("clientb"); err == nil {
		t.Error("double delete should fail")
	}
	store, _ = LoadStore()
	if store.CurrentProfile != "acme" {
		t.Errorf("current after delete = %q, want acme", store.CurrentProfile)
	}
}

func TestClearCredentialsClearsActiveProfileOnly(t *testing.T) {
	setupDir(t)
	if err := SaveProfile("acme", &Credentials{OrgID: "acme", AccessToken: "a"}, true); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile("clientb", &Credentials{OrgID: "clientb", AccessToken: "b"}, false); err != nil {
		t.Fatal(err)
	}

	name, err := ClearCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if name != "acme" {
		t.Errorf("cleared %q, want acme", name)
	}
	store, _ := LoadStore()
	if _, ok := store.Profiles["acme"]; ok {
		t.Error("acme should be gone")
	}
	if _, ok := store.Profiles["clientb"]; !ok {
		t.Error("clientb should survive logout of acme")
	}
	if store.CurrentProfile != "clientb" {
		t.Errorf("current = %q, want clientb", store.CurrentProfile)
	}
}

func TestProfilePrecedenceFlagEnvCurrent(t *testing.T) {
	setupDir(t)
	for _, p := range []string{"fromfile", "fromenv", "fromflag"} {
		if err := SaveProfile(p, &Credentials{OrgID: "org-" + p, AccessToken: "t"}, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := UseProfile("fromfile"); err != nil {
		t.Fatal(err)
	}

	// 1. current_profile only.
	if got := ActiveProfileName(); got != "fromfile" {
		t.Errorf("current_profile: got %q", got)
	}

	// 2. env beats current_profile.
	t.Setenv(EnvProfile, "fromenv")
	if got := ActiveProfileName(); got != "fromenv" {
		t.Errorf("env: got %q", got)
	}

	// 3. flag beats env.
	SetProfileOverride("fromflag")
	if got := ActiveProfileName(); got != "fromflag" {
		t.Errorf("flag: got %q", got)
	}

	// LoadCredentials follows the same resolution.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil || creds.OrgID != "org-fromflag" {
		t.Errorf("LoadCredentials resolved wrong profile: %+v", creds)
	}

	// 4. nothing set → "default".
	SetProfileOverride("")
	t.Setenv(EnvProfile, "")
	store, _ := LoadStore()
	store.CurrentProfile = ""
	if err := SaveStore(store); err != nil {
		t.Fatal(err)
	}
	if got := ActiveProfileName(); got != DefaultProfile {
		t.Errorf("fallback: got %q, want %q", got, DefaultProfile)
	}
}

func TestSaveRoundTripsThroughProfile(t *testing.T) {
	setupDir(t)
	if err := SaveProfile("acme", &Credentials{OrgID: "acme", AccessToken: "old"}, true); err != nil {
		t.Fatal(err)
	}

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	creds.AccessToken = "rotated"
	if err := creds.Save(); err != nil {
		t.Fatal(err)
	}

	store, _ := LoadStore()
	if store.Profiles["acme"].AccessToken != "rotated" {
		t.Error("Save() did not write back to the loaded profile")
	}
}

func TestCredentialsFilePermissions(t *testing.T) {
	dir := setupDir(t)
	if err := SaveProfile("acme", &Credentials{AccessToken: "secret"}, true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, CredentialsFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file perm = %o, want 600", perm)
	}
}

func TestInsecureIsScopedToTheProfileServer(t *testing.T) {
	setupDir(t)
	creds := &Credentials{OrgID: "acme-corp", BaseURL: "https://192.168.1.172:8000/", AccessToken: "at", Insecure: true}
	if err := SaveProfile("default", creds, true); err != nil {
		t.Fatal(err)
	}
	stored, _ := LoadCredentials()

	// Same server (trailing slash irrelevant): inherited.
	cfg := &Config{BaseURL: DefaultBaseURL}
	cfg.InheritFromProfile(stored)
	if cfg.BaseURL != "https://192.168.1.172:8000/" || !cfg.Insecure || cfg.OrgID != "acme-corp" {
		t.Fatalf("expected org, base URL and insecure inherited, got %+v", cfg)
	}

	// A different server pinned by the user: never inherits insecure.
	other := &Config{BaseURL: "https://app.example.com"}
	other.InheritFromProfile(stored)
	if other.Insecure {
		t.Fatal("insecure must not leak to a different server")
	}
}

func TestIsLocalURLCoversPrivateNetworks(t *testing.T) {
	for _, u := range []string{"https://192.168.1.172:8000", "https://10.0.0.5", "https://172.20.1.1:8443", "https://localhost:8000", "http://anything.example", "https://[fd00::1]:8000", "https://lumo.local"} {
		if !IsLocalURL(u) {
			t.Errorf("IsLocalURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://app.lumoauth.dev", "https://8.8.8.8", "https://172.32.0.1"} {
		if IsLocalURL(u) {
			t.Errorf("IsLocalURL(%q) = true, want false", u)
		}
	}
}
