package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// `lumo profile` — named credential profiles for people who work across
// multiple orgs/deployments (consultants, agencies, EU+US operators).
// Profiles live in ~/.lumoauth/credentials.yaml:
//
//   current_profile: acme
//   profiles:
//     acme:    {org_id: acme-corp,  base_url: https://app.lumoauth.dev,    access_token: ...}
//     clientb: {org_id: clientb,    base_url: https://eu.app.lumoauth.dev, access_token: ...}
//
// Selection precedence: --profile flag > LUMO_PROFILE env > current_profile.
// The --org flag still overrides the profile's org for a single command.

var profileCmd = &cobra.Command{
	Use:     "profile",
	Aliases: []string{"profiles"},
	Short:   "Manage named credential profiles (multi-org)",
	Long: `Manage named credential profiles stored in ~/.lumoauth/credentials.yaml.

Each profile holds its own org, base URL, and login tokens, so you can
switch between organizations (or US/EU/self-hosted deployments) without
re-authenticating:

  lumo login --profile acme --org acme-corp
  lumo login --profile clientb --org clientb-corp
  lumo profile use acme            # make acme the default
  lumo users list --profile clientb   # one-off against another profile

Profile selection precedence: --profile flag > LUMO_PROFILE env var >
current_profile in the credentials file.`,
}

var (
	profileCreateOrgID   string
	profileCreateBaseURL string
	profileCreateUse     bool
	profileDeleteForce   bool
)

var profileListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := config.LoadStore()
		if err != nil {
			return err
		}
		active := config.ActiveProfileName()

		p := getPrinter()
		if p.IsJSON() {
			type row struct {
				Name    string `json:"name"`
				Current bool   `json:"current"`
				Active  bool   `json:"active"`
				OrgID   string `json:"org_id"`
				BaseURL string `json:"base_url"`
				Email   string `json:"user_email,omitempty"`
				Status  string `json:"status"`
			}
			rows := make([]row, 0, len(store.Profiles))
			for _, name := range store.ProfileNames() {
				c := store.Profiles[name]
				rows = append(rows, row{
					Name:    name,
					Current: name == store.CurrentProfile,
					Active:  name == active,
					OrgID:   c.OrgID,
					BaseURL: c.BaseURL,
					Email:   c.UserEmail,
					Status:  profileStatus(c),
				})
			}
			p.PrintResult(map[string]interface{}{"data": rows, "current_profile": store.CurrentProfile})
			return nil
		}

		if len(store.Profiles) == 0 {
			fmt.Fprintln(os.Stderr, "No profiles yet. Run 'lumo login' (or 'lumo profile create <name>') to create one.")
			return nil
		}
		headers := []string{"", "NAME", "ORG", "BASE URL", "STATUS"}
		var rows [][]string
		for _, name := range store.ProfileNames() {
			c := store.Profiles[name]
			marker := " "
			if name == active {
				marker = "*"
			}
			rows = append(rows, []string{marker, name, c.OrgID, c.BaseURL, profileStatus(c)})
		}
		p.PrintTable(headers, rows)
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the current profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.UseProfile(args[0]); err != nil {
			return err
		}
		getPrinter().PrintSuccess(fmt.Sprintf("Switched to profile %q", args[0]))
		return nil
	},
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new (token-less) profile",
	Long: `Create a named profile pinning an org and base URL. The profile has no
credentials yet — authenticate it with:

  lumo login --profile <name>

or use it with an API key (the profile supplies org_id/base_url):

  LUMO_API_KEY=lmk_... lumo users list --profile <name>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := config.CreateProfile(name, profileCreateOrgID, profileCreateBaseURL, profileCreateUse); err != nil {
			return err
		}
		getPrinter().PrintSuccess(fmt.Sprintf("Created profile %q. Authenticate it with 'lumo login --profile %s'", name, name))
		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a profile (and its stored tokens)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !profileDeleteForce && isInteractive() {
			answer := promptString(fmt.Sprintf("Delete profile %q and its stored tokens? [y/N]: ", name))
			if !isYes(answer) {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}
		if err := config.DeleteProfile(name); err != nil {
			return err
		}
		getPrinter().PrintSuccess(fmt.Sprintf("Deleted profile %q", name))
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show a profile's details (tokens masked)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := config.LoadStore()
		if err != nil {
			return err
		}
		name := config.ActiveProfileName()
		if len(args) == 1 {
			name = args[0]
		}
		c, ok := store.Profiles[name]
		if !ok {
			return fmt.Errorf("profile %q does not exist (have: %v)", name, store.ProfileNames())
		}

		p := getPrinter()
		if p.IsJSON() {
			p.PrintResult(map[string]interface{}{
				"name":       name,
				"current":    name == store.CurrentProfile,
				"org_id":     c.OrgID,
				"base_url":   c.BaseURL,
				"user_email": c.UserEmail,
				"status":     profileStatus(c),
				"expires_at": c.ExpiresAt,
			})
			return nil
		}

		fmt.Printf("Profile:    %s\n", name)
		fmt.Printf("Org:        %s\n", c.OrgID)
		fmt.Printf("Base URL:   %s\n", c.BaseURL)
		if c.UserEmail != "" {
			fmt.Printf("User:       %s\n", c.UserEmail)
		}
		fmt.Printf("Status:     %s\n", profileStatus(c))
		if !c.ExpiresAt.IsZero() {
			fmt.Printf("Expires at: %s\n", c.ExpiresAt.Format(time.RFC3339))
		}
		if name == store.CurrentProfile {
			fmt.Println("Current:    yes")
		}
		return nil
	},
}

// profileStatus summarizes a profile's auth state for listings.
func profileStatus(c *config.Credentials) string {
	switch {
	case !c.HasToken():
		return "no credentials"
	case c.IsExpired():
		return "expired"
	default:
		return "logged in"
	}
}

// isYes interprets a confirmation prompt answer.
func isYes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	}
	return false
}

// isInteractive reports whether stdin is a TTY — prompts are skipped in
// scripts/CI per the CLI's "no interactivity" convention for agents.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func init() {
	profileCreateCmd.Flags().StringVar(&profileCreateOrgID, "org", "", "Organization slug this profile targets")
	profileCreateCmd.Flags().StringVar(&profileCreateOrgID, "org-id", "", "Alias of --org")
	_ = profileCreateCmd.Flags().MarkHidden("org-id")
	profileCreateCmd.Flags().StringVar(&profileCreateBaseURL, "base-url", "", "LumoAuth base URL this profile targets")
	profileCreateCmd.Flags().BoolVar(&profileCreateUse, "use", false, "Make the new profile the current profile")
	profileDeleteCmd.Flags().BoolVarP(&profileDeleteForce, "force", "f", false, "Delete without confirmation")

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}
