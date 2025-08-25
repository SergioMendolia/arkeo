package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// connectorsCmd manages connectors
var connectorsCmd = &cobra.Command{
	Use:   "connectors",
	Short: "Manage service connectors",
	Long: `Manage and configure connectors for various services like GitHub,
Calendar, File System, etc. Use subcommands to list, enable, disable, and test connectors.`,
}

func init() {
	// Add connectors subcommands
	connectorsCmd.AddCommand(connectorsListCmd)
	connectorsCmd.AddCommand(connectorsEnableCmd)
	connectorsCmd.AddCommand(connectorsDisableCmd)
	connectorsCmd.AddCommand(connectorsInfoCmd)
	connectorsCmd.AddCommand(connectorsTestCmd)
}

var connectorsListCmd = &cobra.Command{
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
}

var connectorsEnableCmd = &cobra.Command{
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
}

var connectorsDisableCmd = &cobra.Command{
	Use:   "disable [connector]",
	Short: "Disable a connector",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configManager, registry := initializeSystem()
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
}

var connectorsInfoCmd = &cobra.Command{
	Use:   "info [connector]",
	Short: "Show connector information and configuration requirements",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configManager, registry := initializeSystem()
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
}

var connectorsTestCmd = &cobra.Command{
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
}