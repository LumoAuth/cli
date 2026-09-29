package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mfaRequirements = []string{"off", "optional", "required", "risk_based"}
	mfaFactorTypes  = []string{"passkey", "push", "totp", "sms_otp", "email_otp"}
	mfaTierLabels   = map[string]string{"1": "Strongest", "2": "Strong", "3": "Standard", "4": "Basic", "5": "Recovery"}
)

var mfaCmd = &cobra.Command{
	Use:   "mfa",
	Short: "Organization MFA policy and enrollment coverage",
	Long: `Organization-wide multi-factor authentication: the MFA policy and the
enrollment coverage report. Per-user factors live under 'lumo users':

  lumo users authenticators list <user>          a user's factors
  lumo users authenticators remove <user> <id>   remove a lost device
  lumo users tap <user> --reason "..."           temporary access code

Policy reads need admin:settings:read, writes admin:settings:write; the
coverage report needs admin:users:read.`,
}

var mfaPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "View and update the MFA policy",
}

var mfaPolicyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the MFA policy",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get("/policies/mfa", nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data      json.RawMessage `json:"data"`
			Effective []string        `json:"effective_allowed_factors"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}
		p.PrintResult(result.Data)
		if result.Effective != nil {
			fmt.Printf("\nEffective factors: %s\n", joinOrDash(result.Effective))
			fmt.Println("(allowed factors this organization can actually deliver, e.g. without an SMS provider sms_otp is dropped)")
		}
		return nil
	},
}

var mfaPolicySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update the MFA policy (only the flags you pass change)",
	Long: `Update the MFA policy. Only the flags you pass are sent; everything else
keeps its current value. Changes apply at each user's next sign-in.

Requirement:
  off          MFA is never asked for
  optional     users may enroll; never forced
  required     every user must enroll (after --grace-days)
  risk_based   enrolled users are challenged when a sign-in looks risky

Tiers: 1 strongest (passkey) … 4 basic (SMS/email). Boolean flags take
--flag or --flag=false.

Examples:
  lumo mfa policy set --requirement required --grace-days 14
  lumo mfa policy set --allowed-factors passkey,push,totp --min-tier-step-up 3
  lumo mfa policy set --email-otp-counts=false --block-voip
  lumo mfa policy set --data '{"sms_country_denylist":["XX"]}'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := mfaPolicyBody(cmd)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("nothing to update: pass at least one flag (see 'lumo mfa policy set --help')")
		}

		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Put("/policies/mfa", body)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}
		p.PrintSuccess("MFA policy updated")
		p.PrintResult(json.RawMessage(resp))
		return nil
	},
}

// mfaPolicyBody builds the partial PUT body from the flags that were set.
func mfaPolicyBody(cmd *cobra.Command) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	f := cmd.Flags()

	if data, _ := f.GetString("data"); data != "" {
		if err := json.Unmarshal([]byte(data), &body); err != nil {
			return nil, fmt.Errorf("invalid JSON in --data: %w", err)
		}
	}
	if f.Changed("requirement") {
		v, _ := f.GetString("requirement")
		if !contains(mfaRequirements, v) {
			return nil, fmt.Errorf("--requirement must be one of: %s", strings.Join(mfaRequirements, ", "))
		}
		body["requirement"] = v
	}
	if f.Changed("allowed-factors") {
		v, _ := f.GetStringSlice("allowed-factors")
		factors := []string{}
		for _, x := range v {
			x = strings.TrimSpace(x)
			if x == "" {
				continue
			}
			if !contains(mfaFactorTypes, x) {
				return nil, fmt.Errorf("unknown factor %q in --allowed-factors (use: %s)", x, strings.Join(mfaFactorTypes, ", "))
			}
			factors = append(factors, x)
		}
		if len(factors) == 0 {
			return nil, fmt.Errorf("--allowed-factors needs at least one factor (use: %s)", strings.Join(mfaFactorTypes, ", "))
		}
		body["allowed_factors"] = factors
	}

	ints := []struct {
		flag, key string
		min, max  int
	}{
		{"grace-days", "grace_period_days", 0, 90},
		{"trusted-device-days", "trusted_device_days", 0, 180},
		{"min-tier-login", "min_tier_login", 1, 4},
		{"min-tier-step-up", "min_tier_step_up", 1, 4},
		{"required-factors", "required_factor_count", 1, 2},
	}
	for _, i := range ints {
		if !f.Changed(i.flag) {
			continue
		}
		v, _ := f.GetInt(i.flag)
		if v < i.min || v > i.max {
			return nil, fmt.Errorf("--%s must be between %d and %d", i.flag, i.min, i.max)
		}
		body[i.key] = v
	}

	bools := []struct{ flag, key string }{
		{"email-otp-counts", "email_otp_counts_as_mfa"},
		{"passwordless-passkey", "passwordless_passkey"},
		{"block-voip", "sms_block_voip"},
	}
	for _, b := range bools {
		if f.Changed(b.flag) {
			v, _ := f.GetBool(b.flag)
			body[b.key] = v
		}
	}
	return body, nil
}

var mfaCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Show MFA enrollment coverage across the organization",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		p := getPrinter()

		resp, err := c.Get("/reports/mfa-coverage", nil)
		if err != nil {
			return err
		}

		if !p.IsTable() {
			p.PrintResult(json.RawMessage(resp))
			return nil
		}

		var result struct {
			Data mfaCoverage `json:"data"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}
		printMfaCoverage(p, result.Data)
		return nil
	},
}

type mfaCoverage struct {
	TotalUsers    int            `json:"total_users"`
	EnrolledUsers int            `json:"enrolled_users"`
	EnrolledPct   float64        `json:"enrolled_pct"`
	ByType        map[string]int `json:"by_type"`
	ByTier        map[string]int `json:"by_tier"`
	Requirement   string         `json:"requirement"`
	InGrace       int            `json:"in_grace"`
	PastGrace     int            `json:"past_grace"`
	Deferred      int            `json:"deferred"`
	GeneratedAt   string         `json:"generated_at"`
}

type tablePrinter interface {
	PrintTable(headers []string, rows [][]string)
}

func printMfaCoverage(p tablePrinter, r mfaCoverage) {
	fmt.Println("MFA coverage")
	if r.Requirement != "" {
		fmt.Printf("  Requirement:  %s\n", r.Requirement)
	}
	fmt.Printf("  Enrolled:     %d of %d users (%.1f%%)\n", r.EnrolledUsers, r.TotalUsers, r.EnrolledPct)
	fmt.Printf("  Not enrolled: %d\n", r.TotalUsers-r.EnrolledUsers)
	fmt.Printf("  In grace:     %d\n", r.InGrace)
	fmt.Printf("  Past grace:   %d\n", r.PastGrace)
	fmt.Printf("  Deferred:     %d\n", r.Deferred)
	if r.GeneratedAt != "" {
		fmt.Printf("  Generated:    %s\n", formatTimestamp(r.GeneratedAt))
	}

	if len(r.ByType) > 0 {
		fmt.Println("\nBy factor type")
		p.PrintTable([]string{"Type", "Users"}, countRows(r.ByType, nil))
	}
	if len(r.ByTier) > 0 {
		fmt.Println("\nBy strongest factor tier")
		p.PrintTable([]string{"Tier", "Users"}, countRows(r.ByTier, mfaTierLabels))
	}
}

// countRows turns a {key: count} map into sorted table rows. Numeric keys sort
// numerically; labels, when given, are appended to the key.
func countRows(m map[string]int, labels map[string]string) [][]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, errA := strconv.Atoi(keys[i])
		b, errB := strconv.Atoi(keys[j])
		if errA == nil && errB == nil {
			return a < b
		}
		return keys[i] < keys[j]
	})
	rows := make([][]string, len(keys))
	for i, k := range keys {
		name := k
		if l, ok := labels[k]; ok {
			name = k + " " + l
		}
		rows[i] = []string{name, strconv.Itoa(m[k])}
	}
	return rows
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func joinOrDash(v []string) string {
	if len(v) == 0 {
		return "—"
	}
	return strings.Join(v, ", ")
}

func init() {
	// Defaults are zero values on purpose: only flags that were passed are
	// sent, so a default here would only mislead in --help.
	f := mfaPolicySetCmd.Flags()
	f.String("requirement", "", "off, optional, required or risk_based")
	f.StringSlice("allowed-factors", nil, "Factors users may enroll (comma-separated): passkey,push,totp,sms_otp,email_otp")
	f.Int("grace-days", 0, "Days a user may sign in without MFA once it is required (0–90)")
	f.Int("trusted-device-days", 0, "Days a device stays trusted after MFA (0 disables, max 180)")
	f.Int("min-tier-login", 0, "Weakest factor tier accepted at sign-in (1 strongest … 4 basic)")
	f.Int("min-tier-step-up", 0, "Weakest factor tier accepted for step-up (1 strongest … 4 basic)")
	f.Int("required-factors", 0, "Number of factors each user must enroll (1–2)")
	f.Bool("email-otp-counts", false, "Count email codes as a second factor")
	f.Bool("passwordless-passkey", false, "Let passkeys sign in without a password")
	f.Bool("block-voip", false, "Refuse VoIP numbers for SMS codes")
	f.String("data", "", "Raw JSON object with other policy fields (flags override it)")

	mfaPolicyCmd.AddCommand(mfaPolicyGetCmd, mfaPolicySetCmd)
	mfaCmd.AddCommand(mfaPolicyCmd, mfaCoverageCmd)
	rootCmd.AddCommand(mfaCmd)
}
