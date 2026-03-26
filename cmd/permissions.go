package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var permissionsCmd = &cobra.Command{
	Use:     "permissions",
	Aliases: []string{"permission", "perms", "perm"},
	Short:   "Manage permissions",
	Long:    "List, create, update, and delete permission definitions.",
}

var permissionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all permissions",
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
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			q.Set("limit", fmt.Sprintf("%d", v))
		}

		resp, err := c.Get("/permissions", q)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID          interface{} `json:"id"`
				Name        string      `json:"name"`
				Slug        string      `json:"slug"`
				Description string      `json:"description"`
			} `json:"data"`
			Meta struct {
				Total      int `json:"total"`
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, p := range result.Data {
			rows[i] = []string{fmt.Sprintf("%v", p.ID), p.Slug, p.Name, truncate(p.Description, 45)}
		}
		p.PrintTable([]string{"ID", "Slug", "Name", "Description"}, rows)
		return nil
	},
}

var permissionsGetCmd = &cobra.Command{
	Use:   "get <permission-id-or-slug>",
	Short: "Get permission details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get(fmt.Sprintf("/permissions/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var permissionsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new permission",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		slug, _ := cmd.Flags().GetString("slug")
		name, _ := cmd.Flags().GetString("name")
		if slug == "" {
			return fmt.Errorf("--slug is required")
		}
		if name == "" {
			name = slug
		}

		body := map[string]interface{}{"slug": slug, "name": name}
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			body["description"] = v
		}

		resp, err := c.Post("/permissions", body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Permission '%s' created", slug))
		return nil
	},
}

var permissionsUpdateCmd = &cobra.Command{
	Use:   "update <permission-id>",
	Short: "Update a permission",
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
		if len(body) == 0 {
			return fmt.Errorf("no fields to update")
		}

		resp, err := c.Patch(fmt.Sprintf("/permissions/%s", args[0]), body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Permission updated")
		return nil
	},
}

var permissionsDeleteCmd = &cobra.Command{
	Use:   "delete <permission-id>",
	Short: "Delete a permission",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/permissions/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Permission deleted")
		return nil
	},
}

func init() {
	permissionsListCmd.Flags().Int("page", 1, "Page number")
	permissionsListCmd.Flags().Int("limit", 25, "Results per page")

	permissionsCreateCmd.Flags().String("slug", "", "Permission slug, e.g. document.edit (required)")
	permissionsCreateCmd.Flags().String("name", "", "Permission display name")
	permissionsCreateCmd.Flags().String("description", "", "Permission description")

	permissionsUpdateCmd.Flags().String("name", "", "Permission name")
	permissionsUpdateCmd.Flags().String("description", "", "Permission description")

	permissionsCmd.AddCommand(permissionsListCmd, permissionsGetCmd, permissionsCreateCmd, permissionsUpdateCmd, permissionsDeleteCmd)
	rootCmd.AddCommand(permissionsCmd)
}
