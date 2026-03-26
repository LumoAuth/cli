package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

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

		var result struct {
			Data []struct {
				ID           string `json:"id"`
				AgentID      string `json:"agentId"`
				Name         string `json:"name"`
				Status       string `json:"status"`
				Type         string `json:"type"`
				Capabilities []string `json:"capabilities"`
			} `json:"data"`
			Meta struct {
				Total      int `json:"total"`
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, a := range result.Data {
			caps := "—"
			if len(a.Capabilities) > 0 {
				caps = truncate(fmt.Sprintf("%v", a.Capabilities), 35)
			}
			rows[i] = []string{a.AgentID, a.Name, a.Status, a.Type, caps}
		}
		p.PrintTable([]string{"Agent ID", "Name", "Status", "Type", "Capabilities"}, rows)
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
			body["type"] = v
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
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Post(fmt.Sprintf("/agents/%s/rotate-credentials", args[0]), nil)
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

		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetInt("ttl"); v > 0 {
			body["ttl"] = v
		}

		resp, err := c.Post(fmt.Sprintf("/agents/%s/token", args[0]), body)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

func init() {
	agentsListCmd.Flags().String("search", "", "Search by name")

	agentsCreateCmd.Flags().String("name", "", "Agent name (required)")
	agentsCreateCmd.Flags().String("type", "", "Agent type")
	agentsCreateCmd.Flags().String("description", "", "Agent description")
	agentsCreateCmd.Flags().StringSlice("capabilities", nil, "Agent capabilities")
	agentsCreateCmd.Flags().Bool("act-on-behalf", false, "Allow acting on behalf of users")

	agentsUpdateCmd.Flags().String("name", "", "Agent name")
	agentsUpdateCmd.Flags().String("description", "", "Agent description")
	agentsUpdateCmd.Flags().StringSlice("capabilities", nil, "Agent capabilities")

	agentsTokenCmd.Flags().Int("ttl", 0, "Token TTL in seconds")

	agentsCmd.AddCommand(
		agentsListCmd, agentsGetCmd, agentsCreateCmd, agentsUpdateCmd,
		agentsDeleteCmd, agentsEnableCmd, agentsDisableCmd,
		agentsRotateCredsCmd, agentsTokenCmd,
	)
	rootCmd.AddCommand(agentsCmd)
}
