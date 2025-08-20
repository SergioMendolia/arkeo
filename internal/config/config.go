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

	// UI settings
	UI UIConfig `yaml:"ui" mapstructure:"ui"`

	// Data storage settings
	Storage StorageConfig `yaml:"storage" mapstructure:"storage"`
}

// AppConfig contains application-level settings
type AppConfig struct {
	// Default date format for display
	DateFormat string `yaml:"date_format" mapstructure:"date_format"`

	// Default timezone
	Timezone string `yaml:"timezone" mapstructure:"timezone"`

	// Log level
	LogLevel string `yaml:"log_level" mapstructure:"log_level"`

	// Cache duration for connector data
	CacheDuration string `yaml:"cache_duration" mapstructure:"cache_duration"`
}

// ConnectorConfig holds configuration for a specific connector
type ConnectorConfig struct {
	// Whether the connector is enabled
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`

	// Connector-specific configuration
	Config map[string]interface{} `yaml:"config" mapstructure:"config"`

	// Refresh interval for this connector
	RefreshInterval string `yaml:"refresh_interval" mapstructure:"refresh_interval"`
}

// UIConfig contains user interface settings
type UIConfig struct {
	// Default view when starting the app
	DefaultView string `yaml:"default_view" mapstructure:"default_view"`

	// Color theme
	Theme string `yaml:"theme" mapstructure:"theme"`

	// Show timestamps in timeline
	ShowTimestamps bool `yaml:"show_timestamps" mapstructure:"show_timestamps"`

	// Group activities by time intervals
	GroupByInterval string `yaml:"group_by_interval" mapstructure:"group_by_interval"`

	// Maximum items to show per page
	PageSize int `yaml:"page_size" mapstructure:"page_size"`
}

// StorageConfig contains data storage settings
type StorageConfig struct {
	// Storage type (file, memory, database)
	Type string `yaml:"type" mapstructure:"type"`

	// Storage location (for file-based storage)
	Location string `yaml:"location" mapstructure:"location"`

	// Retention period for activities
	RetentionDays int `yaml:"retention_days" mapstructure:"retention_days"`
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
	m.viper.SetDefault("app.timezone", "Local")
	m.viper.SetDefault("app.log_level", "info")
	m.viper.SetDefault("app.cache_duration", "1h")

	// UI defaults
	m.viper.SetDefault("ui.default_view", "timeline")
	m.viper.SetDefault("ui.theme", "default")
	m.viper.SetDefault("ui.show_timestamps", true)
	m.viper.SetDefault("ui.group_by_interval", "1h")
	m.viper.SetDefault("ui.page_size", 50)

	// Storage defaults
	m.viper.SetDefault("storage.type", "file")
	m.viper.SetDefault("storage.location", "data")
	m.viper.SetDefault("storage.retention_days", 90)
}

// createDefaultConfig creates a default configuration file
func (m *Manager) createDefaultConfig() error {
	// Create default config
	defaultConfig := &Config{
		App: AppConfig{
			DateFormat:    "2006-01-02",
			Timezone:      "Local",
			LogLevel:      "info",
			CacheDuration: "1h",
		},
		Connectors: map[string]ConnectorConfig{
			"github": {
				Enabled: false,
				Config: map[string]interface{}{
					"token":           "",
					"username":        "",
					"include_private": false,
				},
				RefreshInterval: "15m",
			},
			"calendar": {
				Enabled: false,
				Config: map[string]interface{}{
					"provider":         "google",
					"client_id":        "",
					"client_secret":    "",
					"refresh_token":    "",
					"calendar_ids":     "primary",
					"include_declined": false,
				},
				RefreshInterval: "10m",
			},
			"gitlab": {
				Enabled: false,
				Config: map[string]interface{}{
					"gitlab_url": "https://gitlab.com",
					"username":   "",
					"feed_token": "",
				},
				RefreshInterval: "15m",
			},
		},
		UI: UIConfig{
			DefaultView:     "timeline",
			Theme:           "default",
			ShowTimestamps:  true,
			GroupByInterval: "1h",
			PageSize:        50,
		},
		Storage: StorageConfig{
			Type:          "file",
			Location:      "data",
			RetentionDays: 90,
		},
	}

	// Save to viper and file
	m.config = defaultConfig

	// Marshal to viper
	configMap := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(""), &configMap); err == nil {
		for key, value := range configMap {
			m.viper.Set(key, value)
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

	// Validate UI config
	validViews := []string{"timeline", "connectors", "settings"}
	isValidView := false
	for _, view := range validViews {
		if m.config.UI.DefaultView == view {
			isValidView = true
			break
		}
	}
	if !isValidView {
		return fmt.Errorf("invalid default view: %s", m.config.UI.DefaultView)
	}

	// Validate storage config
	validStorageTypes := []string{"file", "memory"}
	isValidStorage := false
	for _, storageType := range validStorageTypes {
		if m.config.Storage.Type == storageType {
			isValidStorage = true
			break
		}
	}
	if !isValidStorage {
		return fmt.Errorf("invalid storage type: %s", m.config.Storage.Type)
	}

	return nil
}

// Reset resets configuration to defaults
func (m *Manager) Reset() error {
	return m.createDefaultConfig()
}
