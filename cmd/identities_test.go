package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestUsersIdentitiesList(t *testing.T) {
	resp := `{"data":{"auth_source":"saml","ldap_only":false,"bindings":[{"type":"saml","idp_id":3,"idp_name":"Okta","name_id":"jane@acme.test","hashed":false,"legacy":false,"orphaned":false}]}}`
	out, reqs, err := runAgainstFakeServer(t, 200, resp, "users", "identities", "list", "jane@acme.test", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	expectOneRequest(t, reqs, "GET", adminBase+"/users/jane@acme.test/identities")
	if !strings.Contains(out, `"Okta"`) {
		t.Fatalf("JSON output should pass the response through, got:\n%s", out)
	}
}

func TestIdentityBindingRow(t *testing.T) {
	name, id, nameID := "Okta", 3, "jane@acme.test"
	if got := (identityBinding{Type: "saml", IdpID: &id, IdpName: &name, NameID: &nameID}).row(); !reflect.DeepEqual(got, []string{"saml", "Okta (#3)", "jane@acme.test", ""}) {
		t.Fatalf("saml row = %v", got)
	}
	legacy := identityBinding{Type: "saml", NameID: &nameID, Legacy: true}.row()
	if !strings.Contains(legacy[3], "legacy-saml") {
		t.Fatalf("legacy row should point at the relink command, got %v", legacy)
	}
	dn, cfg := "uid=jane,dc=acme", 1
	if got := (identityBinding{Type: "ldap", LdapConfigID: &cfg, DN: &dn, LdapOnly: true}).row(); got[1] != "directory #1" || got[2] != dn || !strings.Contains(got[3], "LDAP only") {
		t.Fatalf("ldap row = %v", got)
	}
}

func TestUsersIdentitiesLinkSaml(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 201, `{"data":{"type":"saml","idp_id":3,"name_id":"jane@acme.test"}}`,
		"users", "identities", "link", "u-1", "--saml-idp", "3", "--name-id", "jane@acme.test", "--mfa-challenge", "ch_1", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "POST", adminBase+"/users/u-1/identities")
	want := map[string]interface{}{"type": "saml", "idp_id": float64(3), "name_id": "jane@acme.test"}
	if !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %v, want %v", r.Body, want)
	}
	if r.Header.Get("X-MFA-Challenge") != "ch_1" {
		t.Fatalf("X-MFA-Challenge header = %q", r.Header.Get("X-MFA-Challenge"))
	}
}

func TestUsersIdentitiesLinkLdap(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 201, `{"data":{"type":"ldap"}}`,
		"users", "identities", "link", "u-1", "--ldap-config", "1", "--dn", "uid=jane,dc=acme", "--ldap-only", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "POST", adminBase+"/users/u-1/identities")
	want := map[string]interface{}{"type": "ldap", "ldap_config_id": float64(1), "dn": "uid=jane,dc=acme", "ldap_only": true}
	if !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %v, want %v", r.Body, want)
	}

	// Without --dn the server looks the entry up; no dn key is sent.
	_, reqs, err = runAgainstFakeServer(t, 201, `{"data":{"type":"ldap"}}`, "users", "identities", "link", "u-1", "--ldap-config", "1", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r = expectOneRequest(t, reqs, "POST", adminBase+"/users/u-1/identities")
	if want := map[string]interface{}{"type": "ldap", "ldap_config_id": float64(1)}; !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %v, want %v", r.Body, want)
	}
}

func TestUsersIdentitiesLinkValidatesLocally(t *testing.T) {
	cases := [][]string{
		{"users", "identities", "link", "u-1"},
		{"users", "identities", "link", "u-1", "--saml-idp", "3"},
		{"users", "identities", "link", "u-1", "--name-id", "x"},
		{"users", "identities", "link", "u-1", "--dn", "uid=x"},
		{"users", "identities", "link", "u-1", "--saml-idp", "3", "--name-id", "x", "--ldap-config", "1"},
	}
	for _, args := range cases {
		_, reqs, err := runAgainstFakeServer(t, 201, `{}`, args...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("%v: expected a local error and no request, got err=%v reqs=%d", args, err, len(reqs))
		}
	}
}

func TestUsersIdentitiesLinkConflictHint(t *testing.T) {
	_, _, err := runAgainstFakeServer(t, 409, `{"error":"identity_conflict","message":"Another user in this organization is already linked to that NameID at this identity provider."}`,
		"users", "identities", "link", "u-1", "--saml-idp", "3", "--name-id", "x")
	if err == nil || !strings.Contains(err.Error(), "409") {
		t.Fatalf("expected an HTTP 409 error, got %v", err)
	}
}

func TestUsersIdentitiesUnlink(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 200, `{"data":{"type":"ldap"}}`, "users", "identities", "unlink", "u-1", "--type", "ldap", "--yes", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "DELETE", adminBase+"/users/u-1/identities/ldap")
	if r.Body != nil {
		t.Fatalf("DELETE should have no body, got %v", r.Body)
	}

	// No TTY and no --yes: refuse without calling the server; bad type is local.
	for _, args := range [][]string{
		{"users", "identities", "unlink", "u-1", "--type", "saml"},
		{"users", "identities", "unlink", "u-1", "--type", "kerberos", "--yes"},
		{"users", "identities", "unlink", "u-1", "--yes"},
	} {
		_, reqs, err = runAgainstFakeServer(t, 200, `{}`, args...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("%v: expected a local error and no request, got err=%v reqs=%d", args, err, len(reqs))
		}
	}
}

func TestIdentitiesLegacySamlReport(t *testing.T) {
	resp := `{"data":{"idp_count":2,"ambiguous":true,"users":[{"user_id":"u-1","email":"a@acme.test","name_id":"a@acme.test","candidate_idp_ids":[3],"suggested_idp_id":3}],"idps":[{"id":3,"display_name":"Okta","allowed_email_domains":["acme.test"]}]}}`
	_, reqs, err := runAgainstFakeServer(t, 200, resp, "identities", "legacy-saml", "--idp", "3", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	expectOneRequest(t, reqs, "GET", adminBase+"/identities/legacy-saml")

	var headers []string
	var rows [][]string
	var notes []string
	if err := printLegacySamlReport([]byte(resp), func(h []string, r [][]string) { headers, rows = h, r }, func(n string) { notes = append(notes, n) }); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][3] != "Okta (#3)" || headers[3] != "Suggested IdP" {
		t.Fatalf("rows = %v", rows)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "cannot sign in") {
		t.Fatalf("notes = %v", notes)
	}
}

func TestIdentitiesLegacySamlRelink(t *testing.T) {
	resp := `{"data":{"dry_run":true,"idp_id":3,"relinked":[{"user_id":"u-1","email":"a@acme.test","name_id":"a@acme.test"}],"skipped":[]}}`
	_, reqs, err := runAgainstFakeServer(t, 200, resp, "identities", "legacy-saml", "--idp", "3", "--relink", "--dry-run", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r := expectOneRequest(t, reqs, "POST", adminBase+"/identities/legacy-saml")
	if want := map[string]interface{}{"idp_id": float64(3), "dry_run": true, "all_matching_domains": true}; !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %v, want %v", r.Body, want)
	}

	_, reqs, err = runAgainstFakeServer(t, 200, resp, "identities", "legacy-saml", "--idp", "3", "--relink", "--user", "u-1", "--user", "u-2", "--yes", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	r = expectOneRequest(t, reqs, "POST", adminBase+"/identities/legacy-saml")
	if want := map[string]interface{}{"idp_id": float64(3), "dry_run": false, "user_ids": []interface{}{"u-1", "u-2"}}; !reflect.DeepEqual(r.Body, want) {
		t.Fatalf("body = %v, want %v", r.Body, want)
	}

	// A real run needs --yes without a TTY; --relink needs --idp; flags need --relink.
	for _, args := range [][]string{
		{"identities", "legacy-saml", "--idp", "3", "--relink"},
		{"identities", "legacy-saml", "--relink", "--dry-run"},
		{"identities", "legacy-saml", "--dry-run"},
	} {
		_, reqs, err = runAgainstFakeServer(t, 200, resp, args...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("%v: expected a local error and no request, got err=%v reqs=%d", args, err, len(reqs))
		}
	}

	var rows [][]string
	var notes []string
	if err := printLegacySamlRelink([]byte(resp), func(_ []string, r [][]string) { rows = r }, func(string) { t.Fatal("dry run must not print success") }, func(n string) { notes = append(notes, n) }); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][3] != "would relink" || !strings.Contains(notes[0], "Dry run") {
		t.Fatalf("rows=%v notes=%v", rows, notes)
	}
}
