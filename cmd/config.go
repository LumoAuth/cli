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
	Long:  "View, set, and initialize CLI configuration for connecting to your LumoAuth tenant.",
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

		// Tenant slug
		defaultTenant := ""
		if existing != nil {
			defaultTenant = existing.Tenant
		}
		fmt.Printf("Tenant slug")
		if defaultTenant != "" {
			fmt.Printf(" [%s]", defaultTenant)
		}
		fmt.Print(": ")
		tenant, _ := reader.ReadString('\n')
		tenant = strings.TrimSpace(tenant)
		if tenant == "" {
			tenant = defaultTenant
		}

		// API key
		fmt.Print("API key (lmk_...): ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)

		// Region
		fmt.Println("Region:")
		fmt.Println("  1) US  (app.lumoauth.dev)")
		fmt.Println("  2) EU  (eu.app.lumoauth.dev)")
		defaultRegion := "1"
		if existing != nil && strings.Contains(existing.BaseURL, "eu.") {
			defaultRegion = "2"
		}
		fmt.Printf("Select region [%s]: ", defaultRegion)
		region, _ := reader.ReadString('\n')
		region = strings.TrimSpace(region)
		if region == "" {
			region = defaultRegion
		}
		var baseURL string
		switch region {
		case "2":
			baseURL = "https://eu.app.lumoauth.dev"
		default:
			baseURL = "https://app.lumoauth.dev"
		}

		cfg := &config.Config{
			Tenant:  tenant,
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
				"tenant":    cfg.Tenant,
				"api_key":   maskedKey,
				"base_url":  cfg.BaseURL,
				"format":    cfg.Format,
				"insecure":  cfg.Insecure,
				"config_path": config.ConfigPath(),
			})
			return nil
		}

		fmt.Println("Current Configuration")
		fmt.Println("─────────────────────")
		fmt.Printf("  Tenant:      %s\n", cfg.Tenant)
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

Available keys: tenant, api_key, base_url, format, insecure`,
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
		case "tenant":
			cfg.Tenant = value
		case "api_key":
			cfg.APIKey = value
		case "base_url":
			cfg.BaseURL = value
		case "format":
			cfg.Format = value
		case "insecure":
			cfg.Insecure = value == "true" || value == "1"
		default:
			return fmt.Errorf("unknown config key: %s (available: tenant, api_key, base_url, format, insecure)", key)
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
