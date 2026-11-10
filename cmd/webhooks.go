package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var webhooksCmd = &cobra.Command{
	Use:     "webhooks",
	Aliases: []string{"webhook", "wh"},
	Short:   "Manage webhooks",
	Long:    "List, create, update, delete webhooks for event notifications.",
}

var webhooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all webhooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		q := url.Values{}
		if v, _ := cmd.Flags().GetInt("page"); v > 0 {
			q.Set("page", fmt.Sprintf("%d", v))
		}

		resp, err := c.Get("/webhooks", q)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID     interface{} `json:"id"`
				URL    string      `json:"url"`
				Events []string    `json:"events"`
				Active bool        `json:"isActive"`
			} `json:"data"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, w := range result.Data {
			evts := truncate(fmt.Sprintf("%v", w.Events), 40)
			rows[i] = []string{fmt.Sprintf("%v", w.ID), truncate(w.URL, 45), evts, boolIcon(w.Active)}
		}
		p.PrintTable([]string{"ID", "URL", "Events", "Active"}, rows)
		return nil
	},
}

var webhooksGetCmd = &cobra.Command{
	Use:   "get <webhook-id>",
	Short: "Get webhook details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get(fmt.Sprintf("/webhooks/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new webhook",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		webhookURL, _ := cmd.Flags().GetString("url")
		if webhookURL == "" {
			return fmt.Errorf("--url is required")
		}
		events, _ := cmd.Flags().GetStringSlice("events")
		if len(events) == 0 {
			return fmt.Errorf("--events is required")
		}

		// LumoAuth generates the signing secret and stores no description, so
		// --secret and --description are deprecated and not sent.
		body := map[string]interface{}{
			"url":    webhookURL,
			"events": events,
		}

		resp, err := c.Post("/webhooks", body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		// The create response is the only place where the signing secret
		// appears, so a table must show it too.
		var created struct {
			Data struct {
				ID     interface{} `json:"id"`
				Secret string      `json:"secret"`
			} `json:"data"`
		}
		_ = json.Unmarshal(resp, &created)
		p.PrintSuccess(fmt.Sprintf("Webhook %v created", created.Data.ID))
		if created.Data.Secret != "" {
			fmt.Printf("Signing secret: %s\n", created.Data.Secret)
			p.PrintNote("Store the signing secret now to verify webhook signatures. LumoAuth does not show it again.")
		}
		return nil
	},
}

var webhooksUpdateCmd = &cobra.Command{
	Use:   "update <webhook-id>",
	Short: "Update a webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("url"); v != "" {
			body["url"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("events"); len(v) > 0 {
			body["events"] = v
		}
		if len(body) == 0 {
			return fmt.Errorf("no fields to update: pass --url or --events")
		}

		resp, err := c.Patch(fmt.Sprintf("/webhooks/%s", args[0]), body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Webhook updated")
		return nil
	},
}

var webhooksDeleteCmd = &cobra.Command{
	Use:   "delete <webhook-id>",
	Short: "Delete a webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/webhooks/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Webhook deleted")
		return nil
	},
}

func init() {
	webhooksListCmd.Flags().Int("page", 1, "Page number")

	webhooksCreateCmd.Flags().String("url", "", "Webhook URL (required)")
	webhooksCreateCmd.Flags().StringSlice("events", nil, "Event types to subscribe to (required)")
	webhooksCreateCmd.Flags().String("description", "", "Webhook description")
	webhooksCreateCmd.Flags().String("secret", "", "Webhook signing secret")
	_ = webhooksCreateCmd.Flags().MarkDeprecated("description", "the API does not store a webhook description; the flag is ignored")
	_ = webhooksCreateCmd.Flags().MarkDeprecated("secret", "LumoAuth generates the signing secret and prints it once; the flag is ignored")

	webhooksUpdateCmd.Flags().String("url", "", "Webhook URL")
	webhooksUpdateCmd.Flags().StringSlice("events", nil, "Event types")
	webhooksUpdateCmd.Flags().String("description", "", "Webhook description")
	_ = webhooksUpdateCmd.Flags().MarkDeprecated("description", "the API does not store a webhook description; the flag is ignored")

	webhooksCmd.AddCommand(webhooksListCmd, webhooksGetCmd, webhooksCreateCmd, webhooksUpdateCmd, webhooksDeleteCmd)
	rootCmd.AddCommand(webhooksCmd)
}
