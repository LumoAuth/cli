package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:     "agents",
	Aliases: []string{"agent"},
	Short:   "Manage AI agents",
	Long:    "List, create, update, delete AI agents and manage their credentials and capabilities.",
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		q := url.Values{}
		if v, _ := cmd.Flags().GetString("search"); v != "" {
			q.Set("search", v)
		}

		resp, err := c.Get("/agents", q)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		// The list items of GET /admin/agents (AgentHandler::serialize):
		// capabilities are only in the single-agent answer (`agents get`).
		var result struct {
			Data []struct {
				AgentID      string  `json:"agentId"`
				Name         string  `json:"name"`
				Status       string  `json:"status"`
				IdentityType string  `json:"identityType"`
				LastUsedAt   *string `json:"lastUsedAt"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("unexpected agent list response: %w", err)
		}

		rows := make([][]string, len(result.Data))
		for i, a := range result.Data {
			lastUsed := "never"
			if a.LastUsedAt != nil && *a.LastUsedAt != "" {
				lastUsed = *a.LastUsedAt
			}
			rows[i] = []string{a.AgentID, a.Name, a.Status, a.IdentityType, lastUsed}
		}
		p.PrintTable([]string{"Agent ID", "Name", "Status", "Identity type", "Last used"}, rows)
		return nil
	},
}

var agentsGetCmd = &cobra.Command{
	Use:   "get <agent-id>",
	Short: "Get agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get(fmt.Sprintf("/agents/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var agentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		body := map[string]interface{}{"name": name}
		if v, _ := cmd.Flags().GetString("type"); v != "" {
			// The admin API reads `identityType`; `type` was silently ignored,
			// so every agent came out as an API key agent.
			body["identityType"] = v
		}
		if err := addKeyLifetime(cmd, body); err != nil {
			return err
		}
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			body["description"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("capabilities"); len(v) > 0 {
			body["capabilities"] = v
		}
		if v, _ := cmd.Flags().GetBool("act-on-behalf"); v {
			body["canActOnBehalfOfUsers"] = true
		}

		resp, err := c.Post("/agents", body)
		if err != nil {
			return err
		}

		// This typically returns the API key which should be shown
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var agentsUpdateCmd = &cobra.Command{
	Use:   "update <agent-id>",
	Short: "Update an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("name"); v != "" {
			body["name"] = v
		}
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			body["description"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("capabilities"); len(v) > 0 {
			body["capabilities"] = v
		}
		if err := addKeyLifetime(cmd, body); err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("no fields to update")
		}

		resp, err := c.Patch(fmt.Sprintf("/agents/%s", args[0]), body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Agent updated")
		return nil
	},
}

var agentsDeleteCmd = &cobra.Command{
	Use:   "delete <agent-id>",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/agents/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Agent deleted")
		return nil
	},
}

var agentsEnableCmd = &cobra.Command{
	Use:   "enable <agent-id>",
	Short: "Enable (activate) an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Post(fmt.Sprintf("/agents/%s/activate", args[0]), nil)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Agent enabled")
		return nil
	},
}

var agentsDisableCmd = &cobra.Command{
	Use:   "disable <agent-id>",
	Short: "Disable (deactivate) an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Post(fmt.Sprintf("/agents/%s/deactivate", args[0]), nil)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Agent disabled")
		return nil
	},
}

var agentsRotateCredsCmd = &cobra.Command{
	Use:   "rotate-credentials <agent-id>",
	Short: "Rotate agent API credentials",
	Long: `Issue a new API key or client secret for an agent. The current one stops
working at once; the new one is printed once.

For an API key agent, choose the new key's lifetime with --key-expires-in-days,
--key-expires-at or --key-never-expires. Without one, the new key keeps the
current expiry date, or gets 180 days when that date has already passed.`,
	Example: `  lumo agents rotate-credentials agt_3f9a51c07be24d18 --key-expires-in-days 90
  lumo agents rotate-credentials agt_3f9a51c07be24d18 --key-expires-at 2027-03-31`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{}
		if err := addKeyLifetime(cmd, body); err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		var payload interface{}
		if len(body) > 0 {
			payload = body
		}
		resp, err := c.Post(fmt.Sprintf("/agents/%s/rotate-credentials", args[0]), payload)
		if err != nil {
			return err
		}
		// Always show full output since it contains the new credentials
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var agentsTokenCmd = &cobra.Command{
	Use:   "token <agent-id>",
	Short: "Generate a bearer token for an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		// Field names follow the admin API: expiresIn, scopes, resource.
		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetInt("ttl"); v > 0 {
			body["expiresIn"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("scopes"); len(v) > 0 {
			body["scopes"] = v
		}
		if v, _ := cmd.Flags().GetString("resource"); v != "" {
			body["resource"] = v
		}

		resp, err := c.Post(fmt.Sprintf("/agents/%s/token", args[0]), body)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

// Flags for the lifetime of an agent's API key, shared by create, update
// and rotate-credentials. They map to the admin API fields
// apiKeyExpiresInDays / apiKeyExpiresAt (null = never expires).
const (
	flagKeyExpiresInDays = "key-expires-in-days"
	flagKeyExpiresAt     = "key-expires-at"
	flagKeyNeverExpires  = "key-never-expires"
)

func addKeyLifetimeFlags(cmd *cobra.Command, what string) {
	cmd.Flags().Int(flagKeyExpiresInDays, 0, "Days until "+what+" expires, 1-3650 (180 is recommended)")
	cmd.Flags().String(flagKeyExpiresAt, "", "When "+what+" expires: a date (YYYY-MM-DD, end of that day UTC) or an RFC 3339 time")
	cmd.Flags().Bool(flagKeyNeverExpires, false, "Make "+what+" never expire (valid until revoked or rotated)")
	cmd.MarkFlagsMutuallyExclusive(flagKeyExpiresInDays, flagKeyExpiresAt, flagKeyNeverExpires)
}

// addKeyLifetime copies the chosen API key lifetime into an admin API body.
// It leaves the body alone when no lifetime flag was given.
func addKeyLifetime(cmd *cobra.Command, body map[string]interface{}) error {
	f := cmd.Flags()
	switch {
	case f.Changed(flagKeyExpiresInDays):
		days, _ := f.GetInt(flagKeyExpiresInDays)
		if days < 1 || days > 3650 {
			return fmt.Errorf("--%s must be between 1 and 3650 (use --%s for no expiry)", flagKeyExpiresInDays, flagKeyNeverExpires)
		}
		body["apiKeyExpiresInDays"] = days
	case f.Changed(flagKeyExpiresAt):
		raw, _ := f.GetString(flagKeyExpiresAt)
		at, err := parseKeyExpiry(raw)
		if err != nil {
			return err
		}
		if !at.After(time.Now()) {
			return fmt.Errorf("--%s must be in the future", flagKeyExpiresAt)
		}
		body["apiKeyExpiresAt"] = at.UTC().Format(time.RFC3339)
	case f.Changed(flagKeyNeverExpires):
		if never, _ := f.GetBool(flagKeyNeverExpires); never {
			body["apiKeyExpiresInDays"] = nil
		}
	}
	return nil
}

// parseKeyExpiry accepts a date (the key works until the end of that day,
// UTC) or a full RFC 3339 time.
func parseKeyExpiry(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if d, err := time.Parse("2006-01-02", raw); err == nil {
		return d.Add(24*time.Hour - time.Second), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("--%s must be a date (2027-03-31) or an RFC 3339 time (2027-03-31T12:00:00Z), got %q", flagKeyExpiresAt, raw)
}

func init() {
	agentsListCmd.Flags().String("search", "", "Search by name")

	agentsCreateCmd.Flags().String("name", "", "Agent name (required)")
	agentsCreateCmd.Flags().String("type", "", "Identity type: api_key (default), client_secret, workload, aauth, private_key_jwt or custom")
	agentsCreateCmd.Flags().String("description", "", "Agent description")
	agentsCreateCmd.Flags().StringSlice("capabilities", nil, "Agent capabilities")
	agentsCreateCmd.Flags().Bool("act-on-behalf", false, "Allow acting on behalf of users")
	addKeyLifetimeFlags(agentsCreateCmd, "the API key")

	agentsUpdateCmd.Flags().String("name", "", "Agent name")
	agentsUpdateCmd.Flags().String("description", "", "Agent description")
	agentsUpdateCmd.Flags().StringSlice("capabilities", nil, "Agent capabilities")
	addKeyLifetimeFlags(agentsUpdateCmd, "the current API key")

	addKeyLifetimeFlags(agentsRotateCredsCmd, "the new API key")

	agentsTokenCmd.Flags().Int("ttl", 0, "Token lifetime in seconds, 60-3600 (default 3600); up to 86400 together with --resource")
	agentsTokenCmd.Flags().StringSlice("scopes", nil, "Narrow the token to these agent capabilities")
	agentsTokenCmd.Flags().String("resource", "", "Bind the token to a registered MCP server (its Resource URI); the LumoAuth API then refuses the token")

	agentsCmd.AddCommand(
		agentsListCmd, agentsGetCmd, agentsCreateCmd, agentsUpdateCmd,
		agentsDeleteCmd, agentsEnableCmd, agentsDisableCmd,
		agentsRotateCredsCmd, agentsTokenCmd,
	)
	rootCmd.AddCommand(agentsCmd)
}
