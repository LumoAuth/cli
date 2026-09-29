package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Per-user MFA administration (server MFA refactor, admin API §3.4):
// list/remove a user's authenticators and issue temporary access codes.
// Admins can no longer switch MFA off for a user; `users mfa-reset` stays
// only to explain that.

var usersMfaResetCmd = &cobra.Command{
	Use:        "mfa-reset <user-id>",
	Short:      "Removed: admins can no longer disable a user's MFA",
	Deprecated: "admins can no longer disable MFA; use 'lumo users tap' or 'lumo users authenticators remove'",
	Args:       cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Fails locally: the server answers 410 Gone (mfa_reset_removed) and
		// there is nothing useful to send it.
		return fmt.Errorf(`MFA reset has been removed: admins can no longer switch MFA off for a user.
  Lost device or locked out:  lumo users tap <user-id> --reason "identity verified by ..."
                              (a one-time code that lets the user sign in and enroll a new factor)
  Remove one authenticator:   lumo users authenticators list <user-id>
                              lumo users authenticators remove <user-id> <authenticator-id>`)
	},
}

var usersAuthenticatorsCmd = &cobra.Command{
	Use:     "authenticators",
	Aliases: []string{"authenticator", "authn", "factors"},
	Short:   "View and remove a user's MFA authenticators",
	Long: `View and remove a user's MFA authenticators (passkeys, push devices,
authenticator apps, phone numbers, email codes, recovery codes).

Removing an authenticator does not switch MFA off: if the organization requires
MFA the user is asked to enroll again at their next sign-in. To get a locked-out
user back in, issue a temporary access code with 'lumo users tap'.`,
}

var usersAuthenticatorsListCmd = &cobra.Command{
	Use:   "list <user-id-or-email>",
	Short: "List a user's authenticators",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/users/%s/authenticators", url.PathEscape(args[0])), nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data []struct {
				ID          string  `json:"id"`
				Type        string  `json:"type"`
				DisplayName string  `json:"display_name"`
				State       string  `json:"state"`
				IsDefault   bool    `json:"is_default"`
				Tier        int     `json:"tier"`
				TierLabel   string  `json:"tier_label"`
				LastUsedAt  *string `json:"last_used_at"`
			} `json:"data"`
			Status  string  `json:"status"`
			Default *string `json:"default"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}

		rows := make([][]string, len(result.Data))
		for i, a := range result.Data {
			name := a.DisplayName
			if name == "" {
				name = "—"
			}
			tier := fmt.Sprintf("%d", a.Tier)
			if a.TierLabel != "" {
				tier = fmt.Sprintf("%d %s", a.Tier, a.TierLabel)
			}
			lastUsed := "never"
			if a.LastUsedAt != nil && *a.LastUsedAt != "" {
				lastUsed = formatTimestamp(*a.LastUsedAt)
			}
			def := ""
			if a.IsDefault {
				def = "✓"
			}
			rows[i] = []string{a.ID, a.Type, name, a.State, tier, def, lastUsed}
		}
		p.PrintTable([]string{"ID", "Type", "Name", "State", "Tier", "Default", "Last used"}, rows)

		fmt.Printf("\nStatus: %s\n", describeMfaStatus(result.Status))
		return nil
	},
}

var usersAuthenticatorsRemoveCmd = &cobra.Command{
	Use:   "remove <user-id> <authenticator-id>",
	Short: "Remove one of a user's authenticators (e.g. a lost phone)",
	Long: `Remove one of a user's authenticators, e.g. a lost phone.

The authenticator id is the reference shown by 'lumo users authenticators list'
(for example totp:12 or passkey:7). The user's trusted devices are revoked and
the user is notified. This does not switch MFA off: if MFA is required the user
enrolls again at their next sign-in.

Asks for confirmation on a terminal; pass --yes in scripts.`,
	Aliases: []string{"rm", "delete"},
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID, authID := args[0], args[1]
		if !strings.Contains(authID, ":") {
			return fmt.Errorf("authenticator id %q should look like <type>:<number> (e.g. totp:12); see 'lumo users authenticators list %s'", authID, userID)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			if !isInteractive() {
				return fmt.Errorf("refusing to remove authenticator %s without confirmation; re-run with --yes", authID)
			}
			answer := promptString(fmt.Sprintf("Remove authenticator %s from user %s? They will be notified and may need to enroll again. [y/N]: ", authID, userID))
			if !isYes(answer) {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Delete(fmt.Sprintf("/users/%s/authenticators/%s", url.PathEscape(userID), url.PathEscape(authID)))
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data struct {
				Type        string `json:"type"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
		}
		_ = json.Unmarshal(resp, &result)
		what := authID
		if result.Data.DisplayName != "" {
			what = fmt.Sprintf("%s (%s)", authID, result.Data.DisplayName)
		}
		p.PrintSuccess(fmt.Sprintf("Authenticator %s removed", what))
		return nil
	},
}

var usersTapCmd = &cobra.Command{
	Use:   "tap <user-id-or-email>",
	Short: "Issue a temporary access code for a locked-out user",
	Long: `Issue a temporary access code (TAP) for a user who lost their device.

The code stands in for the second factor once (or, with --multi-use, until it
expires) and only lets the user sign in and enroll a new factor. It is shown
exactly once — hand it to the user over a verified channel.

A reason is required; the issue is audited (mfa.tap.issued) and the user is
notified. When you are signed in with 'lumo login', the server requires a fresh
MFA check of your own: pass the id of an approved step_up challenge with
--mfa-challenge (organization API keys are not subject to step-up).

Examples:
  lumo users tap jane@acme.com --reason "Lost phone; identity verified by video call"
  lumo users tap 5f1c... --reason "New laptop" --ttl 240 --multi-use`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		if strings.TrimSpace(reason) == "" {
			return fmt.Errorf("--reason is required (it is recorded in the audit log and shown to the user)")
		}
		body := map[string]interface{}{"reason": reason}
		if cmd.Flags().Changed("ttl") {
			ttl, _ := cmd.Flags().GetInt("ttl")
			if ttl < 5 || ttl > 10080 {
				return fmt.Errorf("--ttl must be between 5 and 10080 minutes (7 days)")
			}
			body["ttl_minutes"] = ttl
		}
		multi, _ := cmd.Flags().GetBool("multi-use")
		body["single_use"] = !multi

		c, err := getClient()
		if err != nil {
			return err
		}
		if challenge, _ := cmd.Flags().GetString("mfa-challenge"); challenge != "" {
			c = c.WithHeader("X-MFA-Challenge", challenge)
		}
		p := getPrinter()

		resp, err := c.Post(fmt.Sprintf("/users/%s/temporary-access-code", url.PathEscape(args[0])), body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data struct {
				ID        string `json:"id"`
				Code      string `json:"code"`
				ExpiresAt string `json:"expires_at"`
				SingleUse bool   `json:"single_use"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp, &result); err != nil || result.Data.Code == "" {
			return fmt.Errorf("unexpected response from server: %s", truncate(string(resp), 200))
		}
		printTemporaryAccessCode(args[0], result.Data.Code, result.Data.ExpiresAt, result.Data.SingleUse)
		return nil
	},
}

// printTemporaryAccessCode renders the code so it can't be missed, with its
// expiry and the fact that it will not be shown again.
func printTemporaryAccessCode(user, code, expiresAt string, singleUse bool) {
	use := "single use"
	if !singleUse {
		use = "reusable until it expires"
	}
	width := len(code) + 8
	fmt.Println()
	fmt.Printf("  Temporary access code for %s\n\n", user)
	fmt.Printf("    ┌%s┐\n", strings.Repeat("─", width))
	fmt.Printf("    │    %s    │\n", code)
	fmt.Printf("    └%s┘\n\n", strings.Repeat("─", width))
	if expiresAt != "" {
		fmt.Printf("  Expires: %s (%s)\n", formatTimestamp(expiresAt), use)
	} else {
		fmt.Printf("  Usage:   %s\n", use)
	}
	fmt.Println("  This code is shown once and cannot be retrieved again. Give it to the user")
	fmt.Println("  over a verified channel; it only lets them sign in and enroll a new factor.")
}

// describeMfaStatus explains the authenticator status summary.
func describeMfaStatus(s string) string {
	switch s {
	case "protected":
		return "protected (has a backup sign-in method)"
	case "add_backup":
		return "add_backup (only one factor — a lost device locks the user out)"
	case "at_risk":
		return "at_risk (no usable second factor)"
	case "":
		return "—"
	}
	return s
}

// formatTimestamp renders an RFC 3339 timestamp in local time, or returns it
// unchanged when it doesn't parse.
func formatTimestamp(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04 MST")
}

func init() {
	usersAuthenticatorsRemoveCmd.Flags().BoolP("yes", "y", false, "Remove without asking for confirmation")

	usersTapCmd.Flags().String("reason", "", "Why the code is issued (required; audited and shown to the user)")
	usersTapCmd.Flags().Int("ttl", 60, "Minutes until the code expires (5–10080)")
	usersTapCmd.Flags().Bool("multi-use", false, "Allow the code to be used more than once until it expires")
	usersTapCmd.Flags().String("mfa-challenge", "", "Id of an approved step_up MFA challenge (needed with 'lumo login' tokens)")

	usersAuthenticatorsCmd.AddCommand(usersAuthenticatorsListCmd, usersAuthenticatorsRemoveCmd)
	usersCmd.AddCommand(usersAuthenticatorsCmd, usersTapCmd)
}
