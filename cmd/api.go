package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <PATH> [--data '{...}']",
	Short: "Make raw API requests",
	Long: `Make raw HTTP requests to any LumoAuth API endpoint.
This is useful for AI agents or accessing endpoints not covered by named commands.

The PATH is relative to the base URL. Include the full tenant path.

Examples:
  lumo api GET /t/acme-corp/api/v1/admin/users
  lumo api POST /t/acme-corp/api/v1/admin/roles --data '{"name":"Editor"}'
  lumo api DELETE /t/acme-corp/api/v1/admin/users/123`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		method := strings.ToUpper(args[0])
		path := args[1]

		var body interface{}
		data, _ := cmd.Flags().GetString("data")
		if data != "" {
			var parsed interface{}
			if err := json.Unmarshal([]byte(data), &parsed); err != nil {
				return fmt.Errorf("invalid JSON in --data: %w", err)
			}
			body = parsed
		}

		resp, err := c.RawRequest(method, path, body)
		if err != nil {
			return err
		}

		p := getPrinter()
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

func init() {
	apiCmd.Flags().StringP("data", "d", "", "JSON request body")
	rootCmd.AddCommand(apiCmd)
}
