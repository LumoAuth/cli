package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lumoauth/cli/internal/config"
)

func TestResolveOutputFormat(t *testing.T) {
	cases := []struct {
		flag, env, file string
		want            string
	}{
		{"", "", "", ""},                  // auto
		{"", "", "table", ""},             // config init writes "table": still JSON in a pipe
		{"", "TABLE", "", ""},             // same for the environment
		{"table", "", "", "table"},        // only -o forces a table into a pipe
		{"", "", "yaml", "yaml"},          // config file format is honoured
		{"", "json", "yaml", "json"},      // environment beats the config file
		{"yaml", "json", "table", "yaml"}, // the flag beats both
		{"", " ", "json", "json"},
	}
	for _, tc := range cases {
		if got := resolveOutputFormat(tc.flag, tc.env, tc.file); got != tc.want {
			t.Errorf("resolveOutputFormat(%q, %q, %q) = %q, want %q", tc.flag, tc.env, tc.file, got, tc.want)
		}
	}
}

func TestOutputIsJSONWhenPipedWithoutAFormat(t *testing.T) {
	// runAgainstFakeServer captures stdout through a pipe, like `lumo … | jq`.
	out, _, err := runAgainstFakeServer(t, 200, `{"data":[{"id":"r1","name":"Editor","slug":"editor"}]}`, "roles", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var v map[string]interface{}
	if json.Unmarshal([]byte(out), &v) != nil {
		t.Fatalf("piped output is not JSON:\n%s", out)
	}
}

func TestConfigFileFormatIsUsed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUMO_CONFIG_DIR", dir)
	t.Setenv("LUMO_OUTPUT_FORMAT", "")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("format: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if config.ConfigPath() != filepath.Join(dir, "config.yaml") {
		t.Fatalf("config path = %s", config.ConfigPath())
	}
	if p := getPrinter(); p.IsJSON() || p.IsTable() {
		t.Fatalf("format: yaml in config.yaml was ignored")
	}
}

func TestRolesUpdatePermissionsUseThePermissionsEndpoint(t *testing.T) {
	resp := `{"data":{"id":"r1","slug":"editor","permissions":[{"id":1,"slug":"doc.edit"}]}}`
	_, reqs, err := runAgainstFakeServer(t, 200, resp, "roles", "update", "editor", "--permissions", "doc.edit,doc.view")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "PUT", adminBase+"/roles/editor/permissions")
	got, _ := req.Body["permissions"].([]interface{})
	if len(got) != 2 || got[0] != "doc.edit" || got[1] != "doc.view" {
		t.Fatalf("permissions body = %v", req.Body["permissions"])
	}
}

func TestRolesUpdateNameAndPermissionsSendsTwoRequests(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 200, `{"data":{"id":"r1"}}`, "roles", "update", "editor", "--name", "Editors", "--permissions", "doc.edit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Method != "PATCH" || reqs[0].Path != adminBase+"/roles/editor" {
		t.Fatalf("first request = %s %s", reqs[0].Method, reqs[0].Path)
	}
	if _, ok := reqs[0].Body["permissions"]; ok {
		t.Fatalf("PATCH must not carry permissions (the server ignores them there)")
	}
	if reqs[1].Method != "PUT" || reqs[1].Path != adminBase+"/roles/editor/permissions" {
		t.Fatalf("second request = %s %s", reqs[1].Method, reqs[1].Path)
	}
}

func TestSkippedPermissions(t *testing.T) {
	resp := []byte(`{"data":{"permissions":[{"slug":"doc.edit"},{"slug":"kept.by.server"}]}}`)
	got := skippedPermissions([]string{"doc.edit", "doc.view", " "}, resp)
	if !reflect.DeepEqual(got, []string{"doc.view"}) {
		t.Fatalf("skipped = %v, want [doc.view]", got)
	}
	if got := skippedPermissions([]string{"doc.edit"}, []byte(`{"data":{"id":"r1"}}`)); got != nil {
		t.Fatalf("no permission list in the response: want no warning, got %v", got)
	}
}

func TestGroupsMembersRemoveDeletesEachUser(t *testing.T) {
	out, reqs, err := runAgainstFakeServer(t, 200, `{"message":"Member removed from group"}`,
		"groups", "members", "remove", "eng", "--users", "u1,jane@acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{adminBase + "/groups/eng/members/u1", adminBase + "/groups/eng/members/jane@acme.com"}
	if len(reqs) != len(want) {
		t.Fatalf("expected %d requests, got %d: %+v", len(want), len(reqs), reqs)
	}
	for i, r := range reqs {
		if r.Method != "DELETE" || r.Path != want[i] {
			t.Fatalf("request %d = %s %s, want DELETE %s", i, r.Method, r.Path, want[i])
		}
	}
	if !strings.Contains(out, "jane@acme.com") {
		t.Fatalf("output does not list the removed users: %s", out)
	}
}

func TestSocialCreateSendsTheFieldsThatTheAPIReads(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 201, `{"data":{"id":"p1"}}`,
		"social", "create", "--provider", "google", "--client-id", "cid", "--client-secret", "sec", "--scopes", "openid,email", "--enable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "POST", adminBase+"/social-providers")
	if req.Body["displayName"] != "Google" {
		t.Fatalf("displayName = %v, want Google (required by the API)", req.Body["displayName"])
	}
	if req.Body["enabled"] != true {
		t.Fatalf("enabled = %v, want true with --enable", req.Body["enabled"])
	}
	scopes, _ := req.Body["requestedScopes"].([]interface{})
	if len(scopes) != 2 || scopes[0] != "openid" || scopes[1] != "email" {
		t.Fatalf("requestedScopes = %v", req.Body["requestedScopes"])
	}
	for _, k := range []string{"name", "scopes"} {
		if _, ok := req.Body[k]; ok {
			t.Fatalf("body still carries %q, which the API ignores", k)
		}
	}
}

func TestSocialCreateUsesTheNameFlagAndStaysDisabledByDefault(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 201, `{"data":{"id":"p1"}}`,
		"social", "create", "--provider", "github", "--client-id", "cid", "--client-secret", "sec", "--name", "GitHub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "POST", adminBase+"/social-providers")
	if req.Body["displayName"] != "GitHub" || req.Body["enabled"] != false {
		t.Fatalf("body = %v", req.Body)
	}
}

func TestDefaultProviderDisplayName(t *testing.T) {
	for in, want := range map[string]string{"google": "Google", "microsoft-entra": "Microsoft Entra", "apple_id": "Apple Id"} {
		if got := defaultProviderDisplayName(in); got != want {
			t.Errorf("defaultProviderDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWebhooksCreateShowsTheSigningSecretAndSendsNoIgnoredFields(t *testing.T) {
	resp := `{"data":{"id":42,"url":"https://example.com/hook","secret":"abc123secret","_note":"Store the secret"},"message":"Webhook created successfully"}`
	out, reqs, err := runAgainstFakeServer(t, 201, resp,
		"webhooks", "create", "--url", "https://example.com/hook", "--events", "user.created", "--secret", "mine", "--description", "d", "-o", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "POST", adminBase+"/webhooks")
	for _, k := range []string{"secret", "description"} {
		if _, ok := req.Body[k]; ok {
			t.Fatalf("body carries %q, which the API ignores", k)
		}
	}
	if !strings.Contains(out, "abc123secret") {
		t.Fatalf("table output drops the one-time signing secret:\n%s", out)
	}
}
