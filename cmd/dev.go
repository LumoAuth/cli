package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Manage ephemeral sandbox organizations (Neon-branch style)",
	Long: `Spawn and tear down sandbox organizations for branch previews and PR
environments. Each sandbox is a separate organization with its own slug
and TTL: 24 hours by default, 7 days at most. A sandbox is deleted
automatically after it expires.

A new sandbox holds an administrator role, a signing key, a Starter App
client and one API key that can administer it. It holds no user who can
sign in. 'lumo dev start' saves the API key as a profile named after the
sandbox (in ~/.lumoauth/credentials.yaml), so every command works with
--org <sandbox-slug>, for example to add users:

  lumo --org sandbox-pr-1234-abc123 users create --email dev@example.com

Your own login or API key does not reach into a sandbox, and the sandbox
key reaches nothing outside it. The key ends with the sandbox, and
'lumo dev stop' removes the saved profile. With -o json (the default
when the output is piped) 'lumo dev start' prints the server's answer,
which contains the key in data.api_key: this is the only time it is shown.

A sandbox uses the plan of the organization that created it and cannot
create sandboxes itself.

Spawning needs the Developer sandboxes add-on ($199/month, Business plan).
Without it 'lumo dev start' answers 403 feature_not_in_plan and names the
billing page where an organization admin can add it; 'list' and 'stop'
keep working so existing sandboxes can always be torn down.

Common workflows:
  lumo dev start --name pr-1234        # spawn for a PR
  lumo dev list                        # see what's running
  lumo dev stop sandbox-pr-1234-abc123 # tear down before merge`,
}

var (
	devStartName string
	devStartTTL  int
)

var devStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Spawn a new sandbox organization and save its API key",
	RunE:  runDevStart,
}

var devStopCmd = &cobra.Command{
	Use:   "stop <slug>",
	Short: "Tear down a sandbox organization and remove its saved API key",
	Args:  cobra.ExactArgs(1),
	RunE:  runDevStop,
}

var devListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List your active sandbox organizations",
	RunE:    runDevList,
}

func init() {
	devStartCmd.Flags().StringVar(&devStartName, "name", "", "Optional name suffix (becomes part of the slug)")
	devStartCmd.Flags().IntVar(&devStartTTL, "ttl-hours", 24, "Sandbox TTL in hours (max 168)")

	devCmd.AddCommand(devStartCmd, devStopCmd, devListCmd)
	rootCmd.AddCommand(devCmd)
}

type sandboxRow struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	CreatedAt   string  `json:"created_at"`
	ExpiresAt   *string `json:"expires_at"`
	OwnerEmail  *string `json:"owner_email"`
	SpawnedFrom *string `json:"spawned_from"`
	// APIKey is only in the answer to spawn, once.
	APIKey string `json:"api_key,omitempty"`
}

type sandboxResponse struct {
	Data sandboxRow `json:"data"`
}

type sandboxListResponse struct {
	Data []sandboxRow `json:"data"`
}

func runDevStart(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	cfg := c.Config()

	body := map[string]interface{}{}
	if devStartName != "" {
		body["name"] = devStartName
	}
	if devStartTTL > 0 {
		body["ttl_hours"] = devStartTTL
	}

	resp, err := c.Post("/sandbox/spawn", body)
	if err != nil {
		return fmt.Errorf("spawn sandbox: %w", err)
	}
	var out sandboxResponse
	if err := json.Unmarshal(resp, &out); err != nil {
		return fmt.Errorf("decode spawn response: %w", err)
	}

	// The key is shown by the server once, so keep it before anything else
	// can fail. It is the only credential that reaches the sandbox.
	saved := false
	if out.Data.APIKey != "" {
		var expires time.Time
		if out.Data.ExpiresAt != nil {
			expires, _ = time.Parse(time.RFC3339, *out.Data.ExpiresAt)
		}
		if err := config.SaveSandboxKey(out.Data.Slug, cfg.BaseURL, out.Data.APIKey, expires, cfg.Insecure); err != nil {
			// Not saved and never shown again: print it rather than lose it.
			fmt.Fprintf(os.Stderr, "Warning: the sandbox API key could not be saved (%v).\n", err)
			fmt.Fprintf(os.Stderr, "  Keep it now, it is not shown again: %s\n", out.Data.APIKey)
		} else {
			saved = true
		}
	}

	if !getPrinter().IsTable() {
		getPrinter().PrintResult(resp)
		return nil
	}

	fmt.Fprintf(os.Stderr, "✓ Sandbox spawned\n")
	fmt.Fprintf(os.Stderr, "  Slug:       %s\n", out.Data.Slug)
	fmt.Fprintf(os.Stderr, "  Base URL:   %s\n", cfg.BaseURL)
	if out.Data.ExpiresAt != nil {
		fmt.Fprintf(os.Stderr, "  Expires at: %s\n", *out.Data.ExpiresAt)
	}
	fmt.Fprintln(os.Stderr)
	if out.Data.APIKey == "" {
		fmt.Fprintln(os.Stderr, "The server returned no API key for this sandbox, so this CLI cannot manage it.")
		fmt.Fprintf(os.Stderr, "Tear down with: lumo dev stop %s\n", out.Data.Slug)
		return nil
	}
	if saved {
		fmt.Fprintf(os.Stderr, "  API key:    saved as profile %q in %s\n", out.Data.Slug, config.CredentialsPath())
		fmt.Fprintln(os.Stderr)
	}
	fmt.Fprintln(os.Stderr, "The sandbox has no users yet. Manage it with its API key, which --org selects:")
	fmt.Fprintf(os.Stderr, "  lumo --org %s users create --email dev@example.com\n", out.Data.Slug)
	fmt.Fprintf(os.Stderr, "  lumo --org %s users list\n", out.Data.Slug)
	fmt.Fprintf(os.Stderr, "  lumo --org %s apps list\n", out.Data.Slug)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Tear down with: lumo dev stop %s\n", out.Data.Slug)
	return nil
}

func runDevStop(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	slug := args[0]
	if _, err := c.Post(fmt.Sprintf("/sandbox/%s/destroy", slug), nil); err != nil {
		return fmt.Errorf("destroy sandbox: %w", err)
	}
	// The key died with the sandbox; drop the saved copy too.
	if _, err := config.DeleteSandboxKey(slug); err != nil {
		getPrinter().PrintNote(fmt.Sprintf("Sandbox destroyed, but its saved profile could not be removed: %v. Remove it with 'lumo profile delete %s'.", err, slug))
	}
	getPrinter().PrintSuccess(fmt.Sprintf("Sandbox %s destroyed", slug))
	return nil
}

func runDevList(cmd *cobra.Command, args []string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	resp, err := c.Get("/sandbox", nil)
	if err != nil {
		return fmt.Errorf("list sandboxes: %w", err)
	}
	var out sandboxListResponse
	if err := json.Unmarshal(resp, &out); err != nil {
		return fmt.Errorf("decode sandbox list: %w", err)
	}

	p := getPrinter()
	if !p.IsTable() {
		p.PrintResult(resp)
		return nil
	}
	if len(out.Data) == 0 {
		fmt.Fprintln(os.Stderr, "No active sandboxes.")
		return nil
	}
	rows := make([][]string, len(out.Data))
	for i, s := range out.Data {
		exp := "-"
		if s.ExpiresAt != nil {
			exp = *s.ExpiresAt
		}
		rows[i] = []string{s.Slug, s.Name, s.CreatedAt, exp}
	}
	p.PrintTable([]string{"Slug", "Name", "Created", "Expires"}, rows)
	return nil
}
