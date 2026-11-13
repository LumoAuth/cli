package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

const sandboxSlug = "sandbox-pr-1-abc123"

func TestSandboxKeyIsStoredAsAProfileAndFoundByOrg(t *testing.T) {
	setupDir(t)
	if err := SaveProfile("default", &Credentials{BaseURL: "https://app.lumoauth.dev", OrgID: "acme", AccessToken: "at", ExpiresAt: time.Now().Add(time.Hour)}, true); err != nil {
		t.Fatal(err)
	}
	if err := SaveSandboxKey(sandboxSlug, "https://app.lumoauth.dev/", "lmk_sandbox", time.Now().Add(time.Hour), false); err != nil {
		t.Fatalf("SaveSandboxKey: %v", err)
	}

	store, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if store.CurrentProfile != "default" {
		t.Errorf("current_profile = %q: saving a sandbox key must not switch the active login", store.CurrentProfile)
	}
	c := store.Profiles[sandboxSlug]
	if c == nil || !c.IsSandboxKey() || c.OrgID != sandboxSlug || c.APIKey != "lmk_sandbox" {
		t.Fatalf("sandbox profile wrong: %+v", c)
	}
	if info, err := os.Stat(CredentialsPath()); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("credentials file must be 0600, got %v (%v)", info.Mode().Perm(), err)
	}

	if got := OrgAPIKey(sandboxSlug, "https://app.lumoauth.dev"); got != "lmk_sandbox" {
		t.Errorf("OrgAPIKey = %q, want the sandbox key", got)
	}
	if got := OrgAPIKey("acme", "https://app.lumoauth.dev"); got != "" {
		t.Errorf("the owner organization must not get the sandbox key, got %q", got)
	}
	if got := OrgAPIKey(sandboxSlug, "https://evil.example"); got != "" {
		t.Errorf("the key must only go to the server that issued it, got %q", got)
	}
}

func TestSandboxKeyReplacesAConfiguredKeyOnlyForTheSandbox(t *testing.T) {
	setupDir(t)
	t.Setenv("LUMO_API_KEY", "lmk_owner")
	t.Setenv("LUMO_BASE_URL", "https://app.lumoauth.dev")
	if err := SaveSandboxKey(sandboxSlug, "https://app.lumoauth.dev", "lmk_sandbox", time.Now().Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}

	sandbox, err := Load("", sandboxSlug, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := sandbox.Validate(); err != nil {
		t.Fatalf("Validate for the sandbox: %v", err)
	}
	if sandbox.APIKey != "lmk_sandbox" {
		t.Errorf("--org <sandbox> used %q, want the sandbox key", sandbox.APIKey)
	}

	owner, err := Load("", "acme", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Validate(); err != nil {
		t.Fatalf("Validate for the owner: %v", err)
	}
	if owner.APIKey != "lmk_owner" {
		t.Errorf("--org acme used %q, want the configured key", owner.APIKey)
	}
}

func TestSandboxKeyAloneAuthenticates(t *testing.T) {
	setupDir(t)
	t.Setenv("LUMO_API_KEY", "")
	t.Setenv("LUMO_BASE_URL", "https://app.lumoauth.dev")
	if err := SaveSandboxKey(sandboxSlug, "https://app.lumoauth.dev", "lmk_sandbox", time.Now().Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("", sandboxSlug, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a stored sandbox key must be enough to authenticate: %v", err)
	}

	// --profile <slug> selects the same sandbox without --org.
	SetProfileOverride(sandboxSlug)
	byProfile, err := Load("", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := byProfile.Validate(); err != nil {
		t.Fatalf("Validate with --profile: %v", err)
	}
	if byProfile.OrgID != sandboxSlug || byProfile.APIKey != "lmk_sandbox" {
		t.Errorf("--profile <sandbox> resolved org %q key %q", byProfile.OrgID, byProfile.APIKey)
	}
}

func TestExpiredSandboxKeyIsNotUsedAndIsPruned(t *testing.T) {
	setupDir(t)
	if err := SaveSandboxKey("sandbox-old-000000", "https://app.lumoauth.dev", "lmk_old", time.Now().Add(-time.Minute), false); err != nil {
		t.Fatal(err)
	}
	if got := OrgAPIKey("sandbox-old-000000", "https://app.lumoauth.dev"); got != "" {
		t.Errorf("an ended sandbox's key must not be sent, got %q", got)
	}
	// The next save drops it.
	if err := SaveSandboxKey(sandboxSlug, "https://app.lumoauth.dev", "lmk_sandbox", time.Now().Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}
	store, _ := LoadStore()
	if _, ok := store.Profiles["sandbox-old-000000"]; ok {
		t.Error("the ended sandbox's profile should have been pruned")
	}
	raw, _ := os.ReadFile(CredentialsPath())
	if strings.Contains(string(raw), "lmk_old") {
		t.Error("the ended sandbox's key is still on disk")
	}
}

func TestDeleteSandboxKeyLeavesOtherProfilesAlone(t *testing.T) {
	setupDir(t)
	if err := SaveProfile("mine", &Credentials{BaseURL: "https://app.lumoauth.dev", OrgID: "acme", AccessToken: "at"}, true); err != nil {
		t.Fatal(err)
	}
	if err := SaveSandboxKey(sandboxSlug, "https://app.lumoauth.dev", "lmk_sandbox", time.Now().Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}

	if found, err := DeleteSandboxKey("mine"); err != nil || found {
		t.Fatalf("DeleteSandboxKey on a login profile = %v, %v; want it untouched", found, err)
	}
	if found, err := DeleteSandboxKey(sandboxSlug); err != nil || !found {
		t.Fatalf("DeleteSandboxKey = %v, %v", found, err)
	}
	store, _ := LoadStore()
	if _, ok := store.Profiles[sandboxSlug]; ok {
		t.Error("sandbox profile still stored")
	}
	if c := store.Profiles["mine"]; c == nil || c.AccessToken != "at" || store.CurrentProfile != "mine" {
		t.Errorf("login profile changed: %+v (current %q)", c, store.CurrentProfile)
	}
	if got := OrgAPIKey(sandboxSlug, "https://app.lumoauth.dev"); got != "" {
		t.Errorf("key still resolves after delete: %q", got)
	}

	// A login profile that happens to carry the sandbox's name is never overwritten.
	if err := SaveProfile(sandboxSlug, &Credentials{BaseURL: "https://app.lumoauth.dev", OrgID: sandboxSlug, AccessToken: "at2"}, false); err != nil {
		t.Fatal(err)
	}
	if err := SaveSandboxKey(sandboxSlug, "https://app.lumoauth.dev", "lmk_again", time.Now().Add(time.Hour), false); err == nil {
		t.Error("SaveSandboxKey must refuse to replace a login profile")
	}
}
