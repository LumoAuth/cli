package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
	Header http.Header
}

// resetCommandFlags returns every flag in the tree to its default so one
// test's flags don't leak into the next rootCmd.Execute.
func resetCommandFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetCommandFlags(sub)
	}
}

// runAgainstFakeServer runs `lumo <args>` with an API key against a fake
// server that answers every request with status/response and records it.
func runAgainstFakeServer(t *testing.T, status int, response string, args ...string) (string, []recordedRequest, error) {
	t.Helper()
	t.Setenv("LUMO_CONFIG_DIR", t.TempDir())
	t.Setenv(config.EnvProfile, "")
	config.SetProfileOverride("")

	var reqs []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone()}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &rec.Body); err != nil {
				t.Errorf("request body is not a JSON object: %s", raw)
			}
		}
		reqs = append(reqs, rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	t.Setenv("LUMO_API_KEY", "lmk_test")
	t.Setenv("LUMO_ORG", "acme")
	t.Setenv("LUMO_BASE_URL", srv.URL)

	// Stdin is never a terminal here, so confirmation prompts behave as in CI.
	oldIn := os.Stdin
	inR, inW, _ := os.Pipe()
	inW.Close()
	os.Stdin = inR
	defer func() { os.Stdin = oldIn; inR.Close() }()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	resetCommandFlags(rootCmd)
	rootCmd.SetArgs(args)
	execErr := rootCmd.Execute()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	rootCmd.SetArgs(nil)
	resetCommandFlags(rootCmd)
	flagFormat = ""
	return buf.String(), reqs, execErr
}

const adminBase = "/orgs/acme/api/v1/admin"

func expectOneRequest(t *testing.T, reqs []recordedRequest, method, path string) recordedRequest {
	t.Helper()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Method != method || reqs[0].Path != path {
		t.Fatalf("got %s %s, want %s %s", reqs[0].Method, reqs[0].Path, method, path)
	}
	if reqs[0].Header.Get("X-API-Key") != "lmk_test" {
		t.Fatalf("API key header missing")
	}
	return reqs[0]
}

func TestUsersAuthenticatorsList(t *testing.T) {
	resp := `{"data":[{"id":"totp:12","type":"totp","display_name":"Authenticator app","state":"active","is_default":true,"tier":3,"tier_label":"Standard","last_used_at":null}],"status":"add_backup","default":"totp:12"}`
	out, reqs, err := runAgainstFakeServer(t, 200, resp, "users", "authenticators", "list", "jane@acme.test", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	expectOneRequest(t, reqs, "GET", adminBase+"/users/jane@acme.test/authenticators")
	if !strings.Contains(out, `"totp:12"`) || !strings.Contains(out, `"add_backup"`) {
		t.Fatalf("JSON output should pass the response through, got:\n%s", out)
	}
}

func TestUsersAuthenticatorsRemove(t *testing.T) {
	resp := `{"data":{"id":"sms_otp:3","type":"sms_otp","display_name":"Phone"},"message":"Authenticator removed"}`
	_, reqs, err := runAgainstFakeServer(t, 200, resp, "users", "authenticators", "remove", "u-1", "sms_otp:3", "--yes", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "DELETE", adminBase+"/users/u-1/authenticators/sms_otp:3")
	if r.Body != nil {
		t.Fatalf("DELETE should have no body, got %v", r.Body)
	}
}

func TestUsersAuthenticatorsRemoveNeedsConfirmation(t *testing.T) {
	// Tests run without a TTY on stdin: no --yes means refuse, and no call.
	_, reqs, err := runAgainstFakeServer(t, 200, `{}`, "users", "authenticators", "remove", "u-1", "totp:12")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected a --yes error, got %v", err)
	}
	if len(reqs) != 0 {
		t.Fatalf("no request expected without confirmation, got %+v", reqs)
	}
	// A malformed authenticator id is rejected locally.
	_, reqs, err = runAgainstFakeServer(t, 200, `{}`, "users", "authenticators", "remove", "u-1", "12", "--yes")
	if err == nil || len(reqs) != 0 {
		t.Fatalf("expected local rejection of a bare id, got err=%v reqs=%d", err, len(reqs))
	}
}

func TestUsersTap(t *testing.T) {
	resp := `{"data":{"id":"7","code":"ABCD-EFGH-JKMN","expires_at":"2026-09-24T11:00:00+00:00","single_use":false}}`
	out, reqs, err := runAgainstFakeServer(t, 201, resp,
		"users", "tap", "u-1", "--reason", "lost phone", "--ttl", "120", "--multi-use", "--mfa-challenge", "ch_1", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "POST", adminBase+"/users/u-1/temporary-access-code")
	want := map[string]interface{}{"reason": "lost phone", "ttl_minutes": float64(120), "single_use": false}
	if !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %v, want %v", r.Body, want)
	}
	if r.Header.Get("X-MFA-Challenge") != "ch_1" {
		t.Fatalf("X-MFA-Challenge header = %q", r.Header.Get("X-MFA-Challenge"))
	}
	if !strings.Contains(out, "ABCD-EFGH-JKMN") {
		t.Fatalf("code missing from output:\n%s", out)
	}

	// Defaults: single use, server-side TTL, no step-up header.
	_, reqs, err = runAgainstFakeServer(t, 201, resp, "users", "tap", "u-1", "--reason", "x", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r = expectOneRequest(t, reqs, "POST", adminBase+"/users/u-1/temporary-access-code")
	if want := map[string]interface{}{"reason": "x", "single_use": true}; !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("default body = %v, want %v", r.Body, want)
	}
	if r.Header.Get("X-MFA-Challenge") != "" {
		t.Fatalf("unexpected X-MFA-Challenge header")
	}
}

func TestUsersTapRequiresReason(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 201, `{}`, "users", "tap", "u-1")
	if err == nil || !strings.Contains(err.Error(), "--reason") {
		t.Fatalf("expected --reason error, got %v", err)
	}
	if len(reqs) != 0 {
		t.Fatalf("no request expected, got %+v", reqs)
	}
	_, reqs, err = runAgainstFakeServer(t, 201, `{}`, "users", "tap", "u-1", "--reason", "x", "--ttl", "2")
	if err == nil || len(reqs) != 0 {
		t.Fatalf("expected local --ttl range error, got err=%v reqs=%d", err, len(reqs))
	}
}

func TestPrintTemporaryAccessCode(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printTemporaryAccessCode("jane@acme.test", "ABCD-EFGH-JKMN", "2026-09-24T11:00:00Z", true)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	for _, want := range []string{"ABCD-EFGH-JKMN", "Expires:", "single use", "shown once"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestUsersMfaResetFailsLocally(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 200, `{}`, "users", "mfa-reset", "u-1")
	if err == nil {
		t.Fatal("mfa-reset should fail")
	}
	for _, want := range []string{"lumo users tap", "lumo users authenticators remove"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should point at %q: %v", want, err)
		}
	}
	if len(reqs) != 0 {
		t.Fatalf("mfa-reset must not call the server, got %+v", reqs)
	}
	if usersMfaResetCmd.Deprecated == "" {
		t.Fatal("mfa-reset should be marked deprecated")
	}
}

func TestMfaPolicyGet(t *testing.T) {
	resp := `{"data":{"requirement":"risk_based","allowed_factors":["passkey","totp"]},"effective_allowed_factors":["passkey","totp"]}`
	out, reqs, err := runAgainstFakeServer(t, 200, resp, "mfa", "policy", "get", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	expectOneRequest(t, reqs, "GET", adminBase+"/policies/mfa")
	if !strings.Contains(out, "risk_based") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestMfaPolicySetSendsOnlyChangedFlags(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 200, `{"data":{}}`,
		"mfa", "policy", "set",
		"--requirement", "required",
		"--allowed-factors", "passkey,push,totp",
		"--grace-days", "7",
		"--min-tier-step-up", "2",
		"--email-otp-counts=false",
		"--block-voip",
		"-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "PUT", adminBase+"/policies/mfa")
	want := map[string]interface{}{
		"requirement":             "required",
		"allowed_factors":         []interface{}{"passkey", "push", "totp"},
		"grace_period_days":       float64(7),
		"min_tier_step_up":        float64(2),
		"email_otp_counts_as_mfa": false,
		"sms_block_voip":          true,
	}
	if !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %#v\nwant %#v", r.Body, want)
	}

	// A single flag sends a single key.
	_, reqs, err = runAgainstFakeServer(t, 200, `{"data":{}}`, "mfa", "policy", "set", "--trusted-device-days", "0", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r = expectOneRequest(t, reqs, "PUT", adminBase+"/policies/mfa")
	if want := map[string]interface{}{"trusted_device_days": float64(0)}; !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %#v, want %#v", r.Body, want)
	}
}

func TestMfaPolicySetValidatesLocally(t *testing.T) {
	cases := [][]string{
		{},                                   // nothing to update
		{"--requirement", "sometimes"},       // bad enum
		{"--allowed-factors", "sms,passkey"}, // unknown factor
		{"--min-tier-login", "5"},            // out of range
		{"--required-factors", "3"},          // out of range
	}
	for _, extra := range cases {
		args := append([]string{"mfa", "policy", "set"}, extra...)
		_, reqs, err := runAgainstFakeServer(t, 200, `{}`, args...)
		if err == nil {
			t.Errorf("%v: expected an error", extra)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: no request expected, got %d", extra, len(reqs))
		}
	}
}

func TestMfaCoverage(t *testing.T) {
	resp := `{"data":{"total_users":50,"enrolled_users":42,"enrolled_pct":84.0,"by_type":{"totp":30,"passkey":20},"by_tier":{"1":20,"3":22},"requirement":"required","in_grace":3,"past_grace":2,"deferred":1}}`
	out, reqs, err := runAgainstFakeServer(t, 200, resp, "mfa", "coverage", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	expectOneRequest(t, reqs, "GET", adminBase+"/reports/mfa-coverage")
	if !strings.Contains(out, `"enrolled_users": 42`) {
		t.Fatalf("JSON output should pass the response through:\n%s", out)
	}
}

func TestPrintMfaCoverage(t *testing.T) {
	var got [][]string
	rec := tableRecorder(func(h []string, rows [][]string) { got = append(got, rows...) })

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printMfaCoverage(rec, mfaCoverage{
		TotalUsers: 50, EnrolledUsers: 42, EnrolledPct: 84, Requirement: "required",
		ByType: map[string]int{"totp": 30, "passkey": 20}, ByTier: map[string]int{"3": 22, "1": 20},
		InGrace: 3, PastGrace: 2,
	})
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if !strings.Contains(buf.String(), "42 of 50 users (84.0%)") || !strings.Contains(buf.String(), "Past grace:   2") {
		t.Fatalf("summary missing figures:\n%s", buf.String())
	}
	want := [][]string{{"passkey", "20"}, {"totp", "30"}, {"1 Strongest", "20"}, {"3 Standard", "22"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("table rows = %v, want %v", got, want)
	}
}

type tableRecorder func(headers []string, rows [][]string)

func (f tableRecorder) PrintTable(headers []string, rows [][]string) { f(headers, rows) }
