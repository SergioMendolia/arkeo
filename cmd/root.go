package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/config"
	"github.com/arkeo/arkeo/internal/connectors"
	"github.com/arkeo/arkeo/internal/display"
	"github.com/arkeo/arkeo/internal/editor"
	"github.com/arkeo/arkeo/internal/llm"
	"github.com/arkeo/arkeo/internal/timeline"
)

var (
	configPath  string
	date        string
	format      string
	showDetail  bool
	maxItems    int
	groupByHour bool

	// Analyze command flags
	customPrompt string
	llmModel     string
	debugMode    bool
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
• Analyze your timeline with AI for productivity insights
• Configure connectors through YAML configuration
• Export activity data in various formats

Use the CLI commands to interact with the system and view your daily activities.`,
	Example: `  # Show today's timeline
  arkeo timeline

  # Show timeline for a specific date
  arkeo timeline --date 2023-12-25

  # Show detailed timeline with all information
  arkeo timeline --details

  # Analyze your timeline with AI
  arkeo analyze

  # Analyze with custom prompt
  arkeo analyze --prompt "What were my main focus areas?"

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

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default is $HOME/.config/arkeo/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&date, "date", "", "date for operations (default is today, format: YYYY-MM-DD)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "table", "output format (table, json, csv)")

	// Add subcommands
	rootCmd.AddCommand(timelineCmd)
	rootCmd.AddCommand(connectorsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(llmCmd)
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
			fmt.Println("No connectors are enabled. Use 'arkeo connectors list' to see available connectors.")
			fmt.Println("Enable a connector with: arkeo connectors enable <connector-name>")
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

// llmCmd manages LLM configuration and testing
var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Manage LLM configuration and testing",
	Long: `Configure and test the AI language model integration for timeline analysis.
Use subcommands to test connection, show configuration, and manage LLM settings.`,
}

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

// analyzeCmd sends timeline to LLM for analysis
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze activity timeline using AI",
	Long: `Send the activity timeline for a specific date to an OpenAI-compatible LLM
for analysis and insights. The LLM will provide productivity insights, identify patterns,
and suggest improvements based on your daily activities.`,
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

		// Check LLM configuration
		config := configManager.GetConfig()
		if config.LLM.APIKey == "" {
			fmt.Fprintf(os.Stderr, "LLM API key not configured. Please set it in the configuration file:\n")
			fmt.Fprintf(os.Stderr, "arkeo config edit\n")
			fmt.Fprintf(os.Stderr, "\nOr set the llm.api_key field in your config file.\n")
			os.Exit(1)
		}

		// Show debug information if requested
		if debugMode {
			fmt.Printf("🔍 Debug Information:\n")
			fmt.Printf("  Base URL: %s\n", config.LLM.BaseURL)
			fmt.Printf("  Model: %s\n", config.LLM.Model)
			fmt.Printf("  Max Tokens: %d\n", config.LLM.MaxTokens)
			fmt.Printf("  Temperature: %.1f\n", config.LLM.Temperature)
			fmt.Printf("  Skip TLS: %t\n", config.LLM.SkipTLSVerify)
			fmt.Printf("  API Key: %s\n", func() string {
				key := config.LLM.APIKey
				if len(key) > 8 {
					return key[:4] + "..." + key[len(key)-4:]
				}
				return "***"
			}())
			fmt.Println()
		}

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

		if len(tl.Activities) == 0 {
			fmt.Printf("No activities found for %s. Nothing to analyze.\n", targetDate.Format("January 2, 2006"))
			return
		}

		fmt.Printf("\nAnalyzing %d activities with AI...\n\n", len(tl.Activities))

		// Create LLM client
		llmConfig := llm.Config{
			BaseURL:       config.LLM.BaseURL,
			APIKey:        config.LLM.APIKey,
			Model:         config.LLM.Model,
			MaxTokens:     config.LLM.MaxTokens,
			Temperature:   config.LLM.Temperature,
			SkipTLSVerify: config.LLM.SkipTLSVerify,
		}

		// Override model if specified via flag
		if llmModel != "" {
			llmConfig.Model = llmModel
		}

		client := llm.NewClient(llmConfig)

		// Determine prompt
		prompt := config.LLM.DefaultPrompt
		if customPrompt != "" {
			prompt = customPrompt
		}

		// Send timeline to LLM for analysis
		analysis, err := client.AnalyzeTimeline(ctx, tl, prompt, llmConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error analyzing timeline: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nTroubleshooting:\n")
			fmt.Fprintf(os.Stderr, "• Check your API key configuration\n")
			fmt.Fprintf(os.Stderr, "• Verify the base URL is correct (current: %s)\n", config.LLM.BaseURL)
			fmt.Fprintf(os.Stderr, "• Ensure the model name is valid (current: %s)\n", llmConfig.Model)
			fmt.Fprintf(os.Stderr, "• Test connection: arkeo llm test\n")
			fmt.Fprintf(os.Stderr, "• For self-signed certificates, try setting skip_tls_verify: true\n")
			fmt.Fprintf(os.Stderr, "• Run with --debug flag for more detailed information\n")
			os.Exit(1)
		}

		// Display analysis
		fmt.Println("🤖 AI Analysis")
		fmt.Println("==============")
		fmt.Println(analysis)
	},
}

func init() {
	// Timeline flags
	timelineCmd.Flags().BoolVar(&showDetail, "details", false, "show detailed information for each activity")
	timelineCmd.Flags().IntVar(&maxItems, "max", 500, "maximum number of activities to show")
	timelineCmd.Flags().BoolVar(&groupByHour, "group", false, "group activities by hour")

	// Analyze command flags
	analyzeCmd.Flags().StringVar(&customPrompt, "prompt", "", "custom prompt for AI analysis")
	analyzeCmd.Flags().StringVar(&llmModel, "model", "", "override LLM model to use")
	analyzeCmd.Flags().BoolVar(&debugMode, "debug", false, "show debug information")

	// LLM command flags
	llmCmd.PersistentFlags().StringVar(&llmModel, "model", "", "override LLM model to use")

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
			fmt.Println("💡 Enable a connector: arkeo connectors enable <name>")
			fmt.Println("⚙️  Edit configuration: arkeo config edit")
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
			fmt.Println("💡 Configure it by editing the config file: arkeo config edit")
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
			fmt.Println("💡 Edit configuration: arkeo config edit")
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
			fmt.Println("💡 Edit it with: arkeo config edit")
		},
	})

	// LLM subcommands
	llmCmd.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Test LLM API connection",
		Long: `Test the connection to the configured LLM API endpoint.
This will send a simple test message to verify that the API key, endpoint, and model are working correctly.`,
		Run: func(cmd *cobra.Command, args []string) {
			configManager, _ := initializeSystem()
			config := configManager.GetConfig()

			if config.LLM.APIKey == "" {
				fmt.Fprintf(os.Stderr, "❌ LLM API key not configured\n")
				fmt.Fprintf(os.Stderr, "Configure it with: arkeo config edit\n")
				os.Exit(1)
			}

			llmConfig := llm.Config{
				BaseURL:       config.LLM.BaseURL,
				APIKey:        config.LLM.APIKey,
				Model:         config.LLM.Model,
				MaxTokens:     config.LLM.MaxTokens,
				Temperature:   config.LLM.Temperature,
				SkipTLSVerify: config.LLM.SkipTLSVerify,
			}

			// Override model if specified via flag
			if llmModel != "" {
				llmConfig.Model = llmModel
			}

			client := llm.NewClient(llmConfig)

			fmt.Printf("Testing connection to %s...\n", config.LLM.BaseURL)
			fmt.Printf("Using model: %s\n", llmConfig.Model)

			ctx := context.Background()
			if err := client.TestConnection(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Connection test failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "\nTroubleshooting:\n")
				fmt.Fprintf(os.Stderr, "• Check your API key is valid\n")
				fmt.Fprintf(os.Stderr, "• Verify the base URL is correct\n")
				fmt.Fprintf(os.Stderr, "• Ensure the model name exists\n")
				fmt.Fprintf(os.Stderr, "• Check your network connection\n")
				os.Exit(1)
			}

			fmt.Printf("✅ LLM connection test successful!\n")
		},
	})

	llmCmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show LLM configuration",
		Long:  `Display current LLM configuration settings.`,
		Run: func(cmd *cobra.Command, args []string) {
			configManager, _ := initializeSystem()
			config := configManager.GetConfig()

			fmt.Printf("LLM Configuration\n")
			fmt.Printf("=================\n")
			fmt.Printf("Base URL:     %s\n", config.LLM.BaseURL)
			fmt.Printf("Model:        %s\n", config.LLM.Model)
			fmt.Printf("Max Tokens:   %d\n", config.LLM.MaxTokens)
			fmt.Printf("Temperature:  %.1f\n", config.LLM.Temperature)
			fmt.Printf("Skip TLS:     %t\n", config.LLM.SkipTLSVerify)

			if config.LLM.APIKey == "" {
				fmt.Printf("API Key:      ❌ Not configured\n")
			} else {
				// Show masked API key
				key := config.LLM.APIKey
				if len(key) > 8 {
					key = key[:4] + "..." + key[len(key)-4:]
				}
				fmt.Printf("API Key:      ✅ %s\n", key)
			}

			fmt.Printf("\n💡 Edit configuration: arkeo config edit\n")
			fmt.Printf("💡 Test connection: arkeo llm test\n")
		},
	})

	llmCmd.AddCommand(&cobra.Command{
		Use:   "debug",
		Short: "Debug LLM API connection and response",
		Long: `Send a debug request to the LLM API and show the raw response.
This helps diagnose connection issues by showing exactly what the API returns.`,
		Run: func(cmd *cobra.Command, args []string) {
			configManager, _ := initializeSystem()
			config := configManager.GetConfig()

			if config.LLM.APIKey == "" {
				fmt.Fprintf(os.Stderr, "❌ LLM API key not configured\n")
				fmt.Fprintf(os.Stderr, "Configure it with: arkeo config edit\n")
				os.Exit(1)
			}

			llmConfig := llm.Config{
				BaseURL:       config.LLM.BaseURL,
				APIKey:        config.LLM.APIKey,
				Model:         config.LLM.Model,
				MaxTokens:     50,
				Temperature:   0,
				SkipTLSVerify: config.LLM.SkipTLSVerify,
			}

			if llmModel != "" {
				llmConfig.Model = llmModel
			}

			fmt.Printf("🔍 Debug LLM API Request\n")
			fmt.Printf("========================\n")
			fmt.Printf("URL: %s/chat/completions\n", config.LLM.BaseURL)
			fmt.Printf("Model: %s\n", llmConfig.Model)
			fmt.Printf("Skip TLS: %t\n", config.LLM.SkipTLSVerify)
			fmt.Printf("\n")

			client := llm.NewClient(llmConfig)
			ctx := context.Background()

			// Create a simple debug request
			debugReq := llm.ChatCompletionRequest{
				Model: llmConfig.Model,
				Messages: []llm.ChatMessage{
					{Role: "user", Content: "Reply with exactly: DEBUG_OK"},
				},
				MaxTokens:   10,
				Temperature: 0,
				Stream:      false,
			}

			fmt.Printf("Sending debug request...\n")
			response, err := client.SendDebugRequest(ctx, debugReq)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Debug request failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✅ Debug request successful!\n")
			fmt.Printf("Response: %s\n", response)
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
	registry.Register(connectors.NewYouTrackConnector())
	registry.Register(connectors.NewMacOSSystemConnector())

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
