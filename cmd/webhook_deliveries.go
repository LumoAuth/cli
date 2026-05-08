package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

// `lumo webhooks deliveries list/get/replay` — wraps the Phase 1.4 server
// endpoints (GET/POST /admin/webhooks/{id}/deliveries[/replay]). Stripe
// equivalent: `stripe events resend evt_xxx`.

var webhooksDeliveriesCmd = &cobra.Command{
	Use:     "deliveries",
	Aliases: []string{"delivery"},
	Short:   "Inspect and replay webhook deliveries",
	Long: `View the per-delivery audit trail for a webhook (status, attempts,
response codes) and replay any failed or dead-lettered delivery.`,
}

var webhooksDeliveriesListCmd = &cobra.Command{
	Use:   "list <webhook-id>",
	Short: "List recent deliveries for a webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		q := url.Values{}
		if v, _ := cmd.Flags().GetString("status"); v != "" {
			q.Set("status", v)
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			q.Set("limit", fmt.Sprintf("%d", v))
		}

		resp, err := c.Get(fmt.Sprintf("/admin/webhooks/%s/deliveries", url.PathEscape(args[0])), q)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				DeliveryID string `json:"delivery_id"`
				Event      string `json:"event_type"`
				Status     string `json:"status"`
				Attempts   int    `json:"attempt_count"`
				Last       string `json:"last_attempt_at"`
			} `json:"data"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, d := range result.Data {
			rows[i] = []string{d.DeliveryID, d.Event, d.Status, fmt.Sprintf("%d", d.Attempts), d.Last}
		}
		p.PrintTable([]string{"Delivery", "Event", "Status", "Attempts", "Last attempt"}, rows)
		return nil
	},
}

var webhooksDeliveriesGetCmd = &cobra.Command{
	Use:   "get <webhook-id> <delivery-id>",
	Short: "Show delivery detail with attempt history",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		path := fmt.Sprintf("/admin/webhooks/%s/deliveries/%s",
			url.PathEscape(args[0]), url.PathEscape(args[1]))
		resp, err := c.Get(path, nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var webhooksDeliveriesReplayCmd = &cobra.Command{
	Use:   "replay <webhook-id> <delivery-id>",
	Short: "Replay a failed or dead-lettered delivery",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		path := fmt.Sprintf("/admin/webhooks/%s/deliveries/%s/replay",
			url.PathEscape(args[0]), url.PathEscape(args[1]))
		resp, err := c.Post(path, nil)
		if err != nil {
			return err
		}

		if p.IsTable() {
			p.PrintSuccess(fmt.Sprintf("Replay queued for delivery %s", args[1]))
			return nil
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

func init() {
	webhooksDeliveriesListCmd.Flags().String("status", "", "Filter by status (pending|delivered|failed|dead_lettered)")
	webhooksDeliveriesListCmd.Flags().Int("limit", 0, "Limit number of rows")

	webhooksDeliveriesCmd.AddCommand(
		webhooksDeliveriesListCmd,
		webhooksDeliveriesGetCmd,
		webhooksDeliveriesReplayCmd,
	)
	webhooksCmd.AddCommand(webhooksDeliveriesCmd)
}
