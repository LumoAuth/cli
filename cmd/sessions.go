package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"session"},
	Short:   "Manage active sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		q := url.Values{}
		if v, _ := cmd.Flags().GetString("user"); v != "" {
			resp, err := c.Get(fmt.Sprintf("/users/%s/sessions", v), nil)
			if err != nil {
				return err
			}
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		if v, _ := cmd.Flags().GetInt("page"); v > 0 {
			q.Set("page", fmt.Sprintf("%d", v))
		}
		resp, err := c.Get("/sessions", q)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var sessionsRevokeCmd = &cobra.Command{
	Use:  "revoke <session-id>",
	Short: "Revoke a session",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/sessions/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Session revoked")
		return nil
	},
}

var sessionsRevokeAllCmd = &cobra.Command{
	Use:   "revoke-all",
	Short: "Revoke all sessions for a user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		user, _ := cmd.Flags().GetString("user")
		if user == "" {
			return fmt.Errorf("--user is required")
		}
		resp, err := c.Delete(fmt.Sprintf("/users/%s/sessions", user))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("All sessions revoked")
		return nil
	},
}

// Tokens
var tokensCmd = &cobra.Command{
	Use:     "tokens",
	Aliases: []string{"token"},
	Short:   "Manage OAuth tokens",
}

var tokensListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		q := url.Values{}
		if v, _ := cmd.Flags().GetString("user"); v != "" {
			q.Set("userId", v)
		}
		if v, _ := cmd.Flags().GetString("client"); v != "" {
			q.Set("clientId", v)
		}
		resp, err := c.Get("/tokens", q)
		if err != nil {
			return err
		}
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

var tokensRevokeCmd = &cobra.Command{
	Use:  "revoke <token-id>",
	Short: "Revoke a token",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()
		resp, err := c.Delete(fmt.Sprintf("/tokens/%s", args[0]))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("Token revoked")
		return nil
	},
}

func init() {
	sessionsListCmd.Flags().String("user", "", "Filter by user ID")
	sessionsListCmd.Flags().Int("page", 1, "Page number")
	sessionsRevokeAllCmd.Flags().String("user", "", "User ID (required)")
	tokensListCmd.Flags().String("user", "", "Filter by user ID")
	tokensListCmd.Flags().String("client", "", "Filter by client ID")

	sessionsCmd.AddCommand(sessionsListCmd, sessionsRevokeCmd, sessionsRevokeAllCmd)
	tokensCmd.AddCommand(tokensListCmd, tokensRevokeCmd)
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(tokensCmd)
}
