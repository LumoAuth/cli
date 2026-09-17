package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestStartSendsExplicitScopes(t *testing.T) {
	var gotScope, gotClient string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotScope = r.Form.Get("scope")
		gotClient = r.Form.Get("client_id")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"device_code": "d", "user_code": "ABCD-EFGH", "verification_uri": srv0(r), "expires_in": 600, "interval": 5,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "acme", false)
	if _, err := c.Start(nil); err != nil {
		t.Fatal(err)
	}
	if gotClient != CliClientID {
		t.Fatalf("client_id = %q", gotClient)
	}
	if gotScope != "openid profile email admin" {
		t.Fatalf("default scope = %q", gotScope)
	}
	if _, err := c.Start([]string{"openid", "admin:users:read"}); err != nil {
		t.Fatal(err)
	}
	if gotScope != "openid admin:users:read" {
		t.Fatalf("narrowed scope = %q", gotScope)
	}
}

func srv0(r *http.Request) string { return "http://" + r.Host + "/device" }

func TestSplitScopes(t *testing.T) {
	got := SplitScopes(" openid, admin:users:read  admin ,")
	want := []string{"openid", "admin:users:read", "admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitScopes = %v, want %v", got, want)
	}
}
