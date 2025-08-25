package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/config"
	"github.com/arkeo/arkeo/internal/editor"
)

// configCmd manages application configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage application configuration",
	Long: `View and modify application configuration settings. This includes
connector settings, UI preferences, storage settings, and global application behavior.

Use subcommands to manage both configuration and preferences via the YAML configuration file.`,
}

func init() {
	// Add config subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configGenerateExampleCmd)
	configCmd.AddCommand(preferencesCmd)
	configCmd.AddCommand(preferencesResetCmd)
	configCmd.AddCommand(preferencesExportCmd)
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration path and summary",
	Run: func(cmd *cobra.Command, args []string) {
		configManager, registry := initializeSystem()

		fmt.Printf("Configuration file: %s\n", configManager.GetConfigPath())

		cfg := configManager.GetConfig()
		fmt.Printf("App settings:\n")
		fmt.Printf("  Date format: %s\n", cfg.App.DateFormat)
		fmt.Printf("  Log level: %s\n", cfg.App.LogLevel)

		fmt.Printf("\nConnector status:\n")
		for name, connector := range registry.List() {
			status := "❌ Disabled"
			configStatus := ""

			if configManager.IsConnectorEnabled(name) {
				status = "✅ Enabled"
			}

			// Check if all required fields are configured
			requiredFields := connector.GetRequiredConfig()
			missingFields := 0

			for _, field := range requiredFields {
				if field.Required {
					val, exists := configManager.GetConnectorConfigValue(name, field.Key)
					if !exists || val == nil || (field.Type == "string" && val.(string) == "") {
						missingFields++
					}
				}
			}

			if missingFields > 0 {
				configStatus = fmt.Sprintf(" (Missing %d required fields)", missingFields)
			}

			fmt.Printf("  %s: %s%s\n", name, status, configStatus)
		}
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file with default editor",
	Long: `Open the configuration file in your default editor.
The editor is determined by the VISUAL or EDITOR environment variables,
or falls back to a platform-specific default (nano on Unix, notepad on Windows).`,
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewManager()
		if err := configManager.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		configPath := configManager.GetConfigPath()
		fmt.Printf("Opening configuration file: %s\n", configPath)

		if err := editor.OpenFile(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
			fmt.Printf("\nYou can manually edit the file at: %s\n", configPath)
			os.Exit(1)
		}

		fmt.Println("Configuration file closed.")
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration by copying example config file",
	Long: `Reset your configuration by copying the example config file (config.example.yaml).
This will overwrite your current configuration with the example defaults.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("This will reset your configuration by copying the example config. All your settings will be lost. Continue? (y/N): ")
		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println("Operation cancelled.")
			return
		}

		configManager := config.NewManager()
		if err := configManager.Reset(); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting configuration: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Configuration has been reset using example config file.")
		fmt.Printf("📄 Your config file is located at: %s\n", configManager.GetConfigPath())
		fmt.Println("💡 Edit it with: arkeo config edit")
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewManager()
		if err := configManager.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		if err := configManager.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Configuration is valid")
	},
}

var configGenerateExampleCmd = &cobra.Command{
	Use:   "generate-example",
	Short: "Generate or update the config.example.yaml file",
	Long: `Generate a config.example.yaml file with all available configuration options
and detailed comments explaining how to configure each setting. This ensures the
example file is always up-to-date with the latest configuration options.`,
	Run: func(cmd *cobra.Command, args []string) {
		configManager := config.NewManager()
		
		// Generate to config.example.yaml in current directory
		outputPath := "config.example.yaml"
		
		if err := configManager.ExportExampleConfig(outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error generating example config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Generated example configuration file: %s\n", outputPath)
		fmt.Println("📄 This file contains all available configuration options with detailed comments")
		fmt.Println("💡 Copy and customize this file to ~/.config/arkeo/config.yaml to get started")
	},
}

var preferencesCmd = &cobra.Command{
	Use:   "preferences",
	Short: "Show current preferences",
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		configManager, _ := initializeSystem()
		prefs := configManager.GetPreferences()

		fmt.Println("Current Preferences:")
		fmt.Println(strings.Repeat("=", 40))

		fmt.Printf("Display:\n")
		fmt.Printf("  Use Colors:      %v\n", prefs.UseColors)
		fmt.Printf("  Show Details:    %v\n", prefs.ShowDetails)
		fmt.Printf("  Show Progress:   %v\n", prefs.ShowProgress)
		fmt.Printf("  Default Format:  %s\n", prefs.DefaultFormat)

		fmt.Printf("\nTimeline:\n")
		fmt.Printf("  Group by Hour:   %v\n", prefs.GroupByHour)
		fmt.Printf("  Max Items:       %d\n", prefs.MaxItems)
	},
}

var preferencesResetCmd = &cobra.Command{
	Use:   "preferences-reset",
	Short: "Reset preferences to defaults",
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		configManager, _ := initializeSystem()

		if err := configManager.Reset(); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting preferences: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Preferences reset to defaults")
	},
}

var preferencesExportCmd = &cobra.Command{
	Use:   "preferences-export",
	Short: "Export preferences to YAML",
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		configManager, _ := initializeSystem()
		prefs := configManager.GetPreferences()

		// Export preferences as YAML
		fmt.Println("preferences:")
		fmt.Printf("  use_colors: %v\n", prefs.UseColors)
		fmt.Printf("  show_details: %v\n", prefs.ShowDetails)
		fmt.Printf("  show_progress: %v\n", prefs.ShowProgress)
		fmt.Printf("  show_gaps: %v\n", prefs.ShowGaps)
		fmt.Printf("  default_format: %s\n", prefs.DefaultFormat)
		fmt.Printf("  group_by_hour: %v\n", prefs.GroupByHour)
		fmt.Printf("  max_items: %d\n", prefs.MaxItems)
		fmt.Printf("  parallel_fetch: %v\n", prefs.ParallelFetch)
		fmt.Printf("  fetch_timeout: %d\n", prefs.FetchTimeout)
	},
}