package cmd

import (
	"strings"
	"testing"
)

const rotateResponse = `{"data":{"agentId":"agt_1","apiKey":"agk_new","apiKeyExpiresAt":"2027-02-06T10:00:00+00:00"},"message":"ok"}`

func TestRotateCredentialsSendsTheNewKeysLifetime(t *testing.T) {
	cases := []struct {
		name  string
		flags []string
		check func(t *testing.T, body map[string]interface{})
	}{
		{"days", []string{"--key-expires-in-days", "90"}, func(t *testing.T, body map[string]interface{}) {
			if body["apiKeyExpiresInDays"] != float64(90) {
				t.Fatalf("apiKeyExpiresInDays = %v, want 90", body["apiKeyExpiresInDays"])
			}
		}},
		{"date", []string{"--key-expires-at", "2099-03-31"}, func(t *testing.T, body map[string]interface{}) {
			if body["apiKeyExpiresAt"] != "2099-03-31T23:59:59Z" {
				t.Fatalf("apiKeyExpiresAt = %v, want the end of that day in UTC", body["apiKeyExpiresAt"])
			}
		}},
		{"rfc3339", []string{"--key-expires-at", "2099-03-31T12:00:00+02:00"}, func(t *testing.T, body map[string]interface{}) {
			if body["apiKeyExpiresAt"] != "2099-03-31T10:00:00Z" {
				t.Fatalf("apiKeyExpiresAt = %v, want the time in UTC", body["apiKeyExpiresAt"])
			}
		}},
		{"never", []string{"--key-never-expires"}, func(t *testing.T, body map[string]interface{}) {
			v, ok := body["apiKeyExpiresInDays"]
			if !ok || v != nil {
				t.Fatalf("want apiKeyExpiresInDays: null for a key that never expires, got %v (present %v)", v, ok)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"agents", "rotate-credentials", "agt_1"}, tc.flags...)
			_, reqs, err := runAgainstFakeServer(t, 200, rotateResponse, args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			req := expectOneRequest(t, reqs, "POST", adminBase+"/agents/agt_1/rotate-credentials")
			tc.check(t, req.Body)
		})
	}
}

// The list answer of GET /admin/agents: `data` items with agentId, name,
// status, identityType and lastUsedAt (an integer id; no capabilities).
const agentListResponse = `{"data":[{"id":7,"agentId":"agt_3f9a51c07be24d18","name":"Billing Bot","identityType":"api_key","status":"active","createdAt":"2026-11-01T10:00:00+00:00","lastUsedAt":"2026-11-12T09:30:00+00:00","riskLevel":null},{"id":8,"agentId":"agt_77","name":"Idle Bot","identityType":"workload","status":"inactive","createdAt":"2026-11-02T10:00:00+00:00","lastUsedAt":null,"riskLevel":null}],"meta":{"total":2,"page":1,"limit":20,"totalPages":1},"pagination":{"total":2,"page":1,"limit":20,"totalPages":1},"resourceType":"agents"}`

func TestAgentsListTableShowsTheFieldsTheServerSends(t *testing.T) {
	out, reqs, err := runAgainstFakeServer(t, 200, agentListResponse, "agents", "list", "-o", "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectOneRequest(t, reqs, "GET", adminBase+"/agents")
	for _, want := range []string{"agt_3f9a51c07be24d18", "Billing Bot", "api_key", "2026-11-12T09:30:00+00:00", "agt_77", "workload", "never"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table output lacks %q:\n%s", want, out)
		}
	}
}

func TestRotateCredentialsWithoutALifetimeSendsNoBody(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 200, rotateResponse, "agents", "rotate-credentials", "agt_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "POST", adminBase+"/agents/agt_1/rotate-credentials")
	if len(req.Body) != 0 {
		t.Fatalf("no lifetime flag: the server keeps the current date, so nothing is sent; got %v", req.Body)
	}
}

func TestInvalidKeyLifetimeIsRefusedBeforeAnyRequest(t *testing.T) {
	cases := map[string][]string{
		"zero days":       {"--key-expires-in-days", "0"},
		"too many days":   {"--key-expires-in-days", "3651"},
		"past date":       {"--key-expires-at", "2001-01-01"},
		"not a date":      {"--key-expires-at", "next tuesday"},
		"two lifetimes":   {"--key-expires-in-days", "30", "--key-never-expires"},
		"days and a date": {"--key-expires-in-days", "30", "--key-expires-at", "2099-01-01"},
	}
	for name, flags := range cases {
		t.Run(name, func(t *testing.T) {
			args := append([]string{"agents", "rotate-credentials", "agt_1"}, flags...)
			_, reqs, err := runAgainstFakeServer(t, 200, rotateResponse, args...)
			if err == nil {
				t.Fatalf("accepted %v", flags)
			}
			if len(reqs) != 0 {
				t.Fatalf("sent %d request(s) for an invalid lifetime", len(reqs))
			}
		})
	}
}

func TestCreateSendsIdentityTypeAndKeyLifetime(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 201, `{"data":{"agentId":"agt_2"}}`,
		"agents", "create", "--name", "Support bot", "--type", "api_key", "--key-expires-in-days", "30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "POST", adminBase+"/agents")
	if req.Body["identityType"] != "api_key" {
		t.Fatalf("identityType = %v; the admin API ignores `type`", req.Body["identityType"])
	}
	if _, sent := req.Body["type"]; sent {
		t.Fatalf("the ignored `type` field is still sent: %v", req.Body)
	}
	if req.Body["apiKeyExpiresInDays"] != float64(30) {
		t.Fatalf("apiKeyExpiresInDays = %v, want 30", req.Body["apiKeyExpiresInDays"])
	}
}

func TestUpdateChangesTheCurrentKeysLifetime(t *testing.T) {
	_, reqs, err := runAgainstFakeServer(t, 200, `{"data":{"agentId":"agt_1"}}`,
		"agents", "update", "agt_1", "--key-never-expires")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := expectOneRequest(t, reqs, "PATCH", adminBase+"/agents/agt_1")
	if v, ok := req.Body["apiKeyExpiresInDays"]; !ok || v != nil {
		t.Fatalf("want apiKeyExpiresInDays: null, got %v", req.Body)
	}
}

func TestRotateCredentialsHelpNamesTheLifetimeFlags(t *testing.T) {
	help := agentsRotateCredsCmd.Flags().FlagUsages()
	for _, flag := range []string{"--key-expires-in-days", "--key-expires-at", "--key-never-expires"} {
		if !strings.Contains(help, flag) {
			t.Fatalf("rotate-credentials --help does not list %s", flag)
		}
	}
}
