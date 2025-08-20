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

// GetConnectorConfig gets configuration for a specific connector
func (m *Manager) GetConnectorConfig(name string) (ConnectorConfig, bool) {
	if m.config.Connectors == nil {
		return ConnectorConfig{}, false
	}
	config, exists := m.config.Connectors[name]
	return config, exists
}

// EnableConnector enables a connector
func (m *Manager) EnableConnector(name string) {
	if m.config.Connectors == nil {
		m.config.Connectors = make(map[string]ConnectorConfig)
	}

	config := m.config.Connectors[name]
	config.Enabled = true
	m.config.Connectors[name] = config
}

// DisableConnector disables a connector
func (m *Manager) DisableConnector(name string) {
	if m.config.Connectors == nil {
		return
	}

	config := m.config.Connectors[name]
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

// getConfigDir returns the configuration directory path
func (m *Manager) getConfigDir() (string, error) {
	// Try XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "autotime"), nil
	}

	// Fall back to ~/.config/autotime
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".config", "autotime"), nil
}

// setDefaults sets default configuration values
func (m *Manager) setDefaults() {
	// App defaults
	m.viper.SetDefault("app.date_format", "2006-01-02")
	m.viper.SetDefault("app.log_level", "info")

}

// createDefaultConfig creates a default configuration file
func (m *Manager) createDefaultConfig() error {
	// Create default config
	defaultConfig := &Config{
		App: AppConfig{
			DateFormat: "2006-01-02",
			LogLevel:   "info",
		},
		Connectors: map[string]ConnectorConfig{
			"github": {
				Enabled: false,
				Config: map[string]interface{}{
					"token":           "",
					"username":        "",
					"include_private": false,
				},
			},
			"calendar": {
				Enabled: false,
				Config: map[string]interface{}{
					"ical_urls":        "",
					"include_declined": false,
				},
			},
			"gitlab": {
				Enabled: false,
				Config: map[string]interface{}{
					"gitlab_url": "https://gitlab.com",
					"username":   "",
					"feed_token": "",
				},
			},
		},
	}

	// Save to viper and file
	m.config = defaultConfig

	// Set config values in viper
	m.viper.Set("app.date_format", defaultConfig.App.DateFormat)
	m.viper.Set("app.log_level", defaultConfig.App.LogLevel)

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
	return m.createDefaultConfig()
}
