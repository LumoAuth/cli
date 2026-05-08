package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
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
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initFramework, "framework", "", "Starter to scaffold: next | express | fastapi | go")
	initCmd.Flags().StringVar(&initDir, "dir", ".", "Target directory (created if missing)")
	initCmd.Flags().StringVar(&initOrgID, "org", "", "Organization slug to pre-fill in .env (defaults to current credentials)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a starter project wired to LumoAuth",
	Long: `Scaffolds a working starter project that can sign in with LumoAuth out of the
box. Choose a framework with --framework; .env is pre-filled with the org slug
and base URL from your current credentials (run 'lumo login' first), or pass
--org to override.

Available starters:
  next      Next.js 14 (App Router) + @lumoauth/react
  express   Express.js + @lumoauth/sdk middleware
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
	"next":    "Next.js 14 (App Router) + @lumoauth/react",
	"express": "Express.js + @lumoauth/sdk middleware",
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
	orgID := initOrgID
	baseURL := "https://app.lumoauth.dev"
	if orgID == "" {
		if creds, _ := config.LoadCredentials(); creds != nil {
			orgID = creds.OrgID
			baseURL = creds.BaseURL
		}
	}
	if orgID == "" {
		orgID = "your-tenant-slug"
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
	fmt.Fprintln(os.Stderr, "Next steps:")
	switch initFramework {
	case "next":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && npm install && npm run dev")
	case "express":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && npm install && node server.js")
	case "fastapi":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && pip install -r requirements.txt && uvicorn main:app --reload")
	case "go":
		fmt.Fprintln(os.Stderr, "  cd "+initDir+" && go mod tidy && go run .")
	}
	return nil
}
