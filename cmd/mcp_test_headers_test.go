package cmd

import "testing"

func TestMcpNameHeader(t *testing.T) {
	cases := []struct {
		method string
		params map[string]interface{}
		want   string
	}{
		{"tools/list", map[string]interface{}{}, ""},
		{"tools/call", map[string]interface{}{"name": "get_weather"}, "get_weather"},
		{"resources/read", map[string]interface{}{"uri": "file:///a.txt"}, "file:///a.txt"},
		{"prompts/get", map[string]interface{}{"name": "résumé"}, "=?base64?csOpc3Vtw6k=?="},
		{"tools/call", map[string]interface{}{"name": " padded"}, "=?base64?IHBhZGRlZA==?="},
	}
	for _, c := range cases {
		if got := mcpNameHeader(c.method, c.params); got != c.want {
			t.Errorf("mcpNameHeader(%q) = %q, want %q", c.method, got, c.want)
		}
	}
}
