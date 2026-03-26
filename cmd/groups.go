package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var groupsCmd = &cobra.Command{
	Use:     "groups",
	Aliases: []string{"group"},
	Short:   "Manage groups",
	Long:    "List, create, update, delete groups and manage group membership.",
}

var groupsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all groups",
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

		resp, err := c.Get("/groups", q)
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
				MemberCount int         `json:"memberCount"`
			} `json:"data"`
			Meta struct {
				Total      int `json:"total"`
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, g := range result.Data {
			rows[i] = []string{
				fmt.Sprintf("%v", g.ID),
				g.Name,
				g.Slug,
				truncate(g.Description, 40),
				fmt.Sprintf("%d", g.MemberCount),
			}
		}
		p.PrintTable([]string{"ID", "Name", "Slug", "Description", "Members"}, rows)
		p.PrintPagination(result.Meta.Total, result.Meta.Page, result.Meta.TotalPages)
		return nil
	},
}

var groupsGetCmd = &cobra.Command{
	Use:   "get <group-id-or-slug>",
	Short: "Get group details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/groups/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var groupsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new group",
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

		resp, err := c.Post("/groups", body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Group '%s' created", name))
		return nil
	},
}

var groupsUpdateCmd = &cobra.Command{
	Use:   "update <group-id>",
	Short: "Update a group",
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

		resp, err := c.Patch(fmt.Sprintf("/groups/%s", args[0]), body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Group updated")
		return nil
	},
}

var groupsDeleteCmd = &cobra.Command{
	Use:   "delete <group-id>",
	Short: "Delete a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Delete(fmt.Sprintf("/groups/%s", args[0]))
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Group deleted")
		return nil
	},
}

// ── Members sub-command ──────────────────────────────────────────────

var groupsMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage group members",
}

var groupsMembersListCmd = &cobra.Command{
	Use:   "list <group-id>",
	Short: "List members of a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/groups/%s/members", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var groupsMembersAddCmd = &cobra.Command{
	Use:   "add <group-id>",
	Short: "Add members to a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		users, _ := cmd.Flags().GetStringSlice("users")
		if len(users) == 0 {
			return fmt.Errorf("--users is required")
		}

		resp, err := c.Post(fmt.Sprintf("/groups/%s/members", args[0]), map[string]interface{}{"users": users})
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Added %s to group", strings.Join(users, ", ")))
		return nil
	},
}

var groupsMembersRemoveCmd = &cobra.Command{
	Use:   "remove <group-id>",
	Short: "Remove members from a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		users, _ := cmd.Flags().GetStringSlice("users")
		if len(users) == 0 {
			return fmt.Errorf("--users is required")
		}

		resp, err := c.Delete(fmt.Sprintf("/groups/%s/members", args[0]))
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Removed %s from group", strings.Join(users, ", ")))
		return nil
	},
}

func init() {
	groupsListCmd.Flags().String("search", "", "Search by name")
	groupsListCmd.Flags().Int("page", 1, "Page number")
	groupsListCmd.Flags().Int("limit", 25, "Results per page")

	groupsCreateCmd.Flags().String("name", "", "Group name (required)")
	groupsCreateCmd.Flags().String("description", "", "Group description")

	groupsUpdateCmd.Flags().String("name", "", "Group name")
	groupsUpdateCmd.Flags().String("description", "", "Group description")

	groupsMembersAddCmd.Flags().StringSlice("users", nil, "User IDs or emails to add")
	groupsMembersRemoveCmd.Flags().StringSlice("users", nil, "User IDs or emails to remove")

	groupsMembersCmd.AddCommand(groupsMembersListCmd, groupsMembersAddCmd, groupsMembersRemoveCmd)
	groupsCmd.AddCommand(groupsListCmd, groupsGetCmd, groupsCreateCmd, groupsUpdateCmd, groupsDeleteCmd, groupsMembersCmd)
	rootCmd.AddCommand(groupsCmd)
}
