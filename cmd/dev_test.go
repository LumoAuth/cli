package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lumoauth/cli/internal/config"
)

const (
	devSandboxSlug = "sandbox-pr-7-abc123"
	devSandboxKey  = "lmk_sandbox_0123456789abcdef0123456789abcdef"
	devOwnerKey    = "lmk_owner_key"
)

type devRequest struct {
	Method, Path, APIKey string
}

// runDevFlow runs several `lumo` commands against one fake server and one
// config directory, the way a person (or a CI job) uses a sandbox. The fake
// accepts the owner's key for acme and the sandbox's key for the sandbox.
func runDevFlow(t *testing.T, commands ...[]string) (outputs []string, reqs []devRequest) {
	t.Helper()
	t.Setenv("LUMO_CONFIG_DIR", t.TempDir())
	t.Setenv(config.EnvProfile, "")
	config.SetProfileOverride("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		key := r.Header.Get("X-API-Key")
		reqs = append(reqs, devRequest{Method: r.Method, Path: r.URL.Path, APIKey: key})
		w.Header().Set("Content-Type", "application/json")
		want := devOwnerKey
		if strings.HasPrefix(r.URL.Path, "/orgs/"+devSandboxSlug+"/") {
			want = devSandboxKey
		}
		switch {
		case key != want:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
		case r.URL.Path == "/orgs/acme/api/v1/admin/sandbox/spawn":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"slug":"` + devSandboxSlug + `","name":"Sandbox: pr-7","created_at":"2026-11-13T10:00:00+00:00","expires_at":"2099-01-01T00:00:00+00:00","api_key":"` + devSandboxKey + `"}}`))
		case strings.HasSuffix(r.URL.Path, "/destroy"):
			_, _ = w.Write([]byte(`{"message":"Sandbox destroyed"}`))
		default:
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()

	t.Setenv("LUMO_API_KEY", devOwnerKey)
	t.Setenv("LUMO_ORG", "acme")
	t.Setenv("LUMO_BASE_URL", srv.URL)

	oldIn, oldOut, oldErr := os.Stdin, os.Stdout, os.Stderr
	defer func() { os.Stdin, os.Stdout, os.Stderr = oldIn, oldOut, oldErr }()
	for _, args := range commands {
		inR, inW, _ := os.Pipe()
		inW.Close()
		os.Stdin = inR
		r, w, _ := os.Pipe()
		os.Stdout, os.Stderr = w, w
		resetCommandFlags(rootCmd)
		rootCmd.SetArgs(args)
		err := rootCmd.Execute()
		w.Close()
		os.Stdout, os.Stderr = oldOut, oldErr
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		inR.Close()
		rootCmd.SetArgs(nil)
		resetCommandFlags(rootCmd)
		flagFormat, flagOrgID = "", ""
		if err != nil {
			buf.WriteString("\nERROR: " + err.Error())
		}
		outputs = append(outputs, buf.String())
	}
	return outputs, reqs
}

func TestDevStartSavesTheSandboxKeyAndFollowUpCommandsUseIt(t *testing.T) {
	outputs, reqs := runDevFlow(t,
		[]string{"dev", "start", "--name", "pr-7", "-o", "table"},
		[]string{"--org", devSandboxSlug, "users", "list"},
		[]string{"users", "list"},
		[]string{"dev", "stop", devSandboxSlug},
		[]string{"--org", devSandboxSlug, "users", "list"},
	)

	if strings.Contains(outputs[0], devSandboxKey) || strings.Contains(outputs[0], "lmk_") {
		t.Errorf("the table output of dev start must not print the key:\n%s", outputs[0])
	}
	for _, want := range []string{"Sandbox spawned", "saved as profile", "lumo --org " + devSandboxSlug + " users list"} {
		if !strings.Contains(outputs[0], want) {
			t.Errorf("dev start output lacks %q:\n%s", want, outputs[0])
		}
	}
	if len(reqs) != 5 {
		t.Fatalf("expected 5 requests, got %+v", reqs)
	}
	// 1: spawn with the owner's credential.
	if reqs[0].Path != "/orgs/acme/api/v1/admin/sandbox/spawn" || reqs[0].APIKey != devOwnerKey {
		t.Errorf("spawn request wrong: %+v", reqs[0])
	}
	// 2: the printed follow-up command reaches the sandbox with the sandbox's key.
	if reqs[1].Path != "/orgs/"+devSandboxSlug+"/api/v1/admin/users" || reqs[1].APIKey != devSandboxKey {
		t.Errorf("follow-up command did not use the sandbox key: %+v", reqs[1])
	}
	if strings.Contains(outputs[1], "ERROR") {
		t.Errorf("follow-up command failed:\n%s", outputs[1])
	}
	// 3: the owner organization still gets the owner's key, never the sandbox's.
	if reqs[2].Path != "/orgs/acme/api/v1/admin/users" || reqs[2].APIKey != devOwnerKey {
		t.Errorf("owner request wrong: %+v", reqs[2])
	}
	// 4: stop goes to the owner organization and removes the saved key.
	if reqs[3].Path != "/orgs/acme/api/v1/admin/sandbox/"+devSandboxSlug+"/destroy" || reqs[3].APIKey != devOwnerKey {
		t.Errorf("destroy request wrong: %+v", reqs[3])
	}
	// 5: after stop the sandbox key is gone from the CLI.
	if reqs[4].APIKey == devSandboxKey {
		t.Errorf("the sandbox key was still sent after dev stop: %+v", reqs[4])
	}
	raw, _ := os.ReadFile(config.CredentialsPath())
	if strings.Contains(string(raw), devSandboxKey) {
		t.Error("the sandbox key is still on disk after dev stop")
	}
}

func TestDevStartJSONOutputCarriesTheKeyOnceAndStillSavesIt(t *testing.T) {
	outputs, reqs := runDevFlow(t,
		[]string{"dev", "start", "-o", "json"},
		[]string{"--org", devSandboxSlug, "apps", "list"},
	)
	if !strings.Contains(outputs[0], `"api_key"`) || !strings.Contains(outputs[0], devSandboxKey) {
		t.Errorf("-o json must pass the server's one-time answer through:\n%s", outputs[0])
	}
	if len(reqs) != 2 || reqs[1].APIKey != devSandboxKey {
		t.Fatalf("the key was not saved for --org: %+v", reqs)
	}
	if got := config.OrgAPIKey(devSandboxSlug, os.Getenv("LUMO_BASE_URL")); got != devSandboxKey {
		t.Errorf("stored key = %q", got)
	}
}
