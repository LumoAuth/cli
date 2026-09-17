package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lumoauth/cli/internal/config"
)

// TestDoctorHappyPathWithAPIKey drives the real command tree against a fake
// server: an API key, a reachable discovery document and an admin probe.
func TestDoctorHappyPathWithAPIKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUMO_CONFIG_DIR", dir)
	t.Setenv(config.EnvProfile, "")
	config.SetProfileOverride("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": "http://" + r.Host + "/orgs/acme/api/v1"})
		case strings.HasSuffix(r.URL.Path, "/admin/organization"):
			if r.Header.Get("X-API-Key") != "lmk_test" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"slug":"acme"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	t.Setenv("LUMO_API_KEY", "lmk_test")
	t.Setenv("LUMO_ORG", "acme")
	t.Setenv("LUMO_BASE_URL", srv.URL)

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rootCmd.SetArgs([]string{"doctor", "-o", "json"})
	execErr := rootCmd.Execute()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	rootCmd.SetArgs(nil)
	flagFormat = ""

	if execErr != nil {
		t.Fatalf("doctor failed: %v\n%s", execErr, buf.String())
	}
	var out struct {
		OK     bool          `json:"ok"`
		Checks []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, buf.String())
	}
	if !out.OK {
		t.Fatalf("expected ok, got %s", buf.String())
	}
	names := map[string]string{}
	for _, c := range out.Checks {
		names[c.Name] = c.Status
	}
	for _, n := range []string{"profile", "organization", "server", "credential", "discovery", "admin api"} {
		if names[n] != "ok" {
			t.Fatalf("check %q = %q; all: %s", n, names[n], buf.String())
		}
	}
}
