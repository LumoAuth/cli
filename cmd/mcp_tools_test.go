package cmd

import "testing"

func TestParseMcpSubject(t *testing.T) {
	for _, c := range []struct {
		in, kind, id string
		ok           bool
	}{
		{"agent:agt_1", "agent", "agt_1", true},
		{"agent:agent_agt_1", "agent", "agent_agt_1", true},
		{"user:0198-uuid", "user", "0198-uuid", true},
		{"robot:x", "", "", false},
		{"agent:", "", "", false},
		{"agt_1", "", "", false},
	} {
		kind, id, err := parseMcpSubject(c.in)
		if (err == nil) != c.ok || kind != c.kind || id != c.id {
			t.Errorf("parseMcpSubject(%q) = %q, %q, %v", c.in, kind, id, err)
		}
	}
}

func TestMcpDecision(t *testing.T) {
	for _, c := range []struct {
		status           int
		body             string
		allowed, decided bool
	}{
		{200, `{"allowed":true}`, true, true},
		{403, `{"allowed":false}`, false, true},
		{403, `{"error":"insufficient_scope"}`, false, false},
		{401, `{"error":"invalid_token"}`, false, false},
		{500, `oops`, false, false},
	} {
		a, d := mcpDecision(c.status, []byte(c.body))
		if a != c.allowed || d != c.decided {
			t.Errorf("mcpDecision(%d, %s) = %v, %v", c.status, c.body, a, d)
		}
	}
}
