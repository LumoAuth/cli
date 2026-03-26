package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:     "logs",
	Aliases: []string{"audit-logs", "audit"},
	Short:   "View audit logs",
	Long:    "List, view, and export tenant audit logs.",
}

var logsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit log entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		q := url.Values{}
		if v, _ := cmd.Flags().GetString("action"); v != "" {
			q.Set("action", v)
		}
		if v, _ := cmd.Flags().GetString("user"); v != "" {
			q.Set("userId", v)
		}
		if v, _ := cmd.Flags().GetString("from"); v != "" {
			q.Set("from", v)
		}
		if v, _ := cmd.Flags().GetString("to"); v != "" {
			q.Set("to", v)
		}
		if v, _ := cmd.Flags().GetString("status"); v != "" {
			q.Set("status", v)
		}
		if v, _ := cmd.Flags().GetInt("page"); v > 0 {
			q.Set("page", fmt.Sprintf("%d", v))
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			q.Set("limit", fmt.Sprintf("%d", v))
		}

		resp, err := c.Get("/audit-logs", q)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID        interface{} `json:"id"`
				Action    string      `json:"action"`
				Status    string      `json:"status"`
				UserEmail string      `json:"userEmail"`
				IP        string      `json:"ipAddress"`
				CreatedAt string      `json:"createdAt"`
			} `json:"data"`
			Meta struct {
				Total      int `json:"total"`
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, l := range result.Data {
			rows[i] = []string{
				fmt.Sprintf("%v", l.ID),
				l.Action,
				l.Status,
				l.UserEmail,
				l.IP,
				l.CreatedAt,
			}
		}
		p.PrintTable([]string{"ID", "Action", "Status", "User", "IP", "Time"}, rows)
		p.PrintPagination(result.Meta.Total, result.Meta.Page, result.Meta.TotalPages)
		return nil
	},
}

var logsGetCmd = &cobra.Command{
	Use:   "get <log-id>",
	Short: "Get audit log entry details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get(fmt.Sprintf("/audit-logs/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var logsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get audit log statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		q := url.Values{}
		if v, _ := cmd.Flags().GetString("from"); v != "" {
			q.Set("from", v)
		}
		if v, _ := cmd.Flags().GetString("to"); v != "" {
			q.Set("to", v)
		}

		resp, err := c.Get("/audit-logs/stats", q)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var logsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export audit logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		q := url.Values{}
		if v, _ := cmd.Flags().GetString("from"); v != "" {
			q.Set("from", v)
		}
		if v, _ := cmd.Flags().GetString("to"); v != "" {
			q.Set("to", v)
		}
		if v, _ := cmd.Flags().GetString("export-format"); v != "" {
			q.Set("format", v)
		}

		resp, err := c.Get("/audit-logs/export", q)
		if err != nil {
			return err
		}

		// Export outputs raw data
		fmt.Print(string(resp))
		return nil
	},
}

func init() {
	logsListCmd.Flags().String("action", "", "Filter by action type (e.g., login, user.created)")
	logsListCmd.Flags().String("user", "", "Filter by user ID or email")
	logsListCmd.Flags().String("from", "", "Start date (ISO 8601)")
	logsListCmd.Flags().String("to", "", "End date (ISO 8601)")
	logsListCmd.Flags().String("status", "", "Filter by status (success, failure)")
	logsListCmd.Flags().Int("page", 1, "Page number")
	logsListCmd.Flags().Int("limit", 25, "Results per page")

	logsStatsCmd.Flags().String("from", "", "Start date")
	logsStatsCmd.Flags().String("to", "", "End date")

	logsExportCmd.Flags().String("from", "", "Start date")
	logsExportCmd.Flags().String("to", "", "End date")
	logsExportCmd.Flags().String("export-format", "json", "Export format: json, csv")

	logsCmd.AddCommand(logsListCmd, logsGetCmd, logsStatsCmd, logsExportCmd)
	rootCmd.AddCommand(logsCmd)
}
