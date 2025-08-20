package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/autotime/autotime/internal/config"
	"github.com/autotime/autotime/internal/connectors"
	"github.com/autotime/autotime/internal/display"
	"github.com/autotime/autotime/internal/editor"
	"github.com/autotime/autotime/internal/timeline"
)

var (
	configPath   string
	date         string
	format       string
	showDetail   bool
	maxItems     int
	filterType   string
	filterSource string
	groupByHour  bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "autotime",
	Short: "AutoTime - Daily Activity Timeline Builder",
	Long: `AutoTime is a CLI tool that connects to various services to automatically
gather information about your daily activities and presents them in a chronological timeline.

Features:
• Connect to GitHub, Calendar, File System, and other services
• View activities in a formatted timeline
• Configure connectors through YAML configuration
• Export activity data in various formats

Use the CLI commands to interact with the system and view your daily activities.`,
	Example: `  # Show today's timeline
  autotime timeline

  # Show timeline for a specific date
  autotime timeline --date 2023-12-25

  # Show detailed timeline with all information
  autotime timeline --details

  # List all connectors and their status
  autotime connectors list

  # Edit configuration
  autotime config edit`,
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

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default is $HOME/.config/autotime/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&date, "date", "", "date for operations (default is today, format: YYYY-MM-DD)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "table", "output format (table, json, csv)")

	// Add subcommands
	rootCmd.AddCommand(timelineCmd)
	rootCmd.AddCommand(connectorsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// timelineCmd shows the timeline for a specific date
var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Show activity timeline for a date",
	Long: `Display the activity timeline for a specific date.
Activities are fetched from all enabled connectors and displayed in chronological order.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Parse date
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		parsedDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid date format. Use YYYY-MM-DD: %v\n", err)
			os.Exit(1)
		}
		targetDate := parsedDate

		// Initialize configuration and connectors
		configManager, registry := initializeSystem()

		// Create timeline
		tl := timeline.NewTimeline(targetDate.Truncate(24 * time.Hour))

		// Fetch activities from enabled connectors
		ctx := context.Background()
		enabledConnectors := getEnabledConnectors(configManager, registry)

		if len(enabledConnectors) == 0 {
			fmt.Println("No connectors are enabled. Use 'autotime connectors list' to see available connectors.")
			fmt.Println("Enable a connector with: autotime connectors enable <connector-name>")
			return
		}

		fmt.Printf("Fetching activities for %s...\n", targetDate.Format("January 2, 2006"))

		for name, connector := range enabledConnectors {
			fmt.Printf("• Fetching from %s...\n", name)
			activities, err := connector.GetActivities(ctx, targetDate)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Error fetching from %s: %v\n", name, err)
				continue
			}
			tl.AddActivities(activities)
			fmt.Printf("  Found %d activities\n", len(activities))
		}

		fmt.Println()

		// Display timeline
		opts := display.TimelineOptions{
			ShowDetails:    showDetail,
			ShowTimestamps: true,
			GroupByHour:    groupByHour,
			FilterType:     filterType,
			FilterSource:   filterSource,
			MaxItems:       maxItems,
			Format:         format,
		}

		if err := display.DisplayTimeline(tl, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
			os.Exit(1)
		}
	},
}

// connectorsCmd manages connectors
var connectorsCmd = &cobra.Command{
	Use:   "connectors",
	Short: "Manage service connectors",
	Long: `Manage and configure connectors for various services like GitHub,
Calendar, File System, etc. Use subcommands to list, enable, disable, and test connectors.`,
}

// configCmd manages application configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage application configuration",
	Long: `View and modify application configuration settings. This includes
connector settings, UI preferences, storage settings, and global application behavior.`,
}

// versionCmd shows version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version information for AutoTime.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("AutoTime v0.1.0")
		fmt.Println("Daily Activity Timeline Builder")
		fmt.Println("Built with ❤️  using Go and Cobra")
	},
}

func init() {
	// Timeline flags
	timelineCmd.Flags().BoolVar(&showDetail, "details", false, "show detailed information for each activity")
	timelineCmd.Flags().IntVar(&maxItems, "max", 50, "maximum number of activities to show")
	timelineCmd.Flags().StringVar(&filterType, "type", "", "filter by activity type")
	timelineCmd.Flags().StringVar(&filterSource, "source", "", "filter by activity source")
	timelineCmd.Flags().BoolVar(&groupByHour, "group", true, "group activities by hour")

	// Connectors subcommands
	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all available connectors",
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry := initializeSystem()

			fmt.Println("Available Connectors:")
			fmt.Println("=====================")

			for name, connector := range registry.List() {
				status := "❌ Disabled"
				if configManager.IsConnectorEnabled(name) {
					status = "✅ Enabled"
				}
				fmt.Printf("%-15s %s - %s\n", name, status, connector.Description())
			}
			fmt.Println()
			fmt.Println("💡 Enable a connector: autotime connectors enable <name>")
			fmt.Println("⚙️  Edit configuration: autotime config edit")
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "enable [connector]",
		Short: "Enable a connector",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry := initializeSystem()
			connectorName := args[0]

			if _, exists := registry.Get(connectorName); !exists {
				fmt.Fprintf(os.Stderr, "Connector '%s' not found\n", connectorName)
				fmt.Println("\nAvailable connectors:")
				for name := range registry.List() {
					fmt.Printf("  • %s\n", name)
				}
				os.Exit(1)
			}

			configManager.EnableConnector(connectorName)
			if err := configManager.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✅ Enabled connector: %s\n", connectorName)
			fmt.Println("💡 Configure it by editing the config file: autotime config edit")
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "disable [connector]",
		Short: "Disable a connector",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry := initializeSystem()
			connectorName := args[0]

			if _, exists := registry.Get(connectorName); !exists {
				fmt.Fprintf(os.Stderr, "Connector '%s' not found\n", connectorName)
				os.Exit(1)
			}

			configManager.DisableConnector(connectorName)
			if err := configManager.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("❌ Disabled connector: %s\n", connectorName)
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "info [connector]",
		Short: "Show connector information and configuration requirements",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			_, registry := initializeSystem()
			connectorName := args[0]

			connector, exists := registry.Get(connectorName)
			if !exists {
				fmt.Fprintf(os.Stderr, "Connector '%s' not found\n", connectorName)
				os.Exit(1)
			}

			fmt.Printf("Connector: %s\n", connector.Name())
			fmt.Printf("Description: %s\n", connector.Description())
			fmt.Println(fmt.Sprintf("%s", "="))
			fmt.Println()

			requiredConfig := connector.GetRequiredConfig()
			if len(requiredConfig) > 0 {
				fmt.Println("Required Configuration:")
				for _, field := range requiredConfig {
					required := ""
					if field.Required {
						required = " (required)"
					}
					fmt.Printf("  %-20s %s%s\n", field.Key+":", field.Description, required)
					if field.Default != "" {
						fmt.Printf("  %-20s Default: %s\n", "", field.Default)
					}
				}
			}

			fmt.Println()
			fmt.Println("💡 Edit configuration: autotime config edit")
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "test [connector]",
		Short: "Test connector connection",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry := initializeSystem()
			connectorName := args[0]

			connector, exists := registry.Get(connectorName)
			if !exists {
				fmt.Fprintf(os.Stderr, "Connector '%s' not found\n", connectorName)
				os.Exit(1)
			}

			// Configure the connector
			connectorConfig, hasConfig := configManager.GetConnectorConfig(connectorName)
			if !hasConfig || !connectorConfig.Enabled {
				fmt.Printf("Connector '%s' is not enabled or configured\n", connectorName)
				fmt.Println("Enable it with: autotime connectors enable " + connectorName)
				return
			}

			// Inject app log level into connector config
			configWithLogLevel := make(map[string]interface{})
			for k, v := range connectorConfig.Config {
				configWithLogLevel[k] = v
			}
			configWithLogLevel["log_level"] = configManager.GetConfig().App.LogLevel

			if err := connector.Configure(configWithLogLevel); err != nil {
				fmt.Fprintf(os.Stderr, "Error configuring connector: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Testing connection to %s...\n", connectorName)

			ctx := context.Background()
			if err := connector.TestConnection(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Connection test failed: %v\n", err)
				fmt.Println("💡 Check your configuration: autotime config edit")
				os.Exit(1)
			}

			fmt.Printf("✅ Connection test successful for %s\n", connectorName)
		},
	})

	// Config subcommands
	configCmd.AddCommand(&cobra.Command{
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
			for name := range registry.List() {
				enabled := configManager.IsConnectorEnabled(name)
				status := "❌ Disabled"
				if enabled {
					status = "✅ Enabled"
				}
				fmt.Printf("  %-15s %s\n", name, status)
			}
		},
	})

	configCmd.AddCommand(&cobra.Command{
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
	})

	configCmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("This will reset your configuration to defaults. Continue? (y/N): ")
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
			fmt.Println("✅ Configuration reset to defaults.")
			fmt.Println("💡 Edit it with: autotime config edit")
		},
	})

	configCmd.AddCommand(&cobra.Command{
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
	})
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

	// Initialize connector registry
	registry := connectors.NewConnectorRegistry()

	// Register available connectors
	registry.Register(connectors.NewGitHubConnector())
	registry.Register(connectors.NewCalendarConnector())
	registry.Register(connectors.NewGitLabConnector())

	return configManager, registry
}

// getEnabledConnectors returns configured and enabled connectors
func getEnabledConnectors(configManager *config.Manager, registry *connectors.ConnectorRegistry) map[string]connectors.Connector {
	enabled := make(map[string]connectors.Connector)

	for name, connector := range registry.List() {
		if configManager.IsConnectorEnabled(name) {
			connectorConfig, exists := configManager.GetConnectorConfig(name)
			if exists {
				// Inject app log level into connector config
				configWithLogLevel := make(map[string]interface{})
				for k, v := range connectorConfig.Config {
					configWithLogLevel[k] = v
				}
				configWithLogLevel["log_level"] = configManager.GetConfig().App.LogLevel

				if err := connector.Configure(configWithLogLevel); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Error configuring %s connector: %v\n", name, err)
					continue
				}
				enabled[name] = connector
			}
		}
	}

	return enabled
}
