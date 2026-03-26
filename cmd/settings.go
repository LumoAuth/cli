package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:     "settings",
	Aliases: []string{"setting"},
	Short:   "View and update tenant settings",
	Long: `View and update tenant configuration, authentication settings, branding, and AI settings.

Subresources:
  tenant   - Tenant profile and metadata
  auth     - Authentication policies (MFA, sessions, password rules)
  branding - Login page branding (logo, colors, text)
  ai       - AI and agent settings`,
}

var settingsGetCmd = &cobra.Command{
	Use:   "get <resource>",
	Short: "Get settings for a resource (tenant, auth, branding, ai)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		path := resolveSettingsPath(args[0])
		if path == "" {
			return fmt.Errorf("unknown settings resource: %s (use: tenant, auth, branding, ai)", args[0])
		}

		resp, err := c.Get(path, nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var settingsUpdateCmd = &cobra.Command{
	Use:   "update <resource>",
	Short: "Update settings for a resource (tenant, auth, branding, ai)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		path := resolveSettingsPath(args[0])
		if path == "" {
			return fmt.Errorf("unknown settings resource: %s (use: tenant, auth, branding, ai)", args[0])
		}

		// Build body from flags
		body := map[string]interface{}{}

		// Generic key-value flags
		data, _ := cmd.Flags().GetString("data")
		if data != "" {
			if err := json.Unmarshal([]byte(data), &body); err != nil {
				return fmt.Errorf("invalid JSON in --data: %w", err)
			}
		}

		// Named flags for common settings
		if v, _ := cmd.Flags().GetString("name"); v != "" {
			body["name"] = v
		}
		if v, _ := cmd.Flags().GetString("display-name"); v != "" {
			body["displayName"] = v
		}
		if v, _ := cmd.Flags().GetString("logo-url"); v != "" {
			body["logoUrl"] = v
		}
		if v, _ := cmd.Flags().GetString("primary-color"); v != "" {
			body["primaryColor"] = v
		}
		if cmd.Flags().Changed("mfa-required") {
			v, _ := cmd.Flags().GetBool("mfa-required")
			body["mfaRequired"] = v
		}
		if v, _ := cmd.Flags().GetInt("session-lifetime"); v > 0 {
			body["sessionLifetime"] = v
		}

		if len(body) == 0 {
			return fmt.Errorf("no settings to update. Use --data '{...}' or named flags like --name, --mfa-required")
		}

		resp, err := c.Patch(path, body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Settings for '%s' updated", args[0]))
		return nil
	},
}

func resolveSettingsPath(resource string) string {
	switch resource {
	case "tenant":
		return "/tenant"
	case "auth":
		return "/settings/auth"
	case "branding":
		return "/settings/branding"
	case "ai":
		return "/settings/ai"
	default:
		return ""
	}
}

func init() {
	settingsUpdateCmd.Flags().String("data", "", "JSON object with settings to update")
	settingsUpdateCmd.Flags().String("name", "", "Tenant/resource name")
	settingsUpdateCmd.Flags().String("display-name", "", "Display name")
	settingsUpdateCmd.Flags().String("logo-url", "", "Logo URL (branding)")
	settingsUpdateCmd.Flags().String("primary-color", "", "Primary color (branding)")
	settingsUpdateCmd.Flags().Bool("mfa-required", false, "Require MFA (auth)")
	settingsUpdateCmd.Flags().Int("session-lifetime", 0, "Session lifetime in seconds (auth)")

	settingsCmd.AddCommand(settingsGetCmd, settingsUpdateCmd)
	rootCmd.AddCommand(settingsCmd)
}
