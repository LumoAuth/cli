package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lumoauth/cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  "View, set, and initialize CLI configuration for connecting to your LumoAuth organization.",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize CLI configuration interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)
		p := getPrinter()

		fmt.Println("LumoAuth CLI Configuration")
		fmt.Println("──────────────────────────")
		fmt.Println()

		// Load existing config for defaults
		existing, _ := getConfig()

		// Organization ID
		defaultOrgID := ""
		if existing != nil {
			defaultOrgID = existing.OrgID
		}
		fmt.Printf("Organization ID")
		if defaultOrgID != "" {
			fmt.Printf(" [%s]", defaultOrgID)
		}
		fmt.Print(": ")
		orgID, _ := reader.ReadString('\n')
		orgID = strings.TrimSpace(orgID)
		if orgID == "" {
			orgID = defaultOrgID
		}

		// API key
		fmt.Print("API key (lmk_...): ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)

		// Region — same enum as the mobile app and `lumo login`.
		defaultIdx := 1
		if existing != nil {
			switch config.RegionForURL(existing.BaseURL) {
			case config.RegionEU:
				defaultIdx = 2
			case config.RegionCustom:
				if existing.BaseURL != "" {
					defaultIdx = 3
				}
			}
		}
		fmt.Println("Region:")
		for i, r := range config.Regions {
			fmt.Printf("  %d) %-20s  %s\n", i+1, r.Label, r.Description)
		}
		fmt.Printf("Select region [%d]: ", defaultIdx)
		regionChoice, _ := reader.ReadString('\n')
		regionChoice = strings.TrimSpace(regionChoice)
		if regionChoice == "" {
			regionChoice = fmt.Sprintf("%d", defaultIdx)
		}
		var baseURL string
		switch regionChoice {
		case "1", "us", "US":
			baseURL = config.RegionUS.ServerURL
		case "2", "eu", "EU":
			baseURL = config.RegionEU.ServerURL
		case "3", "custom", "other":
			defaultURL := ""
			if existing != nil && config.RegionForURL(existing.BaseURL) == config.RegionCustom {
				defaultURL = existing.BaseURL
			}
			prompt := "Server URL (e.g. https://localhost:8000)"
			if defaultURL != "" {
				prompt += fmt.Sprintf(" [%s]", defaultURL)
			}
			fmt.Print(prompt + ": ")
			customURL, _ := reader.ReadString('\n')
			customURL = strings.TrimSpace(customURL)
			if customURL == "" {
				customURL = defaultURL
			}
			if customURL == "" {
				return fmt.Errorf("server URL is required for the Custom option")
			}
			baseURL = strings.TrimSuffix(customURL, "/")
		default:
			return fmt.Errorf("invalid selection %q — pick 1, 2, or 3", regionChoice)
		}

		cfg := &config.Config{
			OrgID:   orgID,
			APIKey:  apiKey,
			BaseURL: baseURL,
			Format:  "table",
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println()
		p.PrintSuccess(fmt.Sprintf("Configuration saved to %s", config.ConfigPath()))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getConfig()
		if err != nil {
			return err
		}

		p := getPrinter()

		// Mask API key for display
		maskedKey := cfg.APIKey
		if len(maskedKey) > 12 {
			maskedKey = maskedKey[:12] + "••••••••"
		}

		if p.IsJSON() {
			p.PrintResult(map[string]interface{}{
				"org_id":      cfg.OrgID,
				"api_key":     maskedKey,
				"base_url":    cfg.BaseURL,
				"format":      cfg.Format,
				"insecure":    cfg.Insecure,
				"config_path": config.ConfigPath(),
			})
			return nil
		}

		fmt.Println("Current Configuration")
		fmt.Println("─────────────────────")
		fmt.Printf("  Org ID:      %s\n", cfg.OrgID)
		fmt.Printf("  API Key:     %s\n", maskedKey)
		fmt.Printf("  Base URL:    %s\n", cfg.BaseURL)
		fmt.Printf("  Format:      %s\n", cfg.Format)
		fmt.Printf("  Insecure:    %v\n", cfg.Insecure)
		fmt.Printf("  Config Path: %s\n", config.ConfigPath())
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value in the config file.

Available keys: org_id, api_key, base_url, format, insecure`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		cfg, err := getConfig()
		if err != nil {
			// Start with empty config if none exists
			cfg = &config.Config{}
		}

		switch key {
		case "org_id":
			cfg.OrgID = value
		case "api_key":
			cfg.APIKey = value
		case "base_url":
			cfg.BaseURL = value
		case "format":
			cfg.Format = value
		case "insecure":
			cfg.Insecure = value == "true" || value == "1"
		default:
			return fmt.Errorf("unknown config key: %s (available: org_id, api_key, base_url, format, insecure)", key)
		}

		if err := cfg.Save(); err != nil {
			return err
		}

		p := getPrinter()
		p.PrintSuccess(fmt.Sprintf("Set %s = %s", key, value))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
