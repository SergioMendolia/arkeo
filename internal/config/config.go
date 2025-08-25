package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Application settings
	App AppConfig `yaml:"app" mapstructure:"app"`

	// User preferences
	Preferences PreferencesConfig `yaml:"preferences" mapstructure:"preferences"`

	// Connector configurations
	Connectors map[string]ConnectorConfig `yaml:"connectors" mapstructure:"connectors"`
}

// AppConfig contains application-level settings
type AppConfig struct {
	// Default date format for display
	DateFormat string `yaml:"date_format" mapstructure:"date_format"`

	// Log level
	LogLevel string `yaml:"log_level" mapstructure:"log_level"`
}

// ConnectorConfig holds configuration for a specific connector
type ConnectorConfig struct {
	// Whether the connector is enabled
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`

	// Connector-specific configuration
	Config map[string]interface{} `yaml:"config" mapstructure:"config"`
}

// PreferencesConfig contains user preferences that are remembered across sessions
type PreferencesConfig struct {
	// Display preferences
	UseColors     bool   `yaml:"use_colors" mapstructure:"use_colors"`
	ShowDetails   bool   `yaml:"show_details" mapstructure:"show_details"`
	ShowProgress  bool   `yaml:"show_progress" mapstructure:"show_progress"`
	ShowGaps      bool   `yaml:"show_gaps" mapstructure:"show_gaps"`
	DefaultFormat string `yaml:"default_format" mapstructure:"default_format"`

	// Timeline preferences
	GroupByHour bool `yaml:"group_by_hour" mapstructure:"group_by_hour"`
	MaxItems    int  `yaml:"max_items" mapstructure:"max_items"`

	// Performance preferences
	ParallelFetch bool `yaml:"parallel_fetch" mapstructure:"parallel_fetch"`
	FetchTimeout  int  `yaml:"fetch_timeout" mapstructure:"fetch_timeout"`
}

// Preference setter methods for direct config access
func (m *Manager) SetUseColors(value bool) error {
	m.config.Preferences.UseColors = value
	return m.Save()
}

func (m *Manager) SetShowDetails(value bool) error {
	m.config.Preferences.ShowDetails = value
	return m.Save()
}

func (m *Manager) SetDefaultFormat(format string) error {
	m.config.Preferences.DefaultFormat = format
	return m.Save()
}

func (m *Manager) GetPreferences() *PreferencesConfig {
	return &m.config.Preferences
}

// DefaultConfig returns a config with default values and comprehensive documentation
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			DateFormat: "2006-01-02", // Go time format for displaying dates
			LogLevel:   "info",       // Application logging level (debug, info, warn, error)
		},
		Preferences: PreferencesConfig{
			// Display preferences - control how timeline information is presented
			UseColors:     true,    // Use colors in terminal output
			ShowDetails:   false,   // Show detailed information for activities
			ShowProgress:  true,    // Show progress bars during data fetching
			ShowGaps:      true,    // Highlight time gaps in the timeline
			DefaultFormat: "table", // Default output format (table, json, csv)

			// Timeline preferences - control how timeline data is organized
			GroupByHour: false, // Group activities by hour
			MaxItems:    500,   // Maximum number of items to display

			// Performance preferences - control application performance
			ParallelFetch: true, // Fetch data from connectors in parallel
			FetchTimeout:  30,   // Connector timeout in seconds
		},
		Connectors: map[string]ConnectorConfig{
			"github": {
				Enabled: false,
				Config: map[string]interface{}{
					// Get a personal access token from: https://github.com/settings/tokens
					// Requires 'repo' scope for private repos, 'public_repo' for public only
					"token": "",

					// Your GitHub username
					"username": "",

					// Include activities from private repositories
					"include_private": false,
				},
			},
			"calendar": {
				Enabled: false,
				Config: map[string]interface{}{
					// Google Calendar secret iCal URLs (comma-separated)
					// Get these from: Google Calendar > Settings and sharing > Integrate calendar > Secret address in iCal format
					// Format: https://calendar.google.com/calendar/ical/[calendar-id]/[secret-key]/basic.ics
					"ical_urls": "",

					// Include declined calendar events
					"include_declined": false,
				},
			},
			"gitlab": {
				Enabled: false,
				Config: map[string]interface{}{
					// GitLab instance URL (defaults to https://gitlab.com)
					"gitlab_url": "https://gitlab.com",

					// Your GitLab username
					"username": "",

					// GitLab personal access token from Profile > Access Tokens
					// Requires 'read_api' scope to access user events data
					"access_token": "",
				},
			},
			"youtrack": {
				Enabled: false,
				Config: map[string]interface{}{
					// YouTrack instance URL (e.g., https://mycompany.youtrack.cloud/)
					"base_url": "",

					// YouTrack permanent token
					// Get from: Profile > Account Security > New token
					// Requires YouTrack scope permissions
					"token": "",

					// Username to filter activities for (optional, defaults to token owner)
					"username": "",
				},
			},
			"macos_system": {
				Enabled: false,
				Config:  map[string]interface{}{
					// Note: This connector uses the macOS 'log show' command to retrieve system events.
					// It monitors loginwindow events for screen lock state changes for the full day.
					// When the screen is locked, it generates "Computer is idle" activities.
					// When the screen is unlocked, it generates "Computer is active" activities.
					// No additional configuration is required beyond enabling the connector.
				},
			},
			"webhooks": {
				Enabled: false,
				Config: map[string]interface{}{
					// Array of webhook configurations
					// Each webhook should have: name (display name), url (endpoint), token (Bearer token)
					"webhooks": []map[string]interface{}{
						{
							// Display name for activities from this webhook
							"name": "My Service",

							// Webhook endpoint URL (will be called with ?date=YYYY-MM-DD parameter)
							"url": "https://api.myservice.com/activities",

							// Bearer token for authentication
							"token": "your-bearer-token-here",
						},
					},
				},
			},
		},
	}
}

// Manager handles configuration loading, saving, and management
type Manager struct {
	config     *Config
	configPath string
	viper      *viper.Viper
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	v := viper.New()
	return &Manager{
		viper: v,
	}
}

// Load loads configuration from file or creates default config
func (m *Manager) Load() error {
	// Set up config file paths
	configDir, err := m.getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	m.configPath = filepath.Join(configDir, "config.yaml")

	// Set up viper
	m.viper.SetConfigName("config")
	m.viper.SetConfigType("yaml")
	m.viper.AddConfigPath(configDir)

	// Set default values
	m.setDefaults()

	// Try to read existing config
	if err := m.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, create default
			if err := m.createDefaultConfig(); err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into struct
	m.config = &Config{}
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// Save saves the current configuration to file
func (m *Manager) Save() error {
	if m.config == nil {
		return fmt.Errorf("no config to save")
	}

	// Ensure config directory exists
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config to file
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// SetConnectorConfig sets configuration for a specific connector
func (m *Manager) SetConnectorConfig(name string, config ConnectorConfig) {
	if m.config.Connectors == nil {
		m.config.Connectors = make(map[string]ConnectorConfig)
	}
	m.config.Connectors[name] = config
}

// SetConnectorConfigValue sets a specific configuration value for a connector
func (m *Manager) SetConnectorConfigValue(name string, key string, value interface{}) {
	if m.config.Connectors == nil {
		m.config.Connectors = make(map[string]ConnectorConfig)
	}

	connectorConfig, exists := m.config.Connectors[name]
	if !exists {
		connectorConfig = ConnectorConfig{
			Enabled: false,
			Config:  make(map[string]interface{}),
		}
	} else if connectorConfig.Config == nil {
		connectorConfig.Config = make(map[string]interface{})
	}

	connectorConfig.Config[key] = value
	m.config.Connectors[name] = connectorConfig
}

// GetConnectorConfig gets configuration for a specific connector
func (m *Manager) GetConnectorConfig(name string) (ConnectorConfig, bool) {
	if m.config.Connectors == nil {
		return ConnectorConfig{}, false
	}
	config, exists := m.config.Connectors[name]
	return config, exists
}

// GetConnectorConfigValue gets a specific configuration value for a connector
func (m *Manager) GetConnectorConfigValue(name string, key string) (interface{}, bool) {
	config, exists := m.GetConnectorConfig(name)
	if !exists || config.Config == nil {
		return nil, false
	}

	value, exists := config.Config[key]
	return value, exists
}

// GetConnectorConfigString gets a string configuration value for a connector
func (m *Manager) GetConnectorConfigString(name string, key string, defaultValue string) string {
	value, exists := m.GetConnectorConfigValue(name, key)
	if !exists {
		return defaultValue
	}

	strValue, ok := value.(string)
	if !ok {
		return defaultValue
	}
	return strValue
}

// GetConnectorConfigBool gets a boolean configuration value for a connector
func (m *Manager) GetConnectorConfigBool(name string, key string, defaultValue bool) bool {
	value, exists := m.GetConnectorConfigValue(name, key)
	if !exists {
		return defaultValue
	}

	boolValue, ok := value.(bool)
	if !ok {
		return defaultValue
	}
	return boolValue
}

// GetConnectorConfigInt gets an integer configuration value for a connector
func (m *Manager) GetConnectorConfigInt(name string, key string, defaultValue int) int {
	value, exists := m.GetConnectorConfigValue(name, key)
	if !exists {
		return defaultValue
	}

	// Handle both int and float64 (which JSON unmarshaling might use)
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return defaultValue
	}
}

// EnableConnector enables a connector
func (m *Manager) EnableConnector(name string) {
	if m.config.Connectors == nil {
		m.config.Connectors = make(map[string]ConnectorConfig)
	}

	config, exists := m.config.Connectors[name]
	if !exists {
		config = ConnectorConfig{
			Enabled: true,
			Config:  make(map[string]interface{}),
		}
	} else {
		config.Enabled = true
	}

	m.config.Connectors[name] = config
}

// DisableConnector disables a connector
func (m *Manager) DisableConnector(name string) {
	if m.config.Connectors == nil {
		return
	}

	config, exists := m.config.Connectors[name]
	if !exists {
		return
	}

	config.Enabled = false
	m.config.Connectors[name] = config
}

// IsConnectorEnabled checks if a connector is enabled
func (m *Manager) IsConnectorEnabled(name string) bool {
	config, exists := m.GetConnectorConfig(name)
	return exists && config.Enabled
}

// GetConfigPath returns the path to the config file
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// GetDataDir returns the data directory path
func (m *Manager) GetDataDir() (string, error) {
	configDir, err := m.getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "data"), nil
}

// GetConfigDir returns the configuration directory path (public method)
func (m *Manager) GetConfigDir() (string, error) {
	return m.getConfigDir()
}

// getConfigDir returns the configuration directory path
func (m *Manager) getConfigDir() (string, error) {
	// Try XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "arkeo"), nil
	}

	// Fall back to ~/.config/arkeo
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".config", "arkeo"), nil
}

// setDefaults sets default configuration values using the central default config
func (m *Manager) setDefaults() {
	defaults := DefaultConfig()

	// App defaults
	m.viper.SetDefault("app.date_format", defaults.App.DateFormat)
	m.viper.SetDefault("app.log_level", defaults.App.LogLevel)

	// Preferences defaults
	prefs := defaults.Preferences
	m.viper.SetDefault("preferences.use_colors", prefs.UseColors)
	m.viper.SetDefault("preferences.show_details", prefs.ShowDetails)
	m.viper.SetDefault("preferences.show_progress", prefs.ShowProgress)
	m.viper.SetDefault("preferences.show_gaps", prefs.ShowGaps)
	m.viper.SetDefault("preferences.default_format", prefs.DefaultFormat)
	m.viper.SetDefault("preferences.group_by_hour", prefs.GroupByHour)
	m.viper.SetDefault("preferences.max_items", prefs.MaxItems)
	m.viper.SetDefault("preferences.parallel_fetch", prefs.ParallelFetch)
	m.viper.SetDefault("preferences.fetch_timeout", prefs.FetchTimeout)
}

// createDefaultConfig creates a default configuration file
func (m *Manager) createDefaultConfig() error {
	// Use the central default config
	m.config = DefaultConfig()
	return m.Save()
}

// Validate validates the configuration
func (m *Manager) Validate() error {
	if m.config == nil {
		return fmt.Errorf("no config loaded")
	}

	// Validate app config
	if m.config.App.DateFormat == "" {
		return fmt.Errorf("app.date_format cannot be empty")
	}

	if m.config.App.LogLevel == "" {
		return fmt.Errorf("app.log_level cannot be empty")
	}

	return nil
}

// Reset resets configuration to defaults
func (m *Manager) Reset() error {
	return m.copyExampleConfig()
}

// copyExampleConfig creates a default configuration (simplified approach)
func (m *Manager) copyExampleConfig() error {
	// Simply use the default config instead of searching for example files
	fmt.Println("Creating default configuration...")
	return m.createDefaultConfig()
}

// ExportExampleConfig generates a config.example.yaml file with detailed comments
func (m *Manager) ExportExampleConfig(outputPath string) error {
	exampleYAML := m.GenerateExampleConfigYAML()

	if err := os.WriteFile(outputPath, []byte(exampleYAML), 0644); err != nil {
		return fmt.Errorf("failed to write example config: %w", err)
	}

	return nil
}

// GenerateExampleConfigYAML creates a YAML string with the default configuration and detailed comments
func (m *Manager) GenerateExampleConfigYAML() string {
	var b strings.Builder

	b.WriteString("# arkeo Configuration Example\n")
	b.WriteString("# Copy this file to ~/.config/arkeo/config.yaml and customize as needed\n\n")

	// App section
	b.WriteString("# Application settings\n")
	b.WriteString("app:\n")
	b.WriteString("  # Date format for display (Go time format)\n")
	b.WriteString("  date_format: \"2006-01-02\"\n\n")
	b.WriteString("  # Application logging level (debug, info, warn, error)\n")
	b.WriteString("  # Set to \"debug\" to enable detailed logging for connectors\n")
	b.WriteString("  log_level: \"info\"\n\n\n")

	// Preferences section
	b.WriteString("# User preferences configuration\n")
	b.WriteString("# These settings control the application's behavior and appearance\n")
	b.WriteString("preferences:\n")
	b.WriteString("  # Display preferences\n")
	b.WriteString("  # Control how timeline information is presented\n")
	b.WriteString("  use_colors: true          # Use colors in terminal output\n")
	b.WriteString("  show_details: false       # Show detailed information for activities\n")
	b.WriteString("  show_progress: true       # Show progress bars during data fetching\n")
	b.WriteString("  show_gaps: true           # Highlight time gaps in the timeline\n")
	b.WriteString("  default_format: \"table\"  # Default output format (table, json, csv)\n\n")
	b.WriteString("  # Timeline preferences\n")
	b.WriteString("  # Control how timeline data is organized and displayed\n")
	b.WriteString("  group_by_hour: false          # Group activities by hour\n")
	b.WriteString("  max_items: 500                # Maximum number of items to display\n\n")
	b.WriteString("  # Performance preferences\n")
	b.WriteString("  # Control application performance characteristics\n")
	b.WriteString("  parallel_fetch: true          # Fetch data from connectors in parallel\n")
	b.WriteString("  fetch_timeout: 30             # Connector timeout in seconds\n\n\n")

	// Connectors section
	b.WriteString("# Connector configurations\n")
	b.WriteString("connectors:\n")

	// GitHub connector
	b.WriteString("  # GitHub connector - fetches commits, issues, and PRs\n")
	b.WriteString("  github:\n")
	b.WriteString("    enabled: false\n")
	b.WriteString("    config:\n")
	b.WriteString("      # Get a personal access token from: https://github.com/settings/tokens\n")
	b.WriteString("      # Requires 'repo' scope for private repos, 'public_repo' for public only\n")
	b.WriteString("      token: \"ghp_your_github_token_here\"\n\n")
	b.WriteString("      # Your GitHub username\n")
	b.WriteString("      username: \"your-username\"\n\n")
	b.WriteString("      # Include activities from private repositories\n")
	b.WriteString("      include_private: false\n\n\n")

	// Calendar connector
	b.WriteString("  # Google Calendar connector - fetches calendar events using secret iCal URLs\n")
	b.WriteString("  calendar:\n")
	b.WriteString("    enabled: false\n")
	b.WriteString("    config:\n")
	b.WriteString("      # Google Calendar secret iCal URLs (comma-separated)\n")
	b.WriteString("      # Get these from: Google Calendar > Settings and sharing > Integrate calendar > Secret address in iCal format\n")
	b.WriteString("      # Format: https://calendar.google.com/calendar/ical/[calendar-id]/[secret-key]/basic.ics\n")
	b.WriteString("      ical_urls: \"https://calendar.google.com/calendar/ical/your-email@gmail.com/private-abc123def456/basic.ics\"\n\n")
	b.WriteString("      # Include declined calendar events\n")
	b.WriteString("      include_declined: false\n\n")

	// GitLab connector
	b.WriteString("  # GitLab connector - fetches push events from GitLab API (all branches)\n")
	b.WriteString("  gitlab:\n")
	b.WriteString("    enabled: false\n")
	b.WriteString("    config:\n")
	b.WriteString("      # GitLab instance URL (defaults to https://gitlab.com)\n")
	b.WriteString("      gitlab_url: \"https://gitlab.com\"\n\n")
	b.WriteString("      # Your GitLab username\n")
	b.WriteString("      username: \"your-username\"\n\n")
	b.WriteString("      # GitLab personal access token from Profile > Access Tokens\n")
	b.WriteString("      # Requires 'read_api' scope to access user events data\n")
	b.WriteString("      access_token: \"your-gitlab-access-token\"\n\n")

	// YouTrack connector
	b.WriteString("  # YouTrack connector - fetches activities and issue updates from YouTrack\n")
	b.WriteString("  youtrack:\n")
	b.WriteString("    enabled: false\n")
	b.WriteString("    config:\n")
	b.WriteString("      # YouTrack instance URL (e.g., https://mycompany.youtrack.cloud/)\n")
	b.WriteString("      base_url: \"https://mycompany.youtrack.cloud/\"\n\n")
	b.WriteString("      # YouTrack permanent token\n")
	b.WriteString("      # Get from: Profile > Account Security > New token\n")
	b.WriteString("      # Requires YouTrack scope permissions\n")
	b.WriteString("      token: \"perm:your-youtrack-token-here\"\n\n")
	b.WriteString("      # Username to filter activities for (optional, defaults to token owner)\n")
	b.WriteString("      username: \"your-username\"\n\n")

	// macOS System connector
	b.WriteString("  # macOS System Events connector - fetches screen lock/unlock events (macOS only)\n")
	b.WriteString("  macos_system:\n")
	b.WriteString("    enabled: false\n")
	b.WriteString("    config:\n")
	b.WriteString("      # Note: This connector uses the macOS 'log show' command to retrieve system events.\n")
	b.WriteString("      # It specifically monitors loginwindow events for screen lock state changes for the full day (00:00:00 to 23:59:59).\n")
	b.WriteString("      # When the screen is locked, it generates \"Computer is idle\" activities.\n")
	b.WriteString("      # When the screen is unlocked, it generates \"Computer is active\" activities.\n")
	b.WriteString("      # No additional configuration is required beyond enabling the connector.\n\n")

	// Webhooks connector
	b.WriteString("  # Webhooks connector - fetches activities from HTTP webhook endpoints\n")
	b.WriteString("  webhooks:\n")
	b.WriteString("    enabled: false\n")
	b.WriteString("    config:\n")
	b.WriteString("      # Array of webhook configurations\n")
	b.WriteString("      # Each webhook will be called with ?date=YYYY-MM-DD parameter\n")
	b.WriteString("      # Expected response: JSON array of activity objects\n")
	b.WriteString("      webhooks:\n")
	b.WriteString("        - name: \"My Service\"                              # Display name for activities\n")
	b.WriteString("          url: \"https://api.myservice.com/activities\"     # Webhook endpoint URL\n")
	b.WriteString("          token: \"your-bearer-token-here\"                # Bearer token for authentication\n")
	b.WriteString("        # Add more webhooks as needed:\n")
	b.WriteString("        # - name: \"Another Service\"\n")
	b.WriteString("        #   url: \"https://api.another.com/events\"\n")
	b.WriteString("        #   token: \"another-token\"\n\n")

	return b.String()
}
