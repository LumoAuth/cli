package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var appsCmd = &cobra.Command{
	Use:     "apps",
	Aliases: []string{"app", "clients"},
	Short:   "Manage OAuth applications",
	Long:    "List, create, update, delete OAuth client applications and manage their secrets.",
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all OAuth applications",
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
		if v, _ := cmd.Flags().GetInt("page"); v > 0 {
			q.Set("page", fmt.Sprintf("%d", v))
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			q.Set("limit", fmt.Sprintf("%d", v))
		}

		resp, err := c.Get("/clients", q)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID       string `json:"id"`
				ClientID string `json:"clientId"`
				Name     string `json:"name"`
				Type     string `json:"applicationType"`
				Active   bool   `json:"isActive"`
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
			rows[i] = []string{
				a.ClientID,
				a.Name,
				a.Type,
				boolIcon(a.Active),
			}
		}
		p.PrintTable([]string{"Client ID", "Name", "Type", "Active"}, rows)
		p.PrintPagination(result.Meta.Total, result.Meta.Page, result.Meta.TotalPages)
		return nil
	},
}

var appsGetCmd = &cobra.Command{
	Use:   "get <client-id>",
	Short: "Get application details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get(fmt.Sprintf("/clients/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var appsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new OAuth application",
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
			body["applicationType"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("redirect-uris"); len(v) > 0 {
			body["redirectUris"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("grant-types"); len(v) > 0 {
			body["grantTypes"] = v
		}

		resp, err := c.Post("/clients", body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Application '%s' created", name))
		return nil
	},
}

var appsUpdateCmd = &cobra.Command{
	Use:   "update <client-id>",
	Short: "Update an application",
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
		if v, _ := cmd.Flags().GetStringSlice("redirect-uris"); len(v) > 0 {
			body["redirectUris"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("grant-types"); len(v) > 0 {
			body["grantTypes"] = v
		}
		if len(body) == 0 {
			return fmt.Errorf("no fields to update")
		}

		resp, err := c.Patch(fmt.Sprintf("/clients/%s", args[0]), body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Application updated")
		return nil
	},
}

var appsDeleteCmd = &cobra.Command{
	Use:   "delete <client-id>",
	Short: "Delete an application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/clients/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Application deleted")
		return nil
	},
}

var appsRotateSecretCmd = &cobra.Command{
	Use:   "rotate-secret <client-id>",
	Short: "Rotate client secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Post(fmt.Sprintf("/clients/%s/rotate-secret", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

func init() {
	appsListCmd.Flags().String("search", "", "Search by name")
	appsListCmd.Flags().Int("page", 1, "Page number")
	appsListCmd.Flags().Int("limit", 25, "Results per page")

	appsCreateCmd.Flags().String("name", "", "Application name (required)")
	appsCreateCmd.Flags().String("type", "web", "Application type: web, spa, native, m2m")
	appsCreateCmd.Flags().StringSlice("redirect-uris", nil, "Redirect URIs")
	appsCreateCmd.Flags().StringSlice("grant-types", nil, "Grant types")

	appsUpdateCmd.Flags().String("name", "", "Application name")
	appsUpdateCmd.Flags().StringSlice("redirect-uris", nil, "Redirect URIs")
	appsUpdateCmd.Flags().StringSlice("grant-types", nil, "Grant types")

	appsCmd.AddCommand(appsListCmd, appsGetCmd, appsCreateCmd, appsUpdateCmd, appsDeleteCmd, appsRotateSecretCmd)
	rootCmd.AddCommand(appsCmd)
}
