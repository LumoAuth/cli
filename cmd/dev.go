package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Manage ephemeral sandbox tenants (Neon-branch style)",
	Long: `Spawn and tear down sandbox tenants for branch previews and PR
environments. Each sandbox is a separate tenant with its own slug and
TTL — defaults to 24 hours, max 7 days. Sandboxes are auto-cleaned by
the server's scheduled 'app:sandbox:cleanup' job.

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

func devHTTPClient(insecure bool) *http.Client {
	tr := &http.Transport{}
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Transport: tr, Timeout: 30 * time.Second}
}

func runDevStart(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigValidated()
	if err != nil {
		return err
	}
	auth, err := authHeader(cfg)
	if err != nil {
		return err
	}

	body := map[string]interface{}{}
	if devStartName != "" {
		body["name"] = devStartName
	}
	if devStartTTL > 0 {
		body["ttl_hours"] = devStartTTL
	}
	bodyJSON, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/orgs/%s/api/v1/admin/sandbox/spawn",
		strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(bodyJSON)))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := devHTTPClient(cfg.Insecure).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		var errBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("spawn failed (HTTP %d): %v", resp.StatusCode, errBody)
	}

	var out sandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "✓ Sandbox spawned\n")
	fmt.Fprintf(os.Stderr, "  Slug:       %s\n", out.Data.Slug)
	fmt.Fprintf(os.Stderr, "  Base URL:   %s\n", cfg.BaseURL)
	if out.Data.ExpiresAt != nil {
		fmt.Fprintf(os.Stderr, "  Expires at: %s\n", *out.Data.ExpiresAt)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Use it like any other org:")
	fmt.Fprintf(os.Stderr, "  lumo --org-id %s users list\n", out.Data.Slug)
	fmt.Fprintf(os.Stderr, "  curl %s/orgs/%s/api/v1/me  -H \"Authorization: Bearer ...\"\n", cfg.BaseURL, out.Data.Slug)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Tear down with: lumo dev stop %s\n", out.Data.Slug)
	return nil
}

func runDevStop(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigValidated()
	if err != nil {
		return err
	}
	auth, err := authHeader(cfg)
	if err != nil {
		return err
	}
	slug := args[0]

	url := fmt.Sprintf("%s/orgs/%s/api/v1/admin/sandbox/%s/destroy",
		strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID, slug)
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")

	resp, err := devHTTPClient(cfg.Insecure).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var errBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("destroy failed (HTTP %d): %v", resp.StatusCode, errBody)
	}
	fmt.Fprintf(os.Stderr, "✓ Sandbox %s destroyed\n", slug)
	return nil
}

func runDevList(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigValidated()
	if err != nil {
		return err
	}
	auth, err := authHeader(cfg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/orgs/%s/api/v1/admin/sandbox",
		strings.TrimRight(cfg.BaseURL, "/"), cfg.OrgID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")

	resp, err := devHTTPClient(cfg.Insecure).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := json.Marshal(map[string]int{"status": resp.StatusCode})
		return fmt.Errorf("list failed: %s", string(body))
	}
	var out sandboxListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}

	if getPrinter().IsJSON() {
		raw, _ := json.Marshal(out.Data)
		fmt.Println(string(raw))
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
	getPrinter().PrintTable([]string{"Slug", "Name", "Created", "Expires"}, rows)
	return nil
}
