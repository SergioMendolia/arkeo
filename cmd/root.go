package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/config"
	"github.com/arkeo/arkeo/internal/connectors"
)

var (
	configPath string
	webAddr    string
)

var version = "dev" // Will be set by SetVersion function

// SetVersion sets the application version
func SetVersion(v string) {
	version = v
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "arkeo",
	Short: "arkeo - Daily Activity Timeline Builder",
	Long: `arkeo is a CLI tool that connects to various services to automatically
gather information about your daily activities and presents them in a chronological timeline.

Features:
• Connect to GitHub, GitLab, Calendar, YouTrack, Browser History, and more
• View activities in a formatted timeline or interactive web UI
• Cache past activities for instant re-display
• Configure connectors through YAML configuration

When run without a subcommand, launches the web UI.`,
	Example: `  # Launch the web UI (default)
  arkeo

  # Show yesterday's timeline in the terminal
  arkeo timeline

  # Show timeline for a specific date
  arkeo timeline 2024-01-15

  # Fetch the last 6 months of history
  arkeo timeline --range 180

  # Output in JSON format
  arkeo timeline --format json

  # List all connectors and their status
  arkeo connectors list

  # Manage browser domain exclusions interactively
  arkeo browser domains
`,
	Run: func(cmd *cobra.Command, args []string) {
		runWebCommand(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Disable the default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Web UI flag (also available on the root command since web is the default)
	rootCmd.PersistentFlags().StringVar(&webAddr, "addr", "localhost:7878", "Address for the web UI")

	// Add subcommands
	rootCmd.AddCommand(timelineCmd)
	rootCmd.AddCommand(connectorsCmd)
	rootCmd.AddCommand(browserCmd)
	rootCmd.AddCommand(webCmd)
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	// This will be called before other commands run
}

// initializeSystem initializes the configuration manager and connector registry
func initializeSystem() (*config.Manager, *connectors.ConnectorRegistry) {
	// Initialize configuration
	configManager := config.NewManager()
	if err := configManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize connector registry with direct registration
	registry := connectors.NewConnectorRegistry()

	// Register available connectors directly
	availableConnectors := []connectors.Connector{
		connectors.NewGitHubConnector(),
		connectors.NewCalendarConnector(),
		connectors.NewGitLabConnector(),
		connectors.NewYouTrackConnector(),
		connectors.NewMacOSSystemConnector(),
		connectors.NewWebhooksConnector(),
		connectors.NewBrowserHistoryConnector(),
	}

	for _, connector := range availableConnectors {
		registry.Register(connector)

		// Apply basic configuration even for disabled connectors
		baseConfig := map[string]interface{}{
			connectors.CommonConfigKeys.LogLevel: configManager.GetConfig().App.LogLevel,
		}

		// Get connector config
		if connectorConfig, exists := configManager.GetConnectorConfig(connector.Name()); exists {
			// Add connector-specific config
			for k, v := range connectorConfig.Config {
				baseConfig[k] = v
			}
		}

		// Only configure, but don't enable
		_ = connector.Configure(baseConfig)
	}

	return configManager, registry
}

// getEnabledConnectors returns configured and enabled connectors
func getEnabledConnectors(configManager *config.Manager, registry *connectors.ConnectorRegistry) map[string]connectors.Connector {
	enabled := make(map[string]connectors.Connector)
	appConfig := configManager.GetConfig()

	for name, connector := range registry.List() {
		if configManager.IsConnectorEnabled(name) {
			// Prepare configuration with app-level defaults
			configWithAppSettings := make(map[string]interface{})

			// Get connector config
			connectorConfig, exists := configManager.GetConnectorConfig(name)
			if exists {
				// First apply connector-specific config
				for k, v := range connectorConfig.Config {
					configWithAppSettings[k] = v
				}
			}

			// Add app-level settings as defaults
			configWithAppSettings[connectors.CommonConfigKeys.LogLevel] = appConfig.App.LogLevel
			configWithAppSettings[connectors.CommonConfigKeys.DateFormat] = appConfig.App.DateFormat
			configWithAppSettings[connectors.CommonConfigKeys.Timeout] = 30 // Always use 30 seconds

			// Add debug mode if environment variable is set
			if os.Getenv("ARKEO_DEBUG") != "" {
				configWithAppSettings[connectors.CommonConfigKeys.DebugMode] = true
			}

			if err := connector.Configure(configWithAppSettings); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Error configuring %s connector: %v\n", name, err)
				continue
			}

			connector.SetEnabled(true)
			enabled[name] = connector
		}
	}

	return enabled
}
