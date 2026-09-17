package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <PATH> [--data '{...}'] [--query k=v ...]",
	Short: "Make raw API requests (paths are relative to the Admin API)",
	Long: `Make an HTTP request to any LumoAuth endpoint with the CLI's credentials.

Paths are resolved the way you would expect:
  /users                      → /orgs/<org>/api/v1/admin/users      (Admin API)
  /orgs/<org>/api/v1/me       → as given                            (absolute API path)
  /api/v1/authz/check         → as given                            (global API)
  --raw /anything             → <base-url>/anything

The response is printed as returned (JSON), or rendered as a table in a terminal.
Use -o json in scripts; the exit code follows the HTTP status (see 'lumo --help').

Examples:
  lumo api GET /users --query search=jane --query limit=5
  lumo api POST /roles --data '{"name":"Editor"}'
  lumo api PATCH /organization --data '{"settings":{"allow_root_admin_login":false}}'
  lumo api GET /orgs/acme-corp/api/v1/.well-known/openid-configuration
  lumo api DELETE /users/01JF3K…`,
	Args: cobra.ExactArgs(2),
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

		query := url.Values{}
		pairs, _ := cmd.Flags().GetStringArray("query")
		for _, pair := range pairs {
			k, v, ok := strings.Cut(pair, "=")
			if !ok || k == "" {
				return fmt.Errorf("--query expects key=value, got %q", pair)
			}
			query.Add(k, v)
		}

		raw, _ := cmd.Flags().GetBool("raw")
		var resp json.RawMessage
		if raw {
			if len(query) > 0 {
				path += "?" + query.Encode()
			}
			resp, err = c.RawRequest(method, path, body)
		} else {
			resp, err = c.Request(method, path, query, body)
		}
		if err != nil {
			return err
		}

		p := getPrinter()
		if len(strings.TrimSpace(string(resp))) == 0 {
			p.PrintSuccess(fmt.Sprintf("%s %s", method, path))
			return nil
		}
		p.PrintResult(resp)
		return nil
	},
}

func init() {
	apiCmd.Flags().StringP("data", "d", "", "JSON request body")
	apiCmd.Flags().StringArrayP("query", "Q", nil, "Query parameter key=value (repeatable)")
	apiCmd.Flags().Bool("raw", false, "Treat PATH as relative to the base URL, not the Admin API")
	rootCmd.AddCommand(apiCmd)
}
