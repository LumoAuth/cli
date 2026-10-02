package cmd

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
)

// `all:` prefix is required for Go's embed to include dotfiles like .env —
// without it, .env.local and .env would silently be omitted from the binary.
//
//go:embed all:templates/init
var initTemplates embed.FS

var (
	initFramework string
	initDir       string
	initOrgID     string
	initClientID  string
	initSDKPath   string
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initFramework, "framework", "", "Starter to scaffold: next | express | fastapi | go")
	initCmd.Flags().StringVar(&initDir, "dir", ".", "Target directory (created if missing)")
	initCmd.Flags().StringVar(&initOrgID, "org", "", "Organization slug to pre-fill in .env (defaults to current credentials)")
	initCmd.Flags().StringVar(&initClientID, "client-id", "", "OAuth client ID to pre-fill in .env (e.g. the Starter App's, from the dashboard Quickstart page)")
	initCmd.Flags().StringVar(&initSDKPath, "sdk-path", "", "Install the LumoAuth SDK from a local source checkout (the directory containing sdk-js, sdk-python and sdk-go) instead of the package registry. Defaults to $LUMO_SDK_PATH")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a starter project wired to LumoAuth",
	Long: `Scaffolds a working starter project that can sign in with LumoAuth out of the
box. Choose a framework with --framework; .env is pre-filled with the org slug
and base URL from your current credentials (run 'lumo login' first), or pass
--org to override. Pass --client-id to pre-fill the OAuth client ID as well.

Until the SDK packages are published, pass --sdk-path (or set LUMO_SDK_PATH)
pointing at a LumoAuth source checkout: the starter is then wired to install
the SDK from those local sources rather than npm / PyPI / the Go proxy.

Available starters:
  next      Next.js 14 (App Router) + @lumoauth/nextjs
  express   Express.js + @lumoauth/express middleware
  fastapi   FastAPI + lumoauth Python SDK
  go        Go (chi router) + lumo-auth-go middleware

The generated project is intentionally minimal — copy what you need into a
real project, or use it as the starting point. Any existing files in the
target directory are left alone unless --force is passed.`,
	RunE: runInit,
}

var initForce bool

func init() { initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing files") }

var supportedFrameworks = map[string]string{
	"next":    "Next.js 14 (App Router) + @lumoauth/nextjs",
	"express": "Express.js + @lumoauth/express middleware",
	"fastapi": "FastAPI + lumoauth Python SDK",
	"go":      "Go (chi router) + lumo-auth-go middleware",
}

func runInit(cmd *cobra.Command, args []string) error {
	if initFramework == "" {
		fmt.Fprintln(os.Stderr, "Pick a framework with --framework. Available:")
		for k, v := range supportedFrameworks {
			fmt.Fprintf(os.Stderr, "  %-8s  %s\n", k, v)
		}
		return fmt.Errorf("--framework is required")
	}
	desc, ok := supportedFrameworks[initFramework]
	if !ok {
		return fmt.Errorf("unknown framework %q — try: next, express, fastapi, go", initFramework)
	}

	target, err := filepath.Abs(initDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Pre-fill values for the .env. Prefer explicit --org, then credentials.
	// The base URL always follows the logged-in profile, so --org still
	// targets the same (possibly local) server.
	orgID := initOrgID
	baseURL := "https://app.lumoauth.dev"
	if creds, _ := config.LoadCredentials(); creds != nil {
		if orgID == "" {
			orgID = creds.OrgID
		}
		if creds.BaseURL != "" {
			baseURL = creds.BaseURL
		}
	}
	if orgID == "" {
		orgID = "your-tenant-slug"
	}
	clientID := initClientID
	if clientID == "" {
		clientID = "replace-with-your-oauth-client-id"
	}
	sessionSecret, err := randomHex(32)
	if err != nil {
		return err
	}

	// Resolve the local SDK checkout up front so a bad path fails before
	// anything is written.
	sdkPath := initSDKPath
	if sdkPath == "" {
		sdkPath = os.Getenv("LUMO_SDK_PATH")
	}
	var sdk *localSDK
	if sdkPath != "" {
		if sdk, err = prepareLocalSDK(initFramework, sdkPath, target); err != nil {
			return err
		}
	}

	tmplRoot := "templates/init/" + initFramework
	written := 0
	skipped := 0

	err = fs.WalkDir(initTemplates, tmplRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tmplRoot, path)
		if err != nil {
			return err
		}
		// `go.mod.tmpl` exists in the source tree because Go's `embed`
		// rejects subdirs that contain a `go.mod` (it would treat them as a
		// separate module). Strip `.tmpl` when materialising.
		rel = strings.TrimSuffix(rel, ".tmpl")
		// Likewise a real `.gitignore` in the template tree would apply to
		// this repository and hide the starters' own `.env` files.
		if rel == "gitignore" {
			rel = ".gitignore"
		}
		dest := filepath.Join(target, rel)

		if _, err := os.Stat(dest); err == nil && !initForce {
			fmt.Fprintf(os.Stderr, "skip   %s (exists; pass --force to overwrite)\n", rel)
			skipped++
			return nil
		}

		data, err := initTemplates.ReadFile(path)
		if err != nil {
			return err
		}
		// Substitute placeholders.
		out := string(data)
		out = strings.ReplaceAll(out, "__LUMO_ORG_ID__", orgID)
		out = strings.ReplaceAll(out, "__LUMO_BASE_URL__", baseURL)
		out = strings.ReplaceAll(out, "__LUMO_CLIENT_ID__", clientID)
		out = strings.ReplaceAll(out, "__LUMO_SESSION_SECRET__", sessionSecret)
		if sdk != nil {
			if out, err = sdk.rewrite(rel, out); err != nil {
				return err
			}
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "ok     %s\n", rel)
		written++
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "✓ Scaffolded %s\n", desc)
	fmt.Fprintf(os.Stderr, "  Wrote %d file(s), skipped %d in %s\n", written, skipped, target)
	fmt.Fprintln(os.Stderr)
	if sdk != nil {
		fmt.Fprintf(os.Stderr, "  LumoAuth SDK: local sources from %s\n", sdk.root)
	} else {
		fmt.Fprintln(os.Stderr, "  LumoAuth SDK: package registry. If the install step cannot find the")
		fmt.Fprintln(os.Stderr, "  LumoAuth packages, re-run with --sdk-path <lumoauth checkout> --force.")
	}
	if initClientID == "" {
		fmt.Fprintln(os.Stderr, "  Set the OAuth client ID in the .env file before signing in (or re-run with --client-id).")
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Next steps:")
	switch initFramework {
	case "next":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && npm install && npm run dev")
	case "express":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && npm install && npm start")
	case "fastapi":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt && uvicorn main:app --reload --port 3000")
	case "go":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && go mod tidy && go run .")
	}
	return nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// localSDK wires a starter to SDK sources in a local LumoAuth checkout, for
// use before the packages are published to npm / PyPI / the Go proxy.
type localSDK struct {
	framework string
	root      string
	// npmDeps maps package name → `file:` spec of a tarball vendored into
	// the starter (next, express).
	npmDeps [][2]string
}

// jsSDKPackages lists, per starter, the @lumoauth package it depends on
// followed by that package's own @lumoauth dependency chain. The whole chain
// is vendored because none of it can resolve from the registry.
var jsSDKPackages = map[string][]string{
	"next":    {"nextjs", "react", "client", "shared"},
	"express": {"express", "backend", "shared"},
}

func prepareLocalSDK(framework, sdkPath, target string) (*localSDK, error) {
	root, err := filepath.Abs(sdkPath)
	if err != nil {
		return nil, err
	}
	sdk := &localSDK{framework: framework, root: root}

	sub := map[string]string{"next": "sdk-js", "express": "sdk-js", "fastapi": "sdk-python", "go": "sdk-go"}[framework]
	if _, err := os.Stat(filepath.Join(root, sub)); err != nil {
		return nil, fmt.Errorf("--sdk-path %s: no %s directory found — point it at the root of a LumoAuth source checkout", root, sub)
	}

	pkgs, ok := jsSDKPackages[framework]
	if !ok {
		return sdk, nil
	}

	// The JS packages ship only dist/, so build the workspace once if needed,
	// then pack real tarballs into the starter. A plain directory install
	// would symlink into sdk-js and resolve a second copy of React from there.
	jsRoot := filepath.Join(root, "sdk-js")
	if _, err := exec.LookPath("npm"); err != nil {
		return nil, fmt.Errorf("--sdk-path needs npm on PATH to build and pack the SDK: %w", err)
	}
	built := true
	for _, p := range pkgs {
		if _, err := os.Stat(filepath.Join(jsRoot, "packages", p, "dist")); err != nil {
			built = false
		}
	}
	if !built {
		fmt.Fprintf(os.Stderr, "Building the SDK in %s (first run only)...\n", jsRoot)
		for _, args := range [][]string{{"install"}, {"run", "build"}} {
			if err := runQuiet(jsRoot, "npm", args...); err != nil {
				return nil, err
			}
		}
	}

	vendor := filepath.Join(target, "vendor")
	if err := os.MkdirAll(vendor, 0o755); err != nil {
		return nil, err
	}
	packArgs := []string{"pack", "--pack-destination", vendor}
	for _, p := range pkgs {
		raw, err := os.ReadFile(filepath.Join(jsRoot, "packages", p, "package.json"))
		if err != nil {
			return nil, err
		}
		var meta struct{ Name, Version string }
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, fmt.Errorf("%s/package.json: %w", p, err)
		}
		// npm pack names the tarball <scope>-<name>-<version>.tgz.
		tarball := strings.ReplaceAll(strings.TrimPrefix(meta.Name, "@"), "/", "-") + "-" + meta.Version + ".tgz"
		sdk.npmDeps = append(sdk.npmDeps, [2]string{meta.Name, "file:./vendor/" + tarball})
		packArgs = append(packArgs, "-w", meta.Name)
	}
	if err := runQuiet(jsRoot, "npm", packArgs...); err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "ok     vendor/ (%d SDK tarballs)\n", len(pkgs))
	return sdk, nil
}

// rewrite points the starter's dependency manifest at the local SDK.
func (s *localSDK) rewrite(rel, content string) (string, error) {
	switch {
	case rel == "package.json" && len(s.npmDeps) > 0:
		// Replace the single registry dependency with the vendored chain.
		registryDep := fmt.Sprintf("    %q: \"^1.0.0\"", s.npmDeps[0][0])
		if !strings.Contains(content, registryDep) {
			return "", fmt.Errorf("package.json template has no %s dependency to rewrite", s.npmDeps[0][0])
		}
		lines := make([]string, 0, len(s.npmDeps))
		for _, d := range s.npmDeps {
			lines = append(lines, fmt.Sprintf("    %q: %q", d[0], d[1]))
		}
		return strings.Replace(content, registryDep, strings.Join(lines, ",\n"), 1), nil
	case rel == "requirements.txt" && s.framework == "fastapi":
		const registryDep = "lumoauth[fastapi]>=1.0.0"
		if !strings.Contains(content, registryDep) {
			return "", fmt.Errorf("requirements.txt template has no %s line to rewrite", registryDep)
		}
		local := "lumoauth[fastapi] @ file://" + filepath.ToSlash(filepath.Join(s.root, "sdk-python"))
		return strings.Replace(content, registryDep, local, 1), nil
	case rel == "go.mod" && s.framework == "go":
		return content + "\nreplace github.com/lumoauth/lumo-auth-go => " + filepath.ToSlash(filepath.Join(s.root, "sdk-go")) + "\n", nil
	}
	return content, nil
}

// runQuiet runs a command and only surfaces its output when it fails.
func runQuiet(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s (in %s) failed: %w\n%s", name, strings.Join(args, " "), dir, err, out)
	}
	return nil
}
