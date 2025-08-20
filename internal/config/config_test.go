package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.viper == nil {
		t.Error("viper should be initialized")
	}

	if manager.config != nil {
		t.Error("config should be nil before Load() is called")
	}
}

func TestManager_Load(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set HOME to temp directory
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()

	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if manager.config == nil {
		t.Error("config should be loaded")
	}

	if manager.configPath == "" {
		t.Error("configPath should be set")
	}

	// Verify default values
	if manager.config.App.DateFormat != "2006-01-02" {
		t.Errorf("Expected default date format '2006-01-02', got %s", manager.config.App.DateFormat)
	}

	if manager.config.App.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", manager.config.App.LogLevel)
	}

	// Check that config file was created
	if _, err := os.Stat(manager.configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}
}

func TestManager_LoadExistingConfig(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".config", "autotime")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Write existing config
	configContent := `
app:
  date_format: "2006/01/02"
  log_level: "debug"
connectors:
  github:
    enabled: true
    config:
      token: "test-token"
      username: "testuser"
`
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err = manager.Load()

	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify loaded values
	if manager.config.App.DateFormat != "2006/01/02" {
		t.Errorf("Expected date format '2006/01/02', got %s", manager.config.App.DateFormat)
	}

	if manager.config.App.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", manager.config.App.LogLevel)
	}

	// Check connector config
	githubConfig, exists := manager.GetConnectorConfig("github")
	if !exists {
		t.Error("GitHub connector config should exist")
	}

	if !githubConfig.Enabled {
		t.Error("GitHub connector should be enabled")
	}

	if githubConfig.Config["token"] != "test-token" {
		t.Errorf("Expected token 'test-token', got %v", githubConfig.Config["token"])
	}
}

func TestManager_Save(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Modify config
	manager.config.App.LogLevel = "debug"
	manager.EnableConnector("github")

	err = manager.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create new manager and load to verify changes were saved
	newManager := NewManager()
	err = newManager.Load()
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if newManager.config.App.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", newManager.config.App.LogLevel)
	}

	if !newManager.IsConnectorEnabled("github") {
		t.Error("GitHub connector should be enabled")
	}
}

func TestManager_SaveWithoutConfig(t *testing.T) {
	manager := NewManager()
	// Don't call Load(), so config is nil

	err := manager.Save()
	if err == nil {
		t.Error("Save() should fail when config is nil")
	}

	expectedError := "no config to save"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestManager_GetConfig(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	config := manager.GetConfig()
	if config == nil {
		t.Error("GetConfig() should return the loaded config")
	}

	if config != manager.config {
		t.Error("GetConfig() should return the same config instance")
	}
}

func TestManager_SetConnectorConfig(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	connectorConfig := ConnectorConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"api_key": "test-key",
			"url":     "https://api.example.com",
		},
	}

	manager.SetConnectorConfig("testconnector", connectorConfig)

	retrievedConfig, exists := manager.GetConnectorConfig("testconnector")
	if !exists {
		t.Error("Connector config should exist after setting")
	}

	if !retrievedConfig.Enabled {
		t.Error("Connector should be enabled")
	}

	if retrievedConfig.Config["api_key"] != "test-key" {
		t.Errorf("Expected api_key 'test-key', got %v", retrievedConfig.Config["api_key"])
	}
}

func TestManager_GetConnectorConfig(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Test existing connector
	config, exists := manager.GetConnectorConfig("github")
	if !exists {
		t.Error("GitHub connector should exist in default config")
	} else {
		if config.Enabled {
			t.Error("GitHub connector should be disabled by default")
		}
	}

	// Test non-existent connector
	_, exists = manager.GetConnectorConfig("nonexistent")
	if exists {
		t.Error("Non-existent connector should not exist")
	}
}

func TestManager_EnableConnector(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Enable existing connector
	manager.EnableConnector("github")

	if !manager.IsConnectorEnabled("github") {
		t.Error("GitHub connector should be enabled")
	}

	// Enable non-existent connector
	manager.EnableConnector("newconnector")

	if !manager.IsConnectorEnabled("newconnector") {
		t.Error("New connector should be enabled")
	}

	config, exists := manager.GetConnectorConfig("newconnector")
	if !exists {
		t.Error("New connector config should exist")
	}

	if !config.Enabled {
		t.Error("New connector should be enabled")
	}
}

func TestManager_DisableConnector(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Enable then disable
	manager.EnableConnector("github")
	manager.DisableConnector("github")

	if manager.IsConnectorEnabled("github") {
		t.Error("GitHub connector should be disabled")
	}

	// Try to disable non-existent connector (should not panic)
	manager.config.Connectors = nil
	manager.DisableConnector("nonexistent") // Should not panic
}

func TestManager_IsConnectorEnabled(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Test default state (disabled)
	if manager.IsConnectorEnabled("github") {
		t.Error("GitHub connector should be disabled by default")
	}

	// Test after enabling
	manager.EnableConnector("github")
	if !manager.IsConnectorEnabled("github") {
		t.Error("GitHub connector should be enabled")
	}

	// Test non-existent connector
	if manager.IsConnectorEnabled("nonexistent") {
		t.Error("Non-existent connector should not be enabled")
	}
}

func TestManager_GetConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	configPath := manager.GetConfigPath()
	if configPath == "" {
		t.Error("Config path should not be empty")
	}

	expectedPath := filepath.Join(tempDir, ".config", "autotime", "config.yaml")
	if configPath != expectedPath {
		t.Errorf("Expected config path %s, got %s", expectedPath, configPath)
	}
}

func TestManager_GetDataDir(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()

	dataDir, err := manager.GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() failed: %v", err)
	}

	expectedDataDir := filepath.Join(tempDir, ".config", "autotime", "data")
	if dataDir != expectedDataDir {
		t.Errorf("Expected data dir %s, got %s", expectedDataDir, dataDir)
	}
}

func TestManager_XDGConfigHome(t *testing.T) {
	tempDir := t.TempDir()
	xdgConfigDir := filepath.Join(tempDir, "xdg-config")

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("XDG_CONFIG_HOME", originalXDG)
		os.Setenv("HOME", originalHome)
	}()

	os.Setenv("XDG_CONFIG_HOME", xdgConfigDir)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expectedConfigPath := filepath.Join(xdgConfigDir, "autotime", "config.yaml")
	if manager.GetConfigPath() != expectedConfigPath {
		t.Errorf("Expected config path %s, got %s", expectedConfigPath, manager.GetConfigPath())
	}
}

func TestManager_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "no config loaded",
		},
		{
			name: "valid config",
			config: &Config{
				App: AppConfig{
					DateFormat: "2006-01-02",
					LogLevel:   "info",
				},
			},
			expectError: false,
		},
		{
			name: "empty date format",
			config: &Config{
				App: AppConfig{
					DateFormat: "",
					LogLevel:   "info",
				},
			},
			expectError: true,
			errorMsg:    "app.date_format cannot be empty",
		},
		{
			name: "empty log level",
			config: &Config{
				App: AppConfig{
					DateFormat: "2006-01-02",
					LogLevel:   "",
				},
			},
			expectError: true,
			errorMsg:    "app.log_level cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager()
			manager.config = tt.config

			err := manager.Validate()

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestManager_Reset(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Modify config
	manager.config.App.LogLevel = "debug"
	manager.EnableConnector("github")

	// Reset to defaults
	err = manager.Reset()
	if err != nil {
		t.Fatalf("Reset() failed: %v", err)
	}

	// Verify defaults were restored
	if manager.config.App.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", manager.config.App.LogLevel)
	}

	if manager.IsConnectorEnabled("github") {
		t.Error("GitHub connector should be disabled after reset")
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err := manager.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	config := manager.GetConfig()

	// Test default app config
	if config.App.DateFormat != "2006-01-02" {
		t.Errorf("Expected default date format '2006-01-02', got %s", config.App.DateFormat)
	}

	if config.App.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", config.App.LogLevel)
	}

	// Test default connectors exist
	expectedConnectors := []string{"github", "calendar", "gitlab"}
	for _, connectorName := range expectedConnectors {
		config, exists := manager.GetConnectorConfig(connectorName)
		if !exists {
			t.Errorf("Default connector %s should exist", connectorName)
			continue
		}

		if manager.IsConnectorEnabled(connectorName) {
			t.Errorf("Default connector %s should be disabled", connectorName)
		}

		// Check that config map is not nil
		if config.Config == nil {
			t.Errorf("Default connector %s should have config map", connectorName)
		}
	}

	// Test GitHub connector defaults
	githubConfig, exists := manager.GetConnectorConfig("github")
	if exists {
		if githubConfig.Config["token"] != "" {
			t.Error("GitHub token should be empty by default")
		}
		if githubConfig.Config["include_private"] != false {
			t.Error("GitHub include_private should be false by default")
		}
	}

	// Test calendar connector defaults
	calendarConfig, exists := manager.GetConnectorConfig("calendar")
	if exists {
		if calendarConfig.Config["ical_urls"] != "" {
			t.Error("Calendar ical_urls should be empty by default")
		}
		if calendarConfig.Config["include_declined"] != false {
			t.Error("Calendar include_declined should be false by default")
		}
	}
}

func TestManager_LoadInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".config", "autotime")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Write invalid YAML
	invalidYAML := `
app:
  date_format: "2006-01-02"
  log_level: info
    invalid: indentation
`
	err = os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager := NewManager()
	err = manager.Load()

	if err == nil {
		t.Error("Load() should fail with invalid YAML")
	}
}
