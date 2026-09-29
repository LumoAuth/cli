package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// Federated identity links (server: FederatedIdentityLinkService, admin API
// /users/{userId}/identities and /identities/legacy-saml). A link decides which
// external identity (SAML IdP + NameID, LDAP directory + DN, social provider)
// signs in as a user, so changing one revokes the user's sessions, notifies
// them, and — for 'lumo login' tokens — needs a fresh MFA check of your own.

const identityStepUpNote = `When you are signed in with 'lumo login', the server requires a fresh MFA
check of your own: pass the id of an approved step_up challenge with
--mfa-challenge (organization API keys are not subject to step-up).`

var usersIdentitiesCmd = &cobra.Command{
	Use:     "identities",
	Aliases: []string{"identity", "idents"},
	Short:   "View, link and unlink a user's SAML / LDAP / social identities",
	Long: `View, link and unlink the federated identities that sign in as a user:
a SAML identity provider + NameID, an LDAP directory + DN, or a social provider.

Linking or unlinking changes who can sign in as the account: the user is signed
out everywhere, notified by email, and the change is audited
(identity.link.created / identity.link.removed). You cannot change the links of
an account that holds permissions you do not have.`,
}

var usersIdentitiesListCmd = &cobra.Command{
	Use:   "list <user-id-or-email>",
	Short: "List a user's federated identities",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get(fmt.Sprintf("/users/%s/identities", url.PathEscape(args[0])), nil)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data struct {
				AuthSource *string           `json:"auth_source"`
				LdapOnly   bool              `json:"ldap_only"`
				Bindings   []identityBinding `json:"bindings"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}
		if len(result.Data.Bindings) == 0 {
			p.PrintNote("No federated identities linked.")
			return nil
		}
		rows := make([][]string, len(result.Data.Bindings))
		for i, b := range result.Data.Bindings {
			rows[i] = b.row()
		}
		p.PrintTable([]string{"Type", "Source", "Identity", "Notes"}, rows)
		return nil
	},
}

// identityBinding is one entry of the server's `bindings` list.
type identityBinding struct {
	Type           string  `json:"type"`
	IdpID          *int    `json:"idp_id"`
	IdpName        *string `json:"idp_name"`
	NameID         *string `json:"name_id"`
	Hashed         bool    `json:"hashed"`
	Legacy         bool    `json:"legacy"`
	Orphaned       bool    `json:"orphaned"`
	LdapConfigID   *int    `json:"ldap_config_id"`
	LdapConfigName *string `json:"ldap_config_name"`
	DN             *string `json:"dn"`
	LdapOnly       bool    `json:"ldap_only"`
	Provider       *string `json:"provider"`
	Subject        *string `json:"subject"`
}

func (b identityBinding) row() []string {
	str := func(s *string) string {
		if s == nil || *s == "" {
			return "—"
		}
		return *s
	}
	var source, identity string
	var notes []string
	switch b.Type {
	case "saml":
		switch {
		case b.IdpName != nil:
			source = fmt.Sprintf("%s (#%d)", *b.IdpName, derefInt(b.IdpID))
		case b.IdpID != nil:
			source = fmt.Sprintf("IdP #%d", *b.IdpID)
		default:
			source = "—"
		}
		identity = str(b.NameID)
		if b.Hashed {
			identity = "(hashed long NameID)"
		}
		if b.Legacy {
			notes = append(notes, "legacy: relink with 'lumo identities legacy-saml'")
		}
	case "ldap":
		if b.LdapConfigName != nil {
			source = fmt.Sprintf("%s (#%d)", *b.LdapConfigName, derefInt(b.LdapConfigID))
		} else if b.LdapConfigID != nil {
			source = fmt.Sprintf("directory #%d", *b.LdapConfigID)
		}
		identity = str(b.DN)
		if b.LdapOnly {
			notes = append(notes, "LDAP only (local password disabled)")
		}
	default:
		source = str(b.Provider)
		identity = str(b.Subject)
	}
	if b.Orphaned && !b.Legacy {
		notes = append(notes, "orphaned (IdP/directory deleted)")
	}
	return []string{b.Type, source, identity, strings.Join(notes, "; ")}
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

var usersIdentitiesLinkCmd = &cobra.Command{
	Use:   "link <user-id-or-email>",
	Short: "Link a SAML (IdP + NameID) or LDAP (directory + DN) identity",
	Long: `Link a SAML or LDAP identity to a user, replacing any existing link of that type.

SAML:  --saml-idp <idp-config-id> --name-id <NameID the IdP asserts for the user>
LDAP:  --ldap-config <directory-id> [--dn <distinguished name>] [--ldap-only]
       Without --dn the server looks the entry up in the directory by the
       user's email, then username. The local password keeps working unless
       --ldap-only is given.

Refused (HTTP 409) when another user already holds that identity, or when the
user is linked to a different federated source — unlink it first.
The user is signed out everywhere and notified.

` + identityStepUpNote + `

Examples:
  lumo users identities link jane@acme.com --saml-idp 3 --name-id jane@acme.com
  lumo users identities link jane@acme.com --ldap-config 1 --dn "uid=jane,ou=people,dc=acme,dc=com"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		samlIdp, _ := cmd.Flags().GetInt("saml-idp")
		nameID, _ := cmd.Flags().GetString("name-id")
		ldapConfig, _ := cmd.Flags().GetInt("ldap-config")
		dn, _ := cmd.Flags().GetString("dn")
		ldapOnly, _ := cmd.Flags().GetBool("ldap-only")

		isSaml := cmd.Flags().Changed("saml-idp") || cmd.Flags().Changed("name-id")
		isLdap := cmd.Flags().Changed("ldap-config") || cmd.Flags().Changed("dn") || cmd.Flags().Changed("ldap-only")
		var body map[string]interface{}
		switch {
		case isSaml && isLdap:
			return fmt.Errorf("pass either --saml-idp/--name-id or --ldap-config/--dn, not both")
		case isSaml:
			if !cmd.Flags().Changed("saml-idp") || strings.TrimSpace(nameID) == "" {
				return fmt.Errorf("a SAML link needs both --saml-idp <id> and --name-id <NameID>")
			}
			body = map[string]interface{}{"type": "saml", "idp_id": samlIdp, "name_id": nameID}
		case isLdap:
			if !cmd.Flags().Changed("ldap-config") {
				return fmt.Errorf("an LDAP link needs --ldap-config <id> (and optionally --dn)")
			}
			body = map[string]interface{}{"type": "ldap", "ldap_config_id": ldapConfig}
			if strings.TrimSpace(dn) != "" {
				body["dn"] = dn
			}
			if ldapOnly {
				body["ldap_only"] = true
			}
		default:
			return fmt.Errorf("pass --saml-idp <id> --name-id <NameID>, or --ldap-config <id> [--dn <DN>]")
		}

		c, err := getClient()
		if err != nil {
			return err
		}
		if challenge, _ := cmd.Flags().GetString("mfa-challenge"); challenge != "" {
			c = c.WithHeader("X-MFA-Challenge", challenge)
		}
		p := getPrinter()

		resp, err := c.Post(fmt.Sprintf("/users/%s/identities", url.PathEscape(args[0])), body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		var result struct {
			Data identityBinding `json:"data"`
		}
		_ = json.Unmarshal(resp, &result)
		row := result.Data.row()
		p.PrintSuccess(fmt.Sprintf("Linked %s identity %s (%s) to %s. The user was signed out everywhere and notified.", strings.ToUpper(row[0]), row[2], row[1], args[0]))
		return nil
	},
}

var usersIdentitiesUnlinkCmd = &cobra.Command{
	Use:   "unlink <user-id-or-email>",
	Short: "Unlink a user's SAML, LDAP or social identity",
	Long: `Unlink a user's SAML, LDAP or social identity (--type saml|ldap|social).

The user is signed out everywhere and notified. Unlinking LDAP also re-enables
the local password (which usually needs a reset). Asks for confirmation on a
terminal; pass --yes in scripts.

` + identityStepUpNote,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		typ, _ := cmd.Flags().GetString("type")
		switch typ {
		case "saml", "ldap", "social":
		case "":
			return fmt.Errorf("--type is required (saml, ldap or social)")
		default:
			return fmt.Errorf("--type must be saml, ldap or social (got %q)", typ)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			if !isInteractive() {
				return fmt.Errorf("refusing to unlink the %s identity of %s without confirmation; re-run with --yes", typ, args[0])
			}
			answer := promptString(fmt.Sprintf("Unlink the %s identity of %s? They will be signed out everywhere. [y/N]: ", typ, args[0]))
			if !isYes(answer) {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		c, err := getClient()
		if err != nil {
			return err
		}
		if challenge, _ := cmd.Flags().GetString("mfa-challenge"); challenge != "" {
			c = c.WithHeader("X-MFA-Challenge", challenge)
		}
		p := getPrinter()

		resp, err := c.Delete(fmt.Sprintf("/users/%s/identities/%s", url.PathEscape(args[0]), url.PathEscape(typ)))
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess(fmt.Sprintf("Unlinked the %s identity of %s. The user was signed out everywhere and notified.", typ, args[0]))
		return nil
	},
}

var identitiesCmd = &cobra.Command{
	Use:     "identities",
	Aliases: []string{"identity"},
	Short:   "Organization-wide federated identity tools",
	Long: `Organization-wide federated identity tools. Per-user links live under
'lumo users identities'.`,
}

var identitiesLegacySamlCmd = &cobra.Command{
	Use:   "legacy-saml",
	Short: "Report and relink users with legacy (bare NameID) SAML links",
	Long: `Report and relink users whose SAML link predates per-IdP binding and stores
only a bare NameID. While the organization has more than one SAML identity
provider these users are refused at SAML sign-in (audited as
saml_legacy_binding_ambiguous) until they are relinked to the IdP that issues
their NameID.

Without --relink this prints the report: each legacy user and the IdP(s) whose
allowed email domains claim their address ("suggested" when exactly one does).

With --relink (requires --idp) the users are rebound to that IdP, keeping their
NameID: the listed --user ids, or every legacy user whose email domain the IdP's
allowed email domains claim. --dry-run shows the plan without changing anything;
a real run needs --yes (or a confirmation on a terminal). Users you do not
outrank, or whose NameID is already linked at that IdP, are skipped.

` + identityStepUpNote + `

Examples:
  lumo identities legacy-saml
  lumo identities legacy-saml --idp 3 --relink --dry-run
  lumo identities legacy-saml --idp 3 --relink --yes
  lumo identities legacy-saml --idp 3 --relink --user 0192... --user 0193... --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		idp, _ := cmd.Flags().GetInt("idp")
		hasIdp := cmd.Flags().Changed("idp")
		relink, _ := cmd.Flags().GetBool("relink")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		yes, _ := cmd.Flags().GetBool("yes")
		users, _ := cmd.Flags().GetStringSlice("user")

		if !relink && (dryRun || yes || len(users) > 0) {
			return fmt.Errorf("--dry-run, --yes and --user only apply with --relink")
		}

		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		if !relink {
			q := url.Values{}
			if hasIdp {
				q.Set("idp_id", strconv.Itoa(idp))
			}
			resp, err := c.Get("/identities/legacy-saml", q)
			if err != nil {
				return err
			}
			if !p.IsTable() {
				p.PrintResult(json.RawMessage(resp))
				return nil
			}
			return printLegacySamlReport(resp, p.PrintTable, p.PrintNote)
		}

		if !hasIdp {
			return fmt.Errorf("--relink needs --idp <idp-config-id> (the IdP that issues these users' NameIDs)")
		}
		body := map[string]interface{}{"idp_id": idp, "dry_run": dryRun}
		if len(users) > 0 {
			body["user_ids"] = users
		} else {
			body["all_matching_domains"] = true
		}

		if !dryRun && !yes {
			if !isInteractive() {
				return fmt.Errorf("refusing to relink without confirmation; preview with --dry-run, then re-run with --yes")
			}
			answer := promptString(fmt.Sprintf("Relink legacy SAML users to IdP #%d? Each relinked user is signed out everywhere and notified. [y/N]: ", idp))
			if !isYes(answer) {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}
		if challenge, _ := cmd.Flags().GetString("mfa-challenge"); challenge != "" {
			c = c.WithHeader("X-MFA-Challenge", challenge)
		}

		resp, err := c.Post("/identities/legacy-saml", body)
		if err != nil {
			return err
		}
		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		return printLegacySamlRelink(resp, p.PrintTable, p.PrintSuccess, p.PrintNote)
	},
}

type legacySamlUser struct {
	UserID          string `json:"user_id"`
	Email           string `json:"email"`
	NameID          string `json:"name_id"`
	CandidateIdpIDs []int  `json:"candidate_idp_ids"`
	SuggestedIdpID  *int   `json:"suggested_idp_id"`
	Reason          string `json:"reason"`
}

func printLegacySamlReport(resp []byte, table func([]string, [][]string), note func(string)) error {
	var result struct {
		Data struct {
			IdpCount  int              `json:"idp_count"`
			Ambiguous bool             `json:"ambiguous"`
			Users     []legacySamlUser `json:"users"`
			Idps      []struct {
				ID          int      `json:"id"`
				DisplayName string   `json:"display_name"`
				Domains     []string `json:"allowed_email_domains"`
			} `json:"idps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}
	d := result.Data
	if len(d.Users) == 0 {
		note("No users with legacy SAML links.")
		return nil
	}
	names := map[int]string{}
	for _, idp := range d.Idps {
		names[idp.ID] = fmt.Sprintf("%s (#%d)", idp.DisplayName, idp.ID)
	}
	rows := make([][]string, len(d.Users))
	for i, u := range d.Users {
		suggested := "—"
		if u.SuggestedIdpID != nil {
			suggested = orName(names, *u.SuggestedIdpID)
		} else if len(u.CandidateIdpIDs) > 1 {
			parts := make([]string, len(u.CandidateIdpIDs))
			for j, id := range u.CandidateIdpIDs {
				parts[j] = orName(names, id)
			}
			suggested = "several: " + strings.Join(parts, ", ")
		}
		rows[i] = []string{u.UserID, u.Email, u.NameID, suggested}
	}
	table([]string{"User ID", "Email", "NameID", "Suggested IdP"}, rows)
	if d.Ambiguous {
		note(fmt.Sprintf("%d legacy user(s) cannot sign in with SAML: the organization has %d SAML IdPs. Relink with 'lumo identities legacy-saml --idp <id> --relink --dry-run'.", len(d.Users), d.IdpCount))
	} else {
		note("The organization has a single SAML IdP: these users are relinked automatically at their next SAML sign-in.")
	}
	return nil
}

func printLegacySamlRelink(resp []byte, table func([]string, [][]string), success, note func(string)) error {
	var result struct {
		Data struct {
			DryRun   bool             `json:"dry_run"`
			IdpID    int              `json:"idp_id"`
			Relinked []legacySamlUser `json:"relinked"`
			Skipped  []legacySamlUser `json:"skipped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}
	d := result.Data
	rows := make([][]string, 0, len(d.Relinked)+len(d.Skipped))
	action := "relinked"
	if d.DryRun {
		action = "would relink"
	}
	for _, u := range d.Relinked {
		rows = append(rows, []string{u.UserID, u.Email, u.NameID, action})
	}
	for _, u := range d.Skipped {
		rows = append(rows, []string{u.UserID, u.Email, u.NameID, "skipped: " + u.Reason})
	}
	if len(rows) > 0 {
		table([]string{"User ID", "Email", "NameID", "Result"}, rows)
	}
	if d.DryRun {
		note(fmt.Sprintf("Dry run: %d user(s) would be relinked to IdP #%d, %d skipped. Re-run without --dry-run (with --yes) to apply.", len(d.Relinked), d.IdpID, len(d.Skipped)))
		return nil
	}
	success(fmt.Sprintf("Relinked %d user(s) to IdP #%d (%d skipped). They were signed out everywhere and notified.", len(d.Relinked), d.IdpID, len(d.Skipped)))
	return nil
}

func orName(names map[int]string, id int) string {
	if n, ok := names[id]; ok {
		return n
	}
	return fmt.Sprintf("#%d", id)
}

func init() {
	usersIdentitiesLinkCmd.Flags().Int("saml-idp", 0, "SAML identity provider config id")
	usersIdentitiesLinkCmd.Flags().String("name-id", "", "NameID the SAML IdP asserts for this user")
	usersIdentitiesLinkCmd.Flags().Int("ldap-config", 0, "LDAP directory config id")
	usersIdentitiesLinkCmd.Flags().String("dn", "", "LDAP distinguished name (omit to look it up by email/username)")
	usersIdentitiesLinkCmd.Flags().Bool("ldap-only", false, "Also disable the user's local password (LDAP sign-in only)")
	usersIdentitiesLinkCmd.Flags().String("mfa-challenge", "", "Id of an approved step_up MFA challenge (needed with 'lumo login' tokens)")

	usersIdentitiesUnlinkCmd.Flags().String("type", "", "Identity to unlink: saml, ldap or social")
	usersIdentitiesUnlinkCmd.Flags().BoolP("yes", "y", false, "Unlink without asking for confirmation")
	usersIdentitiesUnlinkCmd.Flags().String("mfa-challenge", "", "Id of an approved step_up MFA challenge (needed with 'lumo login' tokens)")

	identitiesLegacySamlCmd.Flags().Int("idp", 0, "SAML identity provider config id (filters the report; required with --relink)")
	identitiesLegacySamlCmd.Flags().Bool("relink", false, "Relink legacy users to --idp")
	identitiesLegacySamlCmd.Flags().Bool("dry-run", false, "With --relink: show what would change without changing anything")
	identitiesLegacySamlCmd.Flags().BoolP("yes", "y", false, "With --relink: apply without asking for confirmation")
	identitiesLegacySamlCmd.Flags().StringSlice("user", nil, "With --relink: only these user ids (repeatable); default is every user the IdP's email domains claim")
	identitiesLegacySamlCmd.Flags().String("mfa-challenge", "", "Id of an approved step_up MFA challenge (needed with 'lumo login' tokens)")

	usersIdentitiesCmd.AddCommand(usersIdentitiesListCmd, usersIdentitiesLinkCmd, usersIdentitiesUnlinkCmd)
	usersCmd.AddCommand(usersIdentitiesCmd)
	identitiesCmd.AddCommand(identitiesLegacySamlCmd)
	rootCmd.AddCommand(identitiesCmd)
}
