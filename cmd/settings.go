package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// settingsResources maps the names people type to Admin API paths. Aliases
// point at the same path; the first name of each group is the canonical one
// shown in help.
var settingsResources = map[string]string{
	"general":        "/settings/general",
	"authentication": "/settings/authentication",
	"auth":           "/settings/authentication",
	"security":       "/settings/security",
	"email":          "/settings/email",
	"branding":       "/settings/branding",
	"scim":           "/settings/scim",
	"organization":   "/organization",
	"org":            "/organization",
	"tenant":         "/organization",
}

func settingsResourceNames() string {
	seen := map[string]bool{}
	var names []string
	for name, path := range settingsResources {
		if seen[path] {
			continue
		}
		// Prefer the canonical spelling for each path.
		canonical := name
		for n, p := range settingsResources {
			if p == path && len(n) > len(canonical) && n != "tenant" {
				canonical = n
			}
		}
		seen[path] = true
		names = append(names, canonical)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

var settingsCmd = &cobra.Command{
	Use:     "settings",
	Aliases: []string{"setting"},
	Short:   "View and update organization settings by area",
	Long: `View and update one area of organization settings at a time.

Areas:
  general         Name, display name, time zone, locale
  authentication  Password policy, sessions, passkeys (alias: auth);
                  the MFA policy has its own command: 'lumo mfa policy'
  security        OAuth hardening switches (e.g. dpop_require_nonce)
  email           Sender name/address and provider
  branding        Login page logo, colours, texts (sanitised server-side)
  scim            Provisioning policy: allow_user_creation / updates / deletion,
                  allow_group_operations, protected_attributes, deprovisioning_action
  organization    The organization profile (same as 'lumo org')

Reads need the admin:settings:read scope, writes admin:settings:write.

Examples:
  lumo settings get scim
  lumo settings update scim --set allow_user_deletion=false --set deprovisioning_action=deactivate
  lumo settings update authentication --set session_timeout=86400 --set password_min_length=12
  lumo settings update branding --data '{"primary_color":"#0f766e"}'
  lumo settings get all         # every area in one document`,
}

var settingsGetCmd = &cobra.Command{
	Use:   "get <area>",
	Short: "Get settings for an area (or 'all')",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		path := "/settings"
		if args[0] != "all" {
			var ok bool
			path, ok = settingsResources[args[0]]
			if !ok {
				return fmt.Errorf("unknown settings area %q (use one of: %s, all)", args[0], settingsResourceNames())
			}
		}
		resp, err := c.Get(path, nil)
		if err != nil {
			return err
		}
		getPrinter().PrintResult(json.RawMessage(resp))
		return nil
	},
}

var settingsUpdateCmd = &cobra.Command{
	Use:   "update <area>",
	Short: "Update settings for an area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		path, ok := settingsResources[args[0]]
		if !ok {
			return fmt.Errorf("unknown settings area %q (use one of: %s)", args[0], settingsResourceNames())
		}

		body := map[string]interface{}{}
		if data, _ := cmd.Flags().GetString("data"); data != "" {
			if err := json.Unmarshal([]byte(data), &body); err != nil {
				return fmt.Errorf("invalid JSON in --data: %w", err)
			}
		}
		if pairs, _ := cmd.Flags().GetStringArray("set"); len(pairs) > 0 {
			kv, err := parseSetFlags(pairs)
			if err != nil {
				return err
			}
			if path == "/organization" {
				// The organization endpoint nests everything but `name` under settings.
				if n, ok := kv["name"]; ok {
					body["name"] = n
					delete(kv, "name")
				}
				if len(kv) > 0 {
					settings, _ := body["settings"].(map[string]interface{})
					if settings == nil {
						settings = map[string]interface{}{}
					}
					mergeInto(settings, kv)
					body["settings"] = settings
				}
			} else {
				mergeInto(body, kv)
			}
		}
		if len(body) == 0 {
			return fmt.Errorf("nothing to update: pass --set key=value (repeatable) or --data '{...}'")
		}

		resp, err := c.Patch(path, body)
		if err != nil {
			return err
		}
		p := getPrinter()
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Settings for %q updated", args[0]))
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

func init() {
	settingsUpdateCmd.Flags().StringArray("set", nil, "key=value to change (repeatable; dotted keys nest)")
	settingsUpdateCmd.Flags().String("data", "", "Raw JSON object with the fields to change")
	settingsCmd.AddCommand(settingsGetCmd, settingsUpdateCmd)
	rootCmd.AddCommand(settingsCmd)
}
