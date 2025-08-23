package config

import (
	"fmt"
	"os"
	"path/filepath"

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
	GroupByHour    bool `yaml:"group_by_hour" mapstructure:"group_by_hour"`
	MaxItems       int  `yaml:"max_items" mapstructure:"max_items"`
	ShowTimeline   bool `yaml:"show_timeline" mapstructure:"show_timeline"`
	ShowTimestamps bool `yaml:"show_timestamps" mapstructure:"show_timestamps"`

	// Performance preferences
	ParallelFetch bool `yaml:"parallel_fetch" mapstructure:"parallel_fetch"`
	FetchTimeout  int  `yaml:"fetch_timeout" mapstructure:"fetch_timeout"`
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

// setDefaults sets default configuration values
func (m *Manager) setDefaults() {
	// App defaults
	m.viper.SetDefault("app.date_format", "2006-01-02")
	m.viper.SetDefault("app.log_level", "info")

	// Preferences defaults
	m.viper.SetDefault("preferences.use_colors", true)
	m.viper.SetDefault("preferences.show_details", false)
	m.viper.SetDefault("preferences.show_progress", true)
	m.viper.SetDefault("preferences.show_gaps", true)
	m.viper.SetDefault("preferences.default_format", "visual")

	m.viper.SetDefault("preferences.group_by_hour", false)
	m.viper.SetDefault("preferences.max_items", 500)
	m.viper.SetDefault("preferences.show_timeline", false)
	m.viper.SetDefault("preferences.show_timestamps", true)

	m.viper.SetDefault("preferences.parallel_fetch", true)
	m.viper.SetDefault("preferences.fetch_timeout", 30) // 30 seconds

}

// createDefaultConfig creates a default configuration file
func (m *Manager) createDefaultConfig() error {
	// Create default config
	defaultConfig := &Config{
		App: AppConfig{
			DateFormat: "2006-01-02",
			LogLevel:   "info",
		},

		Preferences: PreferencesConfig{
			UseColors:      true,
			ShowDetails:    false,
			ShowProgress:   true,
			ShowGaps:       true,
			DefaultFormat:  "visual",
			GroupByHour:    false,
			MaxItems:       500,
			ShowTimeline:   false,
			ShowTimestamps: true,
			ParallelFetch:  true,
			FetchTimeout:   30,
		},
		Connectors: map[string]ConnectorConfig{
			"github": {
				Enabled: false,
				Config: map[string]interface{}{
					"token":           "",
					"username":        "",
					"include_private": false,
					"max_items":       100,
					"timeout":         30,
				},
			},
			"calendar": {
				Enabled: false,
				Config: map[string]interface{}{
					"ical_urls":        "",
					"include_declined": false,
					"max_items":        100,
					"timeout":          30,
				},
			},
			"gitlab": {
				Enabled: false,
				Config: map[string]interface{}{
					"gitlab_url":   "https://gitlab.com",
					"username":     "",
					"feed_token":   "",
					"access_token": "",
					"max_items":    100,
					"timeout":      30,
				},
			},
			"youtrack": {
				Enabled: false,
				Config: map[string]interface{}{
					"base_url":  "",
					"token":     "",
					"username":  "",
					"max_items": 100,
					"timeout":   30,
				},
			},
			"macos_system": {
				Enabled: false,
				Config: map[string]interface{}{
					"max_items": 100,
					"timeout":   30,
				},
			},
		},
	}

	// Save to viper and file
	m.config = defaultConfig

	// Set config values in viper
	m.viper.Set("app.date_format", defaultConfig.App.DateFormat)
	m.viper.Set("app.log_level", defaultConfig.App.LogLevel)

	// Set preferences configuration
	m.viper.Set("preferences.use_colors", defaultConfig.Preferences.UseColors)
	m.viper.Set("preferences.show_details", defaultConfig.Preferences.ShowDetails)
	m.viper.Set("preferences.show_progress", defaultConfig.Preferences.ShowProgress)
	m.viper.Set("preferences.show_gaps", defaultConfig.Preferences.ShowGaps)
	m.viper.Set("preferences.default_format", defaultConfig.Preferences.DefaultFormat)
	m.viper.Set("preferences.group_by_hour", defaultConfig.Preferences.GroupByHour)
	m.viper.Set("preferences.max_items", defaultConfig.Preferences.MaxItems)
	m.viper.Set("preferences.show_timeline", defaultConfig.Preferences.ShowTimeline)
	m.viper.Set("preferences.show_timestamps", defaultConfig.Preferences.ShowTimestamps)
	m.viper.Set("preferences.parallel_fetch", defaultConfig.Preferences.ParallelFetch)
	m.viper.Set("preferences.fetch_timeout", defaultConfig.Preferences.FetchTimeout)

	// Set connector configurations
	for name, config := range defaultConfig.Connectors {
		m.viper.Set(fmt.Sprintf("connectors.%s.enabled", name), config.Enabled)
		for key, value := range config.Config {
			m.viper.Set(fmt.Sprintf("connectors.%s.config.%s", name, key), value)
		}
	}

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

// copyExampleConfig copies the example config file to the user's config path
func (m *Manager) copyExampleConfig() error {
	// Get directory of executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	// Define possible locations for config.example.yaml
	possiblePaths := []string{
		filepath.Join(exeDir, "config.example.yaml"),                          // Next to executable
		filepath.Join(filepath.Dir(exeDir), "config.example.yaml"),            // Parent dir of executable
		"config.example.yaml",                                                 // Current working directory
		"/etc/arkeo/config.example.yaml",                                      // System-wide config
		filepath.Join(os.Getenv("HOME"), "arkeo/config.example.yaml"),         // User home directory
		filepath.Join(os.Getenv("HOME"), ".config/arkeo/config.example.yaml"), // XDG config dir
	}

	// Find the example config file
	var examplePath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			examplePath = path
			break
		}
	}

	if examplePath == "" {
		// If we can't find the example config, try to create a minimal valid config
		fmt.Fprintf(os.Stderr, "Warning: Could not find config.example.yaml in any standard location.\n")
		fmt.Fprintf(os.Stderr, "Creating a minimal default configuration instead.\n")
		return m.createDefaultConfig()
	}

	// Ensure config directory exists
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create config directory: %v\n", err)
		fmt.Fprintf(os.Stderr, "Falling back to creating minimal default configuration.\n")
		return m.createDefaultConfig()
	}

	// Read example config
	exampleData, err := os.ReadFile(examplePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read example config at %s: %v\n", examplePath, err)
		fmt.Fprintf(os.Stderr, "Falling back to creating minimal default configuration.\n")
		return m.createDefaultConfig()
	}

	// Write to config path
	if err := os.WriteFile(m.configPath, exampleData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to write config file: %v\n", err)
		fmt.Fprintf(os.Stderr, "Falling back to creating minimal default configuration.\n")
		return m.createDefaultConfig()
	}

	fmt.Printf("Successfully copied example config from: %s\n", examplePath)

	// Load the new config
	if err := m.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	// Unmarshal into struct
	m.config = &Config{}
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal new config: %w", err)
	}

	return nil
}
