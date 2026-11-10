package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var socialCmd = &cobra.Command{
	Use:     "social",
	Aliases: []string{"social-providers"},
	Short:   "Manage social login providers",
}

var socialListCmd = &cobra.Command{
	Use:   "list",
	Short: "List social login providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get("/social-providers", nil)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		var result struct {
			Data []struct {
				ID       interface{} `json:"id"`
				Provider string      `json:"provider"`
				Name     string      `json:"name"`
				Active   bool        `json:"isActive"`
			} `json:"data"`
		}
		json.Unmarshal(resp, &result)
		rows := make([][]string, len(result.Data))
		for i, s := range result.Data {
			rows[i] = []string{fmt.Sprintf("%v", s.ID), s.Provider, s.Name, boolIcon(s.Active)}
		}
		p.PrintTable([]string{"ID", "Provider", "Name", "Active"}, rows)
		return nil
	},
}

var socialGetCmd = &cobra.Command{
	Use:   "get <provider-id>",
	Short: "Get provider details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Get(fmt.Sprintf("/social-providers/%s", args[0]), nil)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var socialCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a social login provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		provider, _ := cmd.Flags().GetString("provider")
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		if provider == "" || clientID == "" || clientSecret == "" {
			return fmt.Errorf("--provider, --client-id, --client-secret are required")
		}
		displayName, _ := cmd.Flags().GetString("name")
		if displayName == "" {
			displayName = defaultProviderDisplayName(provider)
		}
		enable, _ := cmd.Flags().GetBool("enable")
		body := map[string]interface{}{
			"provider":     provider,
			"clientId":     clientID,
			"clientSecret": clientSecret,
			"displayName":  displayName, // required by the API
			"enabled":      enable,
		}
		if v, _ := cmd.Flags().GetStringSlice("scopes"); len(v) > 0 {
			body["requestedScopes"] = v
		}
		resp, err := c.Post("/social-providers", body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		if enable {
			p.PrintSuccess(fmt.Sprintf("Social provider '%s' created and enabled", provider))
		} else {
			p.PrintSuccess(fmt.Sprintf("Social provider '%s' created (disabled; pass --enable to show it on the sign-in page)", provider))
		}
		return nil
	},
}

// defaultProviderDisplayName turns a provider type such as "google" or
// "microsoft-entra" into a display name ("Google", "Microsoft Entra").
func defaultProviderDisplayName(provider string) string {
	words := strings.FieldsFunc(provider, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

var socialDeleteCmd = &cobra.Command{
	Use:   "delete <provider-id>",
	Short: "Delete a social provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/social-providers/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Social provider deleted")
		return nil
	},
}

func init() {
	socialCreateCmd.Flags().String("provider", "", "Provider type: google, github, etc. (required)")
	socialCreateCmd.Flags().String("client-id", "", "OAuth client ID (required)")
	socialCreateCmd.Flags().String("client-secret", "", "OAuth client secret (required)")
	socialCreateCmd.Flags().String("name", "", "Display name on the sign-in page (default: the provider type, capitalised)")
	socialCreateCmd.Flags().StringSlice("scopes", nil, "Scopes to request from the provider (default: openid,profile,email)")
	socialCreateCmd.Flags().Bool("enable", false, "Enable the provider at once (it is created disabled otherwise)")

	socialCmd.AddCommand(socialListCmd, socialGetCmd, socialCreateCmd, socialDeleteCmd)
	rootCmd.AddCommand(socialCmd)
}
