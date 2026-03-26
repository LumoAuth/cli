package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var rolesCmd = &cobra.Command{
	Use:     "roles",
	Aliases: []string{"role"},
	Short:   "Manage roles",
	Long:    "List, create, update, and delete roles. Manage permission assignments.",
}

var rolesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all roles",
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

		resp, err := c.Get("/roles", q)
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
				UserCount   int         `json:"userCount"`
				Permissions int         `json:"permissionCount"`
			} `json:"data"`
			Meta struct {
				Total      int `json:"total"`
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, r := range result.Data {
			rows[i] = []string{
				fmt.Sprintf("%v", r.ID),
				r.Name,
				r.Slug,
				truncate(r.Description, 40),
				fmt.Sprintf("%d", r.UserCount),
				fmt.Sprintf("%d", r.Permissions),
			}
		}
		p.PrintTable([]string{"ID", "Name", "Slug", "Description", "Users", "Perms"}, rows)
		p.PrintPagination(result.Meta.Total, result.Meta.Page, result.Meta.TotalPages)
		return nil
	},
}

var rolesGetCmd = &cobra.Command{
	Use:   "get <role-id-or-slug>",
	Short: "Get role details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/roles/%s", args[0]), nil)
		if err != nil {
			return err
		}

		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var rolesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new role",
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
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			body["description"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("permissions"); len(v) > 0 {
			body["permissions"] = v
		}

		resp, err := c.Post("/roles", body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Role '%s' created", name))
		return nil
	},
}

var rolesUpdateCmd = &cobra.Command{
	Use:   "update <role-id>",
	Short: "Update a role",
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
		if v, _ := cmd.Flags().GetStringSlice("permissions"); len(v) > 0 {
			body["permissions"] = v
		}
		if len(body) == 0 {
			return fmt.Errorf("no fields to update")
		}

		resp, err := c.Patch(fmt.Sprintf("/roles/%s", args[0]), body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Role updated")
		return nil
	},
}

var rolesDeleteCmd = &cobra.Command{
	Use:   "delete <role-id>",
	Short: "Delete a role",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Delete(fmt.Sprintf("/roles/%s", args[0]))
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Role deleted")
		return nil
	},
}

func init() {
	rolesListCmd.Flags().Int("page", 1, "Page number")
	rolesListCmd.Flags().Int("limit", 25, "Results per page")

	rolesCreateCmd.Flags().String("name", "", "Role name (required)")
	rolesCreateCmd.Flags().String("description", "", "Role description")
	rolesCreateCmd.Flags().StringSlice("permissions", nil, "Permission slugs to assign")

	rolesUpdateCmd.Flags().String("name", "", "Role name")
	rolesUpdateCmd.Flags().String("description", "", "Role description")
	rolesUpdateCmd.Flags().StringSlice("permissions", nil, "Permission slugs")

	rolesCmd.AddCommand(rolesListCmd, rolesGetCmd, rolesCreateCmd, rolesUpdateCmd, rolesDeleteCmd)
	rootCmd.AddCommand(rolesCmd)
}
