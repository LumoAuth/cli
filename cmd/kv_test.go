package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/lumoauth/cli/internal/client"
	"github.com/lumoauth/cli/internal/config"
)

func TestExitCodes(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{fmt.Errorf("%w: nothing configured", config.ErrNotAuthenticated), ExitAuth},
		{&client.APIError{StatusCode: 401}, ExitAuth},
		{&client.APIError{StatusCode: 403}, ExitAuth},
		{&client.APIError{StatusCode: 404}, ExitNotFound},
		{&client.APIError{StatusCode: 400}, ExitInput},
		{&client.APIError{StatusCode: 409}, ExitInput},
		{&client.APIError{StatusCode: 429}, ExitRateLimit},
		{&client.APIError{StatusCode: 500}, ExitGeneral},
		{errors.New("plain"), ExitGeneral},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.want {
			t.Errorf("exitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestParseSetFlags(t *testing.T) {
	got, err := parseSetFlags([]string{
		"name=Acme Corp",
		"security.dpop_require_nonce=true",
		"security.password_policy.min_length=12",
		"locale=str:123",
		"tags=[\"a\",\"b\"]",
		"note=null",
		"ratio=0.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"name": "Acme Corp",
		"security": map[string]interface{}{
			"dpop_require_nonce": true,
			"password_policy":    map[string]interface{}{"min_length": int64(12)},
		},
		"locale": "123",
		"tags":   []interface{}{"a", "b"},
		"note":   nil,
		"ratio":  0.5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSetFlags = %#v\nwant %#v", got, want)
	}
	if _, err := parseSetFlags([]string{"novalue"}); err == nil {
		t.Fatal("expected error for a pair without '='")
	}
}
