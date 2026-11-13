package cmd

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// recordSleeps replaces the importer's sleep for the test's duration.
func recordSleeps(t *testing.T) *[]time.Duration {
	t.Helper()
	var (
		mu     sync.Mutex
		sleeps []time.Duration
	)
	previous := migrateSleep
	migrateSleep = func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		sleeps = append(sleeps, d)
	}
	t.Cleanup(func() { migrateSleep = previous })
	return &sleeps
}

func TestImportOneSendsResetForCreatedUser(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing credential on %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/users" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"id":"019d6190-9bba-7b5f-a0a5-07a6d10184d2","email":"a@example.com"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"Password reset email sent successfully"}`))
	}))
	defer srv.Close()

	got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, false, true)

	if got.Status != "created" || got.Reset != "sent" {
		t.Fatalf("status=%q reset=%q message=%q", got.Status, got.Reset, got.ResetMessage)
	}
	want := []string{"POST /users", "POST /users/019d6190-9bba-7b5f-a0a5-07a6d10184d2/password-reset"}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("requests = %v, want %v", paths, want)
	}
}

func TestImportOneSendsNoResetWithoutTheFlagOrForExistingUsers(t *testing.T) {
	status := http.StatusCreated
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"data":{"id":"u1"}}`))
	}))
	defer srv.Close()

	if got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, false, false); got.Reset != "" || calls != 1 {
		t.Fatalf("without --send-reset: reset=%q calls=%d", got.Reset, calls)
	}

	// An address that already exists is skipped: its owner may be set up already.
	status, calls = http.StatusConflict, 0
	got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, false, true)
	if got.Status != "skipped" || got.Reset != "" || calls != 1 {
		t.Fatalf("existing user: status=%q reset=%q calls=%d", got.Status, got.Reset, calls)
	}

	// A dry run contacts nothing.
	calls = 0
	if got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, true, true); got.Status != "created" || calls != 0 {
		t.Fatalf("dry run: status=%q calls=%d", got.Status, calls)
	}
}

func TestImportOneReportsAFailedReset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"id":"u1"}}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"not allowed"}`))
	}))
	defer srv.Close()

	got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, false, true)
	if got.Status != "created" || got.Reset != "failed" || got.ResetMessage != "HTTP 403  not allowed" {
		t.Fatalf("status=%q reset=%q message=%q", got.Status, got.Reset, got.ResetMessage)
	}
}

func TestThrottledRequestWaitsForRetryAfterAndIsSentAgain(t *testing.T) {
	sleeps := recordSleeps(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"u1"}}`))
	}))
	defer srv.Close()

	got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, false, false)

	if got.Status != "created" || calls != 3 {
		t.Fatalf("status=%q calls=%d", got.Status, calls)
	}
	if len(*sleeps) != 2 || (*sleeps)[0] != 7*time.Second || (*sleeps)[1] != 7*time.Second {
		t.Fatalf("sleeps = %v, want two waits of 7s", *sleeps)
	}
}

func TestThrottlingGivesUpAfterTheRetryBudget(t *testing.T) {
	sleeps := recordSleeps(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"Too many requests for this credential"}`))
	}))
	defer srv.Close()

	got := importOne(srv.Client(), srv.URL+"/users", "Bearer tok", auth0User{Email: "a@example.com"}, false, false)

	if got.Status != "failed" || got.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%q code=%d", got.Status, got.Code)
	}
	if calls != migrateMaxRetries+1 || len(*sleeps) != migrateMaxRetries {
		t.Fatalf("calls=%d sleeps=%d", calls, len(*sleeps))
	}
}

func TestRetryAfterDelay(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		header  string
		attempt int
		want    time.Duration
	}{
		{"seconds", "30", 0, 30 * time.Second},
		{"zero becomes one second", "0", 0, time.Second},
		{"http date", now.Add(45 * time.Second).Format(http.TimeFormat), 0, 45 * time.Second},
		{"missing header backs off", "", 0, migrateDefaultBackoff},
		{"backoff doubles", "", 2, 4 * migrateDefaultBackoff},
		{"capped", "3600", 0, migrateMaxRetryWait},
		{"garbage backs off", "soon", 1, 2 * migrateDefaultBackoff},
	}
	for _, c := range cases {
		if got := retryAfterDelay(c.header, c.attempt, now); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
