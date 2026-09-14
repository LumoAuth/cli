package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Drift protection: every string-literal path handed to the admin client
// (c.Get/Post/Put/Patch/Delete in cmd/*.go) must exist in the server's
// OpenAPI spec once the prefix internal/client.adminURL adds is applied.
//
// This catches the "/admin/admin/..." class of bug where a command repeats a
// prefix the client already supplies, or targets a route that no longer
// exists on the server.

// adminPrefix mirrors client.adminURL: "{base}/orgs/{orgId}/api/v1/admin{path}".
const adminPrefix = "/orgs/{orgId}/api/v1/admin"

var (
	// c.Get("/agents", q)  |  c.Post(fmt.Sprintf("/agents/%s/token", id), body)
	callRe  = regexp.MustCompile(`\bc\.(Get|Post|Put|Patch|Delete)\(\s*(?:fmt\.Sprintf\(\s*)?"([^"]+)"`)
	verbRe  = regexp.MustCompile(`%[sdvq]`)
	paramRe = regexp.MustCompile(`\{[^}]+\}`)
)

// normalise collapses every {name} placeholder to {} so
// /agents/{agentId} and /agents/{param} compare equal.
func normalise(p string) string {
	return paramRe.ReplaceAllString(p, "{}")
}

func locateSpec(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("LUMO_OPENAPI_PATH"); p != "" {
		return p
	}
	// cmd/ -> cli/ -> monorepo root -> server/openapi.json
	candidates := []string{
		filepath.Join("..", "..", "server", "openapi.json"),
		filepath.Join("..", "..", "api-clients", "openapi.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Skipf("openapi.json not found (set LUMO_OPENAPI_PATH or check out the server repo next to cli/); looked in %v", candidates)
	return ""
}

func loadSpecPaths(t *testing.T, specFile string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read %s: %v", specFile, err)
	}
	var spec struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse %s: %v", specFile, err)
	}
	if len(spec.Paths) == 0 {
		t.Fatalf("%s has no paths", specFile)
	}
	out := make(map[string]string, len(spec.Paths))
	for p := range spec.Paths {
		out[normalise(p)] = p
	}
	return out
}

type cliRoute struct {
	file   string
	method string
	path   string
}

func collectCliRoutes(t *testing.T) []cliRoute {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var routes []cliRoute
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range callRe.FindAllStringSubmatch(string(src), -1) {
			p := verbRe.ReplaceAllString(m[2], "{param}")
			routes = append(routes, cliRoute{file: f, method: strings.ToUpper(m[1]), path: p})
		}
	}
	if len(routes) == 0 {
		t.Fatal("no c.Get/Post/... string-literal calls found in cmd/*.go; regex out of date?")
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].path < routes[j].path })
	return routes
}

func TestAdminRoutesExistInOpenAPISpec(t *testing.T) {
	specFile := locateSpec(t)
	specPaths := loadSpecPaths(t, specFile)
	routes := collectCliRoutes(t)

	seen := map[string]bool{}
	for _, r := range routes {
		full := adminPrefix + r.path
		key := normalise(full)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := specPaths[key]; !ok {
			t.Errorf("%s: %s %s -> %s is not a path in %s (route drift: the client already prefixes %q)",
				r.file, r.method, r.path, full, specFile, adminPrefix)
		}
	}
	t.Logf("checked %d distinct admin paths against %s", len(seen), specFile)
}

func TestNoDoubleAdminPrefix(t *testing.T) {
	// Independent of the spec so it also runs in CI without the server repo.
	for _, r := range collectCliRoutes(t) {
		if strings.HasPrefix(r.path, "/admin/") || strings.HasPrefix(r.path, "/api/v1/") {
			t.Errorf("%s: %s %q repeats a prefix the client already adds (%s)", r.file, r.method, r.path, adminPrefix)
		}
	}
}
