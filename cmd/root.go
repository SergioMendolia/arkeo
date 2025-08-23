package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/config"
	"github.com/arkeo/arkeo/internal/connectors"
	"github.com/arkeo/arkeo/internal/display"
	"github.com/arkeo/arkeo/internal/editor"
	"github.com/arkeo/arkeo/internal/timeline"
	"github.com/arkeo/arkeo/internal/ui"
	"github.com/arkeo/arkeo/internal/utils"
)

var (
	configPath  string
	date        string
	format      string
	showDetail  bool
	maxItems    int
	groupByHour bool

	// Enhanced timeline flags
	useColors    bool
	showTimeline bool
	showProgress bool
	showGaps     bool
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
• Connect to GitHub, Calendar, File System, and other services
• View activities in a formatted timeline
• Configure connectors through YAML configuration
• Export activity data in various formats

Use the CLI commands to interact with the system and view your daily activities.`,
	Example: `  # Show today's timeline
  arkeo timeline

  # Show timeline for a specific date
  arkeo timeline --date 2023-12-25

  # Show detailed timeline with all information
  arkeo timeline --details

  # List all connectors and their status
  arkeo connectors list

  # Edit configuration
  arkeo config edit`,
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

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default is $HOME/.config/arkeo/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&date, "date", "", "date for operations (default is today, format: YYYY-MM-DD)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "visual", "output format (table, json, csv, visual)")

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

		// Initialize configuration, connectors and preferences
		configManager, registry, _ := initializeSystem()

		// Create timeline
		tl := timeline.NewTimeline(targetDate.Truncate(24 * time.Hour))

		// Fetch activities from enabled connectors
		ctx := context.Background()
		enabledConnectors := getEnabledConnectors(configManager, registry)

		if len(enabledConnectors) == 0 {
			fmt.Println("No connectors are enabled. Use 'arkeo connectors list' to see available connectors.")
			fmt.Println("Enable a connector with: arkeo connectors enable <connector-name>")
			return
		}

		fmt.Printf("Fetching activities for %s...\n", targetDate.Format("January 2, 2006"))

		// Initialize progress tracker if enabled
		var progress *ui.ConnectorProgress
		if showProgress {
			progress = ui.NewConnectorProgress(useColors)
		}

		// Convert connectors to utils.Connector interface and start progress tracking
		utilsConnectors := make(map[string]utils.Connector)
		for name, conn := range enabledConnectors {
			utilsConnectors[name] = conn
			if progress != nil {
				progress.StartConnector(name)
			}
		}

		// Fetch activities from all connectors with progress tracking
		var activities []timeline.Activity
		if progress != nil {
			// Fetch with progress updates (simplified for now - would need utils.FetchActivitiesWithProgress)
			activities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)
			for name := range enabledConnectors {
				// Simulate progress completion (in real implementation, this would be integrated into the fetch)
				connectorActivities := 0
				for _, activity := range activities {
					if activity.Source == name {
						connectorActivities++
					}
				}
				progress.FinishConnector(name, connectorActivities, nil)
			}
			progress.PrintSummary()
		} else {
			activities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)
		}

		tl.AddActivitiesUnsorted(activities)
		tl.EnsureSorted()

		fmt.Println()

		// Use enhanced display
		enhancedOpts := display.EnhancedTimelineOptions{
			TimelineOptions: display.TimelineOptions{
				ShowDetails:    showDetail,
				ShowTimestamps: true,
				GroupByHour:    groupByHour,
				MaxItems:       maxItems,
				Format:         format,
			},
			UseColors:    useColors,
			ShowTimeline: showTimeline,
			ShowProgress: showProgress,
			ShowGaps:     showGaps,
		}

		if err := display.DisplayEnhancedTimeline(tl, enhancedOpts); err != nil {
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
connector settings, UI preferences, storage settings, and global application behavior.

Use subcommands to manage both configuration and preferences via the YAML configuration file.`,
}

// Version command provides version information

// versionCmd shows version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version information for arkeo.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("arkeo %s\n", version)
		fmt.Println("Daily Activity Timeline Builder")
		fmt.Println("Built with ❤️  using Go and Cobra")
	},
}

func init() {
	// Add preferences subcommands to config command
	configCmd.AddCommand(&cobra.Command{
		Use:   "preferences",
		Short: "Show current preferences",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize configuration and preferences
			_, _, prefsManager := initializeSystem()
			if err := prefsManager.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "Error loading preferences: %v\n", err)
				os.Exit(1)
			}

			prefs := prefsManager.GetPreferences()
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
			fmt.Printf("  Show Timeline:   %v\n", prefs.ShowTimeline)

			// Recent dates and quick dates have been removed from preferences
		},
	})

	configCmd.AddCommand(&cobra.Command{
		Use:   "preferences-reset",
		Short: "Reset preferences to defaults",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize configuration and preferences
			_, _, prefsManager := initializeSystem()

			if err := prefsManager.Reset(); err != nil {
				fmt.Fprintf(os.Stderr, "Error resetting preferences: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("✅ Preferences reset to defaults")
		},
	})

	configCmd.AddCommand(&cobra.Command{
		Use:   "preferences-export",
		Short: "Export preferences to JSON",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize configuration and preferences
			_, _, prefsManager := initializeSystem()
			if err := prefsManager.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "Error loading preferences: %v\n", err)
				os.Exit(1)
			}

			jsonData, err := prefsManager.Export()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error exporting preferences: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(jsonData)
		},
	})

	// Add all subcommands to root
	// Timeline flags
	timelineCmd.Flags().BoolVar(&showDetail, "details", false, "show detailed information for each activity")
	timelineCmd.Flags().IntVar(&maxItems, "max", 500, "maximum number of activities to show")
	timelineCmd.Flags().BoolVar(&groupByHour, "group", false, "group activities by hour")

	// Enhanced timeline flags
	timelineCmd.Flags().BoolVar(&useColors, "colors", true, "use colors in output")
	timelineCmd.Flags().BoolVar(&showTimeline, "visual", true, "show visual timeline view")
	timelineCmd.Flags().BoolVar(&showProgress, "progress", true, "show progress indicators")
	timelineCmd.Flags().BoolVar(&showGaps, "gaps", true, "highlight time gaps")

	// Connectors subcommands
	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all available connectors",
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry, _ := initializeSystem()

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
			fmt.Println("💡 Enable a connector: arkeo connectors enable <name>")
			fmt.Println("⚙️  Edit configuration: arkeo config edit")
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "enable [connector]",
		Short: "Enable a connector",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry, _ := initializeSystem()
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
			fmt.Println("💡 Configure it by editing the config file: arkeo config edit")
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "disable [connector]",
		Short: "Disable a connector",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry, _ := initializeSystem()
			connectorName := args[0]

			connector, exists := registry.Get(connectorName)
			if !exists {
				fmt.Fprintf(os.Stderr, "Connector '%s' not found\n", connectorName)
				os.Exit(1)
			}

			// Get required config fields to check if all required fields are configured
			requiredFields := connector.GetRequiredConfig()

			// Check if all required fields are configured
			missingFields := []string{}
			for _, field := range requiredFields {
				if field.Required {
					val, exists := configManager.GetConnectorConfigValue(connectorName, field.Key)
					isEmptyString := false
					if str, ok := val.(string); ok && str == "" {
						isEmptyString = true
					}

					if !exists || val == nil || isEmptyString {
						missingFields = append(missingFields, field.Key)
					}
				}
			}

			if len(missingFields) > 0 {
				fmt.Fprintf(os.Stderr, "Cannot enable connector '%s' - missing required configuration:\n", connectorName)
				for _, field := range missingFields {
					fmt.Fprintf(os.Stderr, "  • %s\n", field)
				}
				fmt.Fprintf(os.Stderr, "\nUse 'arkeo connectors config %s' to configure these fields\n", connectorName)
				os.Exit(1)
			}

			configManager.EnableConnector(connectorName)
			if err := configManager.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Connector '%s' enabled\n", connectorName)
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "info [connector]",
		Short: "Show connector information and configuration requirements",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry, _ := initializeSystem()
			connectorName := args[0]

			connector, exists := registry.Get(connectorName)
			if !exists {
				fmt.Fprintf(os.Stderr, "Connector '%s' not found\n", connectorName)
				os.Exit(1)
			}

			// Get connector configuration status
			connectorConfig, configExists := configManager.GetConnectorConfig(connectorName)

			// Status indicators
			enabledStatus := "❌ Disabled"
			if configManager.IsConnectorEnabled(connectorName) {
				enabledStatus = "✅ Enabled"
			}

			// Header
			fmt.Printf("Connector: %s (%s)\n", connector.Name(), enabledStatus)
			fmt.Printf("Description: %s\n", connector.Description())
			fmt.Println(strings.Repeat("=", 50))
			fmt.Println()

			// Configuration fields
			requiredConfig := connector.GetRequiredConfig()
			if len(requiredConfig) > 0 {
				fmt.Println("Configuration Fields:")
				fmt.Println(strings.Repeat("-", 50))

				for _, field := range requiredConfig {
					// Determine if field is configured
					valueStr := "<not set>"
					configuredSymbol := " "

					if configExists {
						if val, exists := connectorConfig.Config[field.Key]; exists && val != nil {
							switch v := val.(type) {
							case string:
								if field.Type == "secret" && v != "" {
									valueStr = "********"
									configuredSymbol = "✓"
								} else if v != "" {
									valueStr = v
									configuredSymbol = "✓"
								}
							case bool:
								valueStr = fmt.Sprintf("%t", v)
								configuredSymbol = "✓"
							case int:
								valueStr = fmt.Sprintf("%d", v)
								configuredSymbol = "✓"
							case float64:
								valueStr = fmt.Sprintf("%.0f", v)
								configuredSymbol = "✓"
							default:
								valueStr = fmt.Sprintf("%v", val)
								configuredSymbol = "✓"
							}
						}
					}

					// Format field status
					requiredMark := " "
					if field.Required {
						requiredMark = "*"
					}

					fmt.Printf(" %s%s %-18s │ %-10s │ %s\n",
						configuredSymbol, requiredMark, field.Key, field.Type, valueStr)
					fmt.Printf("    └─ %s\n", field.Description)

					// Show default if available
					if field.Default != nil && valueStr == "<not set>" {
						fmt.Printf("       Default: %v\n", field.Default)
					}
					fmt.Println()
				}

				fmt.Println("* Required field")
			}

			fmt.Println()
			fmt.Printf("📝 Configure: arkeo connectors config %s\n", connectorName)
			if configManager.IsConnectorEnabled(connectorName) {
				fmt.Printf("🔌 Disable: arkeo connectors disable %s\n", connectorName)
			} else {
				fmt.Printf("🔌 Enable: arkeo connectors enable %s\n", connectorName)
			}
		},
	})

	connectorsCmd.AddCommand(&cobra.Command{
		Use:   "test [connector]",
		Short: "Test connector connection",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			configManager, registry, _ := initializeSystem()
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
				fmt.Println("Enable it with: arkeo connectors enable " + connectorName)
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
				fmt.Println("💡 Check your configuration: arkeo config edit")
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
			configManager, registry, _ := initializeSystem()

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
// ConnectorFactory represents a function that creates a connector
type ConnectorFactory func() connectors.Connector

// Available connector factories for lazy initialization
var connectorFactories = map[string]ConnectorFactory{
	"github":       func() connectors.Connector { return connectors.NewGitHubConnector() },
	"calendar":     func() connectors.Connector { return connectors.NewCalendarConnector() },
	"gitlab":       func() connectors.Connector { return connectors.NewGitLabConnector() },
	"youtrack":     func() connectors.Connector { return connectors.NewYouTrackConnector() },
	"macos_system": func() connectors.Connector { return connectors.NewMacOSSystemConnector() },
}

func initializeSystem() (*config.Manager, *connectors.ConnectorRegistry, *config.PreferencesManager) {
	// Initialize configuration
	configManager := config.NewManager()
	if err := configManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize preferences
	prefsManager := config.NewPreferencesManager(configManager)
	if err := prefsManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading preferences: %v\n", err)
		os.Exit(1)
	}

	// Initialize connector registry with lazy loading
	registry := connectors.NewConnectorRegistry()

	// Register available connectors
	for name, factory := range connectorFactories {
		connector := factory()
		registry.Register(connector)

		// Apply basic configuration even for disabled connectors
		baseConfig := map[string]interface{}{
			connectors.CommonConfigKeys.LogLevel: configManager.GetConfig().App.LogLevel,
		}

		// Get connector config
		connectorConfig, exists := configManager.GetConnectorConfig(name)
		if exists {
			// Add connector-specific config
			for k, v := range connectorConfig.Config {
				baseConfig[k] = v
			}
		}

		// Only configure, but don't enable
		_ = connector.Configure(baseConfig)
	}

	return configManager, registry, prefsManager
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
