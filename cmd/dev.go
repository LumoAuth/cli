package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Manage ephemeral sandbox tenants (Neon-branch style)",
	Long: `Spawn and tear down sandbox tenants for branch previews and PR
environments. Each sandbox is a separate tenant with its own slug and
TTL — defaults to 24 hours, max 7 days. Sandboxes are auto-cleaned by
the server's scheduled 'app:sandbox:cleanup' job.

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
	Short: "Spawn a new sandbox tenant",
	RunE:  runDevStart,
}

var devStopCmd = &cobra.Command{
	Use:   "stop <slug>",
	Short: "Tear down a sandbox tenant",
	Args:  cobra.ExactArgs(1),
	RunE:  runDevStop,
}

var devListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List your active sandbox tenants",
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

	if getPrinter().IsJSON() {
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
	fmt.Fprintln(os.Stderr, "Use it like any other org:")
	fmt.Fprintf(os.Stderr, "  lumo --org %s users list\n", out.Data.Slug)
	fmt.Fprintf(os.Stderr, "  curl %s/orgs/%s/api/v1/me  -H \"Authorization: Bearer ...\"\n", cfg.BaseURL, out.Data.Slug)
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
