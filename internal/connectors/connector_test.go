package connectors

import (
	"context"
	"testing"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// MockConnector implements the Connector interface for testing
type MockConnector struct {
	*BaseConnector
	activities        []timeline.Activity
	testConnectionErr error
	configValidation  func(map[string]interface{}) error
	requiredConfig    []ConfigField
}

func NewMockConnector(name, description string) *MockConnector {
	return &MockConnector{
		BaseConnector: NewBaseConnector(name, description),
		activities:    []timeline.Activity{},
		requiredConfig: []ConfigField{
			{
				Key:         "api_key",
				Type:        "secret",
				Required:    true,
				Description: "API key for the service",
			},
			{
				Key:         "enabled",
				Type:        "bool",
				Required:    false,
				Description: "Whether the connector is enabled",
				Default:     "false",
			},
		},
	}
}

func (m *MockConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	return m.activities, nil
}

func (m *MockConnector) SetActivities(activities []timeline.Activity) {
	m.activities = activities
}

func (m *MockConnector) TestConnection(ctx context.Context) error {
	return m.testConnectionErr
}

func (m *MockConnector) SetTestConnectionError(err error) {
	m.testConnectionErr = err
}

func (m *MockConnector) ValidateConfig(config map[string]interface{}) error {
	if m.configValidation != nil {
		return m.configValidation(config)
	}
	return m.BaseConnector.ValidateConfig(config)
}

func (m *MockConnector) SetConfigValidation(fn func(map[string]interface{}) error) {
	m.configValidation = fn
}

func (m *MockConnector) GetRequiredConfig() []ConfigField {
	return m.requiredConfig
}

func (m *MockConnector) SetRequiredConfig(fields []ConfigField) {
	m.requiredConfig = fields
}

func TestNewConnectorRegistry(t *testing.T) {
	registry := NewConnectorRegistry()

	if registry == nil {
		t.Fatal("NewConnectorRegistry returned nil")
	}

	if registry.connectors == nil {
		t.Error("connectors map should be initialized")
	}

	if len(registry.connectors) != 0 {
		t.Error("new registry should be empty")
	}
}

func TestConnectorRegistry_Register(t *testing.T) {
	registry := NewConnectorRegistry()
	connector := NewMockConnector("test", "Test connector")

	registry.Register(connector)

	if len(registry.connectors) != 1 {
		t.Errorf("Expected 1 connector, got %d", len(registry.connectors))
	}

	if registry.connectors["test"] != connector {
		t.Error("Connector not properly registered")
	}
}

func TestConnectorRegistry_Get(t *testing.T) {
	registry := NewConnectorRegistry()
	connector := NewMockConnector("test", "Test connector")
	registry.Register(connector)

	// Test existing connector
	retrieved, exists := registry.Get("test")
	if !exists {
		t.Error("Connector should exist")
	}
	if retrieved != connector {
		t.Error("Retrieved connector should be the same instance")
	}

	// Test non-existent connector
	_, exists = registry.Get("nonexistent")
	if exists {
		t.Error("Non-existent connector should not exist")
	}
}

func TestConnectorRegistry_List(t *testing.T) {
	registry := NewConnectorRegistry()
	connector1 := NewMockConnector("test1", "Test connector 1")
	connector2 := NewMockConnector("test2", "Test connector 2")

	registry.Register(connector1)
	registry.Register(connector2)

	list := registry.List()

	if len(list) != 2 {
		t.Errorf("Expected 2 connectors in list, got %d", len(list))
	}

	if list["test1"] != connector1 {
		t.Error("First connector not in list")
	}

	if list["test2"] != connector2 {
		t.Error("Second connector not in list")
	}

	// Verify it's a copy (modifying the returned map shouldn't affect the registry)
	delete(list, "test1")
	if len(registry.connectors) != 2 {
		t.Error("Original registry should not be affected by modifications to returned list")
	}
}

func TestConnectorRegistry_GetEnabled(t *testing.T) {
	registry := NewConnectorRegistry()
	connector1 := NewMockConnector("enabled", "Enabled connector")
	connector2 := NewMockConnector("disabled", "Disabled connector")

	connector1.SetEnabled(true)
	connector2.SetEnabled(false)

	registry.Register(connector1)
	registry.Register(connector2)

	enabled := registry.GetEnabled()

	if len(enabled) != 1 {
		t.Errorf("Expected 1 enabled connector, got %d", len(enabled))
	}

	if enabled["enabled"] != connector1 {
		t.Error("Enabled connector not in list")
	}

	if _, exists := enabled["disabled"]; exists {
		t.Error("Disabled connector should not be in enabled list")
	}
}

func TestNewBaseConnector(t *testing.T) {
	name := "test"
	description := "Test connector"

	connector := NewBaseConnector(name, description)

	if connector == nil {
		t.Fatal("NewBaseConnector returned nil")
	}

	if connector.Name() != name {
		t.Errorf("Expected name %q, got %q", name, connector.Name())
	}

	if connector.Description() != description {
		t.Errorf("Expected description %q, got %q", description, connector.Description())
	}

	if connector.IsEnabled() {
		t.Error("New connector should be disabled by default")
	}

	if connector.config == nil {
		t.Error("Config map should be initialized")
	}
}

func TestBaseConnector_SetEnabled(t *testing.T) {
	connector := NewBaseConnector("test", "Test connector")

	// Initially disabled
	if connector.IsEnabled() {
		t.Error("New connector should be disabled")
	}

	// Enable
	connector.SetEnabled(true)
	if !connector.IsEnabled() {
		t.Error("Connector should be enabled")
	}

	// Disable
	connector.SetEnabled(false)
	if connector.IsEnabled() {
		t.Error("Connector should be disabled")
	}
}

func TestBaseConnector_Configure(t *testing.T) {
	connector := NewBaseConnector("test", "Test connector")

	config := map[string]interface{}{
		"api_key": "test-key",
		"timeout": 30,
		"enabled": true,
	}

	err := connector.Configure(config)
	if err != nil {
		t.Errorf("Configure should not fail: %v", err)
	}

	retrievedConfig := connector.GetConfig()
	// The BaseConnector now includes common configuration fields from CommonConfigFields(),
	// so the actual config will have more items than just what we provided.
	// Instead of checking the total count, we should verify our provided values are present.

	// Verify that our provided config values are correctly stored
	for key, value := range config {
		if retrievedConfig[key] != value {
			t.Errorf("Config key %s: expected %v, got %v", key, value, retrievedConfig[key])
		}
	}

	// Verify a few common config fields are present with default values
	if _, exists := retrievedConfig[CommonConfigKeys.LogLevel]; !exists {
		t.Error("Common config field log_level should be present")
	}

	if _, exists := retrievedConfig[CommonConfigKeys.Timeout]; !exists {
		t.Error("Common config field timeout should be present")
	}
}

func TestBaseConnector_GetConfigString(t *testing.T) {
	connector := NewBaseConnector("test", "Test connector")

	config := map[string]interface{}{
		"string_key": "test-value",
		"int_key":    123,
		"bool_key":   true,
	}

	connector.Configure(config)

	// Test existing string key
	if connector.GetConfigString("string_key") != "test-value" {
		t.Errorf("Expected 'test-value', got %q", connector.GetConfigString("string_key"))
	}

	// Test non-string value
	if connector.GetConfigString("int_key") != "" {
		t.Error("Non-string value should return empty string")
	}

	// Test non-existent key
	if connector.GetConfigString("nonexistent") != "" {
		t.Error("Non-existent key should return empty string")
	}
}

func TestBaseConnector_GetConfigBool(t *testing.T) {
	connector := NewBaseConnector("test", "Test connector")

	config := map[string]interface{}{
		"bool_true":  true,
		"bool_false": false,
		"string_key": "not-a-bool",
		"int_key":    1,
	}

	connector.Configure(config)

	// Test existing bool keys
	if !connector.GetConfigBool("bool_true") {
		t.Error("bool_true should return true")
	}

	if connector.GetConfigBool("bool_false") {
		t.Error("bool_false should return false")
	}

	// Test non-bool value
	if connector.GetConfigBool("string_key") {
		t.Error("Non-bool value should return false")
	}

	// Test non-existent key
	if connector.GetConfigBool("nonexistent") {
		t.Error("Non-existent key should return false")
	}
}

func TestBaseConnector_GetConfigInt(t *testing.T) {
	connector := NewBaseConnector("test", "Test connector")

	config := map[string]interface{}{
		"int_key":    123,
		"float_key":  45.67,
		"string_key": "not-an-int",
		"bool_key":   true,
	}

	connector.Configure(config)

	// Test existing int key
	if connector.GetConfigInt("int_key") != 123 {
		t.Errorf("Expected 123, got %d", connector.GetConfigInt("int_key"))
	}

	// Test float64 value (should be converted)
	if connector.GetConfigInt("float_key") != 45 {
		t.Errorf("Expected 45, got %d", connector.GetConfigInt("float_key"))
	}

	// Test non-numeric value
	if connector.GetConfigInt("string_key") != 0 {
		t.Error("Non-numeric value should return 0")
	}

	// Test non-existent key
	if connector.GetConfigInt("nonexistent") != 0 {
		t.Error("Non-existent key should return 0")
	}
}

func TestBaseConnector_ValidateConfig(t *testing.T) {
	connector := NewBaseConnector("test", "Test connector")

	// Base implementation should always pass
	config := map[string]interface{}{
		"any_key": "any_value",
	}

	err := connector.ValidateConfig(config)
	if err != nil {
		t.Errorf("Base ValidateConfig should not fail: %v", err)
	}
}

func TestMockConnector_Interface(t *testing.T) {
	// Verify MockConnector implements the Connector interface
	var _ Connector = &MockConnector{}

	connector := NewMockConnector("mock", "Mock connector for testing")

	if connector.Name() != "mock" {
		t.Errorf("Expected name 'mock', got %q", connector.Name())
	}

	if connector.Description() != "Mock connector for testing" {
		t.Errorf("Expected description 'Mock connector for testing', got %q", connector.Description())
	}
}

func TestMockConnector_GetActivities(t *testing.T) {
	connector := NewMockConnector("mock", "Mock connector")

	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Test with no activities
	activities, err := connector.GetActivities(ctx, date)
	if err != nil {
		t.Errorf("GetActivities failed: %v", err)
	}

	if len(activities) != 0 {
		t.Errorf("Expected 0 activities, got %d", len(activities))
	}

	// Test with some activities
	testActivities := []timeline.Activity{
		{
			ID:        "test-1",
			Type:      timeline.ActivityTypeGitCommit,
			Title:     "Test commit",
			Timestamp: date,
			Source:    "mock",
		},
	}

	connector.SetActivities(testActivities)
	activities, err = connector.GetActivities(ctx, date)
	if err != nil {
		t.Errorf("GetActivities failed: %v", err)
	}

	if len(activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(activities))
	}

	if activities[0].ID != "test-1" {
		t.Errorf("Expected activity ID 'test-1', got %q", activities[0].ID)
	}
}

func TestMockConnector_TestConnection(t *testing.T) {
	connector := NewMockConnector("mock", "Mock connector")
	ctx := context.Background()

	// Test successful connection
	err := connector.TestConnection(ctx)
	if err != nil {
		t.Errorf("TestConnection should succeed by default: %v", err)
	}

	// Test connection error
	expectedErr := context.DeadlineExceeded
	connector.SetTestConnectionError(expectedErr)

	err = connector.TestConnection(ctx)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMockConnector_GetRequiredConfig(t *testing.T) {
	connector := NewMockConnector("mock", "Mock connector")

	config := connector.GetRequiredConfig()

	if len(config) != 2 {
		t.Errorf("Expected 2 config fields, got %d", len(config))
	}

	// Check first field
	if config[0].Key != "api_key" {
		t.Errorf("Expected first field key 'api_key', got %q", config[0].Key)
	}

	if config[0].Type != "secret" {
		t.Errorf("Expected first field type 'secret', got %q", config[0].Type)
	}

	if !config[0].Required {
		t.Error("api_key should be required")
	}

	// Check second field
	if config[1].Key != "enabled" {
		t.Errorf("Expected second field key 'enabled', got %q", config[1].Key)
	}

	if config[1].Required {
		t.Error("enabled should not be required")
	}

	if config[1].Default != "false" {
		t.Errorf("Expected default 'false', got %q", config[1].Default)
	}
}

func TestConfigField(t *testing.T) {
	field := ConfigField{
		Key:         "test_key",
		Type:        "string",
		Required:    true,
		Description: "A test configuration field",
		Default:     "default_value",
	}

	if field.Key != "test_key" {
		t.Errorf("Expected key 'test_key', got %q", field.Key)
	}

	if field.Type != "string" {
		t.Errorf("Expected type 'string', got %q", field.Type)
	}

	if !field.Required {
		t.Error("Field should be required")
	}

	if field.Description != "A test configuration field" {
		t.Errorf("Expected description 'A test configuration field', got %q", field.Description)
	}

	if field.Default != "default_value" {
		t.Errorf("Expected default 'default_value', got %q", field.Default)
	}
}

// Integration tests

func TestConnectorRegistryIntegration(t *testing.T) {
	registry := NewConnectorRegistry()

	// Create multiple connectors with different states
	connector1 := NewMockConnector("github", "GitHub connector")
	connector2 := NewMockConnector("calendar", "Calendar connector")
	connector3 := NewMockConnector("jira", "Jira connector")

	connector1.SetEnabled(true)
	connector2.SetEnabled(false)
	connector3.SetEnabled(true)

	registry.Register(connector1)
	registry.Register(connector2)
	registry.Register(connector3)

	// Test full workflow
	all := registry.List()
	if len(all) != 3 {
		t.Errorf("Expected 3 total connectors, got %d", len(all))
	}

	enabled := registry.GetEnabled()
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled connectors, got %d", len(enabled))
	}

	// Test individual retrieval
	github, exists := registry.Get("github")
	if !exists {
		t.Error("GitHub connector should exist")
	}

	if !github.IsEnabled() {
		t.Error("GitHub connector should be enabled")
	}

	calendar, exists := registry.Get("calendar")
	if !exists {
		t.Error("Calendar connector should exist")
	}

	if calendar.IsEnabled() {
		t.Error("Calendar connector should be disabled")
	}
}

func TestBaseConnectorConfigurationWorkflow(t *testing.T) {
	connector := NewBaseConnector("test", "Test workflow")

	// Initial state
	if connector.IsEnabled() {
		t.Error("Connector should start disabled")
	}

	// Configure
	config := map[string]interface{}{
		"api_url":    "https://api.example.com",
		"api_key":    "secret-key",
		"timeout":    30,
		"retry":      true,
		"batch_size": 100,
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Configuration failed: %v", err)
	}

	// Enable
	connector.SetEnabled(true)
	if !connector.IsEnabled() {
		t.Error("Connector should be enabled")
	}

	// Verify configuration values
	if connector.GetConfigString("api_url") != "https://api.example.com" {
		t.Error("API URL not configured correctly")
	}

	if connector.GetConfigString("api_key") != "secret-key" {
		t.Error("API key not configured correctly")
	}

	if connector.GetConfigInt("timeout") != 30 {
		t.Error("Timeout not configured correctly")
	}

	if !connector.GetConfigBool("retry") {
		t.Error("Retry flag not configured correctly")
	}

	if connector.GetConfigInt("batch_size") != 100 {
		t.Error("Batch size not configured correctly")
	}
}

// Benchmark tests

func BenchmarkConnectorRegistry_Register(b *testing.B) {
	registry := NewConnectorRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connector := NewMockConnector("test", "Test connector")
		registry.Register(connector)
	}
}

func BenchmarkConnectorRegistry_Get(b *testing.B) {
	registry := NewConnectorRegistry()
	connector := NewMockConnector("test", "Test connector")
	registry.Register(connector)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("test")
	}
}

func BenchmarkBaseConnector_GetConfigString(b *testing.B) {
	connector := NewBaseConnector("test", "Test connector")
	config := map[string]interface{}{
		"test_key": "test_value",
	}
	connector.Configure(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connector.GetConfigString("test_key")
	}
}
