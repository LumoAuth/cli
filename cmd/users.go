package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:     "users",
	Aliases: []string{"user", "u"},
	Short:   "Manage tenant users",
	Long:    "List, create, update, delete, block/unblock users and manage their roles and groups.",
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
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
		if v, _ := cmd.Flags().GetString("role"); v != "" {
			q.Set("role", v)
		}
		if v, _ := cmd.Flags().GetString("group"); v != "" {
			q.Set("group", v)
		}
		if v, _ := cmd.Flags().GetInt("page"); v > 0 {
			q.Set("page", fmt.Sprintf("%d", v))
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			q.Set("limit", fmt.Sprintf("%d", v))
		}
		if cmd.Flags().Changed("blocked") {
			v, _ := cmd.Flags().GetBool("blocked")
			q.Set("blocked", fmt.Sprintf("%t", v))
		}
		if v, _ := cmd.Flags().GetString("sort"); v != "" {
			q.Set("sortBy", v)
		}

		resp, err := c.Get("/users", q)
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
				Email    string `json:"email"`
				Name     string `json:"name"`
				Active   bool   `json:"isActive"`
				Blocked  bool   `json:"blocked"`
				Verified bool   `json:"emailVerified"`
				MFA      bool   `json:"mfaEnabled"`
				Logins   int    `json:"loginCount"`
			} `json:"data"`
			Meta struct {
				Total      int `json:"total"`
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, u := range result.Data {
			status := "active"
			if u.Blocked {
				status = "blocked"
			} else if !u.Active {
				status = "inactive"
			}
			name := u.Name
			if name == "" {
				name = "—"
			}
			rows[i] = []string{
				u.ID[:8] + "…",
				u.Email,
				name,
				status,
				boolIcon(u.Verified),
				boolIcon(u.MFA),
				fmt.Sprintf("%d", u.Logins),
			}
		}

		p.PrintTable(
			[]string{"ID", "Email", "Name", "Status", "Verified", "MFA", "Logins"},
			rows,
		)
		p.PrintPagination(result.Meta.Total, result.Meta.Page, result.Meta.TotalPages)
		return nil
	},
}

var usersGetCmd = &cobra.Command{
	Use:   "get <user-id-or-email>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/users/%s", args[0]), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data map[string]interface{} `json:"data"`
		}
		json.Unmarshal(resp, &result)

		for k, v := range result.Data {
			fmt.Printf("  %-20s %v\n", k+":", v)
		}
		return nil
	},
}

var usersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		body := map[string]interface{}{}

		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			return fmt.Errorf("--email is required")
		}
		body["email"] = email

		if v, _ := cmd.Flags().GetString("name"); v != "" {
			body["name"] = v
		}
		if v, _ := cmd.Flags().GetString("given-name"); v != "" {
			body["givenName"] = v
		}
		if v, _ := cmd.Flags().GetString("family-name"); v != "" {
			body["familyName"] = v
		}
		if v, _ := cmd.Flags().GetString("username"); v != "" {
			body["username"] = v
		}
		if v, _ := cmd.Flags().GetString("password"); v != "" {
			body["password"] = v
		}
		if v, _ := cmd.Flags().GetString("phone"); v != "" {
			body["phoneNumber"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("roles"); len(v) > 0 {
			body["roles"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("groups"); len(v) > 0 {
			body["groups"] = v
		}
		if cmd.Flags().Changed("verified") {
			v, _ := cmd.Flags().GetBool("verified")
			body["emailVerified"] = v
		}

		resp, err := c.Post("/users", body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess(fmt.Sprintf("User %s created", email))
		return nil
	},
}

var usersUpdateCmd = &cobra.Command{
	Use:   "update <user-id>",
	Short: "Update a user",
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
		if v, _ := cmd.Flags().GetString("email"); v != "" {
			body["email"] = v
		}
		if v, _ := cmd.Flags().GetString("given-name"); v != "" {
			body["givenName"] = v
		}
		if v, _ := cmd.Flags().GetString("family-name"); v != "" {
			body["familyName"] = v
		}
		if v, _ := cmd.Flags().GetString("phone"); v != "" {
			body["phoneNumber"] = v
		}
		if v, _ := cmd.Flags().GetString("picture"); v != "" {
			body["picture"] = v
		}
		if cmd.Flags().Changed("active") {
			v, _ := cmd.Flags().GetBool("active")
			body["isActive"] = v
		}
		if cmd.Flags().Changed("verified") {
			v, _ := cmd.Flags().GetBool("verified")
			body["emailVerified"] = v
		}

		if len(body) == 0 {
			return fmt.Errorf("no fields to update. Use flags like --name, --email, etc.")
		}

		resp, err := c.Patch(fmt.Sprintf("/users/%s", args[0]), body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess("User updated")
		return nil
	},
}

var usersDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Delete(fmt.Sprintf("/users/%s", args[0]))
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess("User deleted")
		return nil
	},
}

var usersBlockCmd = &cobra.Command{
	Use:   "block <user-id>",
	Short: "Block a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Post(fmt.Sprintf("/users/%s/block", args[0]), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess("User blocked")
		return nil
	},
}

var usersUnblockCmd = &cobra.Command{
	Use:   "unblock <user-id>",
	Short: "Unblock a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Post(fmt.Sprintf("/users/%s/unblock", args[0]), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess("User unblocked")
		return nil
	},
}

var usersSetPasswordCmd = &cobra.Command{
	Use:   "set-password <user-id>",
	Short: "Set a user's password",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		pw, _ := cmd.Flags().GetString("password")
		if pw == "" {
			return fmt.Errorf("--password is required")
		}

		resp, err := c.Put(fmt.Sprintf("/users/%s/password", args[0]), map[string]string{"password": pw})
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess("Password updated")
		return nil
	},
}

var usersMfaResetCmd = &cobra.Command{
	Use:   "mfa-reset <user-id>",
	Short: "Reset a user's MFA enrollment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Post(fmt.Sprintf("/users/%s/mfa/reset", args[0]), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess("MFA reset")
		return nil
	},
}

// ── Sub-resource: roles ──────────────────────────────────────────────

var usersRolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "Manage user role assignments",
}

var usersRolesGetCmd = &cobra.Command{
	Use:   "get <user-id>",
	Short: "Get roles assigned to a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/users/%s/roles", args[0]), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID   interface{} `json:"id"`
				Name string      `json:"name"`
				Slug string      `json:"slug"`
				Desc string      `json:"description"`
			} `json:"data"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, r := range result.Data {
			rows[i] = []string{fmt.Sprintf("%v", r.ID), r.Name, r.Slug, truncate(r.Desc, 40)}
		}
		p.PrintTable([]string{"ID", "Name", "Slug", "Description"}, rows)
		return nil
	},
}

var usersRolesSetCmd = &cobra.Command{
	Use:   "set <user-id>",
	Short: "Set roles for a user (replaces existing)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		roles, _ := cmd.Flags().GetStringSlice("roles")
		if len(roles) == 0 {
			return fmt.Errorf("--roles is required (comma-separated role slugs)")
		}

		resp, err := c.Put(fmt.Sprintf("/users/%s/roles", args[0]), map[string]interface{}{"roles": roles})
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess(fmt.Sprintf("Roles updated: %s", strings.Join(roles, ", ")))
		return nil
	},
}

// ── Sub-resource: groups ─────────────────────────────────────────────

var usersGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "Manage user group memberships",
}

var usersGroupsGetCmd = &cobra.Command{
	Use:   "get <user-id>",
	Short: "Get groups a user belongs to",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/users/%s/groups", args[0]), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID   interface{} `json:"id"`
				Name string      `json:"name"`
				Slug string      `json:"slug"`
				Desc string      `json:"description"`
			} `json:"data"`
		}
		json.Unmarshal(resp, &result)

		rows := make([][]string, len(result.Data))
		for i, g := range result.Data {
			rows[i] = []string{fmt.Sprintf("%v", g.ID), g.Name, g.Slug, truncate(g.Desc, 40)}
		}
		p.PrintTable([]string{"ID", "Name", "Slug", "Description"}, rows)
		return nil
	},
}

var usersGroupsSetCmd = &cobra.Command{
	Use:   "set <user-id>",
	Short: "Set groups for a user (replaces existing)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		groups, _ := cmd.Flags().GetStringSlice("groups")
		if len(groups) == 0 {
			return fmt.Errorf("--groups is required (comma-separated group slugs)")
		}

		resp, err := c.Put(fmt.Sprintf("/users/%s/groups", args[0]), map[string]interface{}{"groups": groups})
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		p.PrintSuccess(fmt.Sprintf("Groups updated: %s", strings.Join(groups, ", ")))
		return nil
	},
}

// ── Helpers ──────────────────────────────────────────────────────────

func boolIcon(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func init() {
	// List flags
	usersListCmd.Flags().String("search", "", "Search by email, name, or username")
	usersListCmd.Flags().String("role", "", "Filter by role slug")
	usersListCmd.Flags().String("group", "", "Filter by group slug")
	usersListCmd.Flags().Bool("blocked", false, "Filter blocked users")
	usersListCmd.Flags().Int("page", 1, "Page number")
	usersListCmd.Flags().Int("limit", 25, "Results per page")
	usersListCmd.Flags().String("sort", "", "Sort by field (email, name, createdAt, lastLoginAt)")

	// Create flags
	usersCreateCmd.Flags().String("email", "", "Email address (required)")
	usersCreateCmd.Flags().String("name", "", "Display name")
	usersCreateCmd.Flags().String("given-name", "", "Given (first) name")
	usersCreateCmd.Flags().String("family-name", "", "Family (last) name")
	usersCreateCmd.Flags().String("username", "", "Username (defaults to email)")
	usersCreateCmd.Flags().String("password", "", "Password")
	usersCreateCmd.Flags().String("phone", "", "Phone number")
	usersCreateCmd.Flags().StringSlice("roles", nil, "Role slugs to assign (comma-separated)")
	usersCreateCmd.Flags().StringSlice("groups", nil, "Group slugs to assign (comma-separated)")
	usersCreateCmd.Flags().Bool("verified", false, "Mark email as verified")

	// Update flags
	usersUpdateCmd.Flags().String("name", "", "Display name")
	usersUpdateCmd.Flags().String("email", "", "Email address")
	usersUpdateCmd.Flags().String("given-name", "", "Given name")
	usersUpdateCmd.Flags().String("family-name", "", "Family name")
	usersUpdateCmd.Flags().String("phone", "", "Phone number")
	usersUpdateCmd.Flags().String("picture", "", "Profile picture URL")
	usersUpdateCmd.Flags().Bool("active", true, "Active status")
	usersUpdateCmd.Flags().Bool("verified", false, "Email verified status")

	// Set password flags
	usersSetPasswordCmd.Flags().String("password", "", "New password (required)")

	// Roles set flags
	usersRolesSetCmd.Flags().StringSlice("roles", nil, "Role slugs (comma-separated)")

	// Groups set flags
	usersGroupsSetCmd.Flags().StringSlice("groups", nil, "Group slugs (comma-separated)")

	// Wire up sub-commands
	usersRolesCmd.AddCommand(usersRolesGetCmd, usersRolesSetCmd)
	usersGroupsCmd.AddCommand(usersGroupsGetCmd, usersGroupsSetCmd)

	usersCmd.AddCommand(
		usersListCmd, usersGetCmd, usersCreateCmd, usersUpdateCmd,
		usersDeleteCmd, usersBlockCmd, usersUnblockCmd,
		usersSetPasswordCmd, usersMfaResetCmd,
		usersRolesCmd, usersGroupsCmd,
	)
	rootCmd.AddCommand(usersCmd)
}
