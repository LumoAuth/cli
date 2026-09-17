package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:     "org",
	Aliases: []string{"organization"},
	Short:   "View and update the organization profile and top-level settings",
	Long: `Read and change the organization you are signed in to: its name and the
small set of organization-level settings the Admin API allows.

Only documented, non-security keys are writable: display_name, timezone,
locale, security.*, email.*, audit_log_retention_days, audit_log_auto_delete,
agent_governance.*, allow_root_admin_login. Anything else (or any key ending
in _hash/_secret/_token/_key/_password) is refused with the offending keys
listed, and nothing is saved. Object-valued keys merge key by key.

Examples:
  lumo org get
  lumo org update --name "ACME Corporation"
  lumo org update --set timezone=Europe/Berlin --set locale=de
  lumo org update --set security.dpop_require_nonce=true
  lumo org update --set allow_root_admin_login=false
  lumo org update --data '{"settings":{"audit_log_retention_days":365}}'`,
}

var orgGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the organization profile and settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		resp, err := c.Get("/organization", nil)
		if err != nil {
			return err
		}
		getPrinter().PrintResult(json.RawMessage(resp))
		return nil
	},
}

var orgUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the organization name or settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if data, _ := cmd.Flags().GetString("data"); data != "" {
			if err := json.Unmarshal([]byte(data), &body); err != nil {
				return fmt.Errorf("invalid JSON in --data: %w", err)
			}
		}
		if name, _ := cmd.Flags().GetString("name"); name != "" {
			body["name"] = name
		}
		if pairs, _ := cmd.Flags().GetStringArray("set"); len(pairs) > 0 {
			kv, err := parseSetFlags(pairs)
			if err != nil {
				return err
			}
			// `name` is the only top-level field; everything else lives under settings.
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
		}
		if len(body) == 0 {
			return fmt.Errorf("nothing to update: pass --name, --set key=value, or --data '{...}'")
		}

		resp, err := c.Patch("/organization", body)
		if err != nil {
			return err
		}
		p := getPrinter()
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Organization updated")
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

func init() {
	orgUpdateCmd.Flags().String("name", "", "New organization name")
	orgUpdateCmd.Flags().StringArray("set", nil, "Settings key=value (repeatable; dotted keys nest, e.g. security.dpop_require_nonce=true)")
	orgUpdateCmd.Flags().String("data", "", "Raw JSON body, e.g. '{\"name\":\"…\",\"settings\":{…}}'")
	orgCmd.AddCommand(orgGetCmd, orgUpdateCmd)
	rootCmd.AddCommand(orgCmd)
}
