package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/lumoauth/cli/internal/output"
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
		if v, ok := body["permissions"].([]string); ok {
			warnSkippedPermissions(p, v, resp)
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
	Long: `Update the name, the description or the permissions of a role.

--permissions replaces the permissions of the role with the slugs that you
list. LumoAuth keeps any permission of the role that you cannot grant
yourself, and it skips slugs that do not exist or that you cannot grant;
lumo prints a warning for each skipped slug.`,
	Args: cobra.ExactArgs(1),
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
		perms, _ := cmd.Flags().GetStringSlice("permissions")
		if len(body) == 0 && len(perms) == 0 {
			return fmt.Errorf("no fields to update: pass --name, --description or --permissions")
		}

		var resp []byte
		if len(body) > 0 {
			// PATCH /roles/{id} changes name and description only; the server
			// ignores a permissions field there.
			resp, err = c.Patch(fmt.Sprintf("/roles/%s", url.PathEscape(args[0])), body)
			if err != nil {
				return err
			}
		}
		if len(perms) > 0 {
			resp, err = c.Put(fmt.Sprintf("/roles/%s/permissions", url.PathEscape(args[0])), map[string]interface{}{"permissions": perms})
			if err != nil {
				return err
			}
			warnSkippedPermissions(p, perms, resp)
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Role updated")
		return nil
	},
}

// warnSkippedPermissions prints a note for each requested permission slug
// that is missing from the role in resp. The server skips unknown slugs and
// permissions that the caller cannot grant without an error.
func warnSkippedPermissions(p *output.Printer, requested []string, resp []byte) {
	for _, slug := range skippedPermissions(requested, resp) {
		p.PrintNote(fmt.Sprintf("! Permission %q was not assigned: it does not exist, or you cannot grant it.", slug))
	}
}

func skippedPermissions(requested []string, resp []byte) []string {
	var out struct {
		Data struct {
			Permissions *[]struct {
				Slug string `json:"slug"`
			} `json:"permissions"`
		} `json:"data"`
	}
	if json.Unmarshal(resp, &out) != nil || out.Data.Permissions == nil {
		return nil // response without a permission list: nothing to compare
	}
	have := map[string]bool{}
	for _, perm := range *out.Data.Permissions {
		have[perm.Slug] = true
	}
	var skipped []string
	for _, slug := range requested {
		slug = strings.TrimSpace(slug)
		if slug != "" && !have[slug] {
			skipped = append(skipped, slug)
		}
	}
	return skipped
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
