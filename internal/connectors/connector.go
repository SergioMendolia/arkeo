package connectors

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// CommonConfigKeys defines standard configuration keys used across connectors
var CommonConfigKeys = struct {
	LogLevel   string
	DebugMode  string
	DateFormat string
	MaxItems   string
	Timeout    string
	UseCache   string
	CacheTTL   string
}{
	LogLevel:   "log_level",
	DebugMode:  "debug_mode",
	DateFormat: "date_format",
	MaxItems:   "max_items",
	Timeout:    "timeout",
	UseCache:   "use_cache",
	CacheTTL:   "cache_ttl",
}

// Connector defines the interface for activity connectors
type Connector interface {
	// Name returns the name of the connector
	Name() string

	// Description returns a description of what this connector does
	Description() string

	// IsEnabled returns whether this connector is enabled
	IsEnabled() bool

	// SetEnabled sets the enabled state of the connector
	SetEnabled(enabled bool)

	// Configure sets up the connector with the given configuration
	Configure(config map[string]interface{}) error

	// ValidateConfig validates the connector configuration
	ValidateConfig(config map[string]interface{}) error

	// GetActivities retrieves activities for the specified date
	GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error)

	// GetRequiredConfig returns the required configuration keys for this connector
	GetRequiredConfig() []ConfigField

	// TestConnection tests if the connector can connect to its service
	TestConnection(ctx context.Context) error
}

// ConfigField represents a configuration field required by a connector
type ConfigField struct {
	Key         string      `json:"key"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
}

// CommonConfigFields returns a set of standard configuration fields that can be used by connectors
func CommonConfigFields() map[string]ConfigField {
	return map[string]ConfigField{
		CommonConfigKeys.LogLevel: {
			Key:         CommonConfigKeys.LogLevel,
			Type:        "string",
			Required:    false,
			Description: "Log level (debug, info, warn, error)",
			Default:     "info",
		},
		CommonConfigKeys.DebugMode: {
			Key:         CommonConfigKeys.DebugMode,
			Type:        "bool",
			Required:    false,
			Description: "Enable debug mode",
			Default:     false,
		},
		CommonConfigKeys.DateFormat: {
			Key:         CommonConfigKeys.DateFormat,
			Type:        "string",
			Required:    false,
			Description: "Date format for display",
			Default:     "2006-01-02",
		},
		CommonConfigKeys.MaxItems: {
			Key:         CommonConfigKeys.MaxItems,
			Type:        "int",
			Required:    false,
			Description: "Maximum number of items to fetch (0 for unlimited)",
			Default:     100,
		},
		CommonConfigKeys.Timeout: {
			Key:         CommonConfigKeys.Timeout,
			Type:        "int",
			Required:    false,
			Description: "Timeout in seconds for API requests",
			Default:     30,
		},
		CommonConfigKeys.UseCache: {
			Key:         CommonConfigKeys.UseCache,
			Type:        "bool",
			Required:    false,
			Description: "Whether to use caching for API requests",
			Default:     true,
		},
		CommonConfigKeys.CacheTTL: {
			Key:         CommonConfigKeys.CacheTTL,
			Type:        "int",
			Required:    false,
			Description: "Cache TTL in minutes",
			Default:     60,
		},
	}
}

// ConnectorRegistry manages all available connectors
type ConnectorRegistry struct {
	connectors map[string]Connector
}

// NewConnectorRegistry creates a new connector registry
func NewConnectorRegistry() *ConnectorRegistry {
	return &ConnectorRegistry{
		connectors: make(map[string]Connector),
	}
}

// Register registers a connector with the registry
func (r *ConnectorRegistry) Register(connector Connector) {
	r.connectors[connector.Name()] = connector
}

// Get retrieves a connector by name
func (r *ConnectorRegistry) Get(name string) (Connector, bool) {
	connector, exists := r.connectors[name]
	return connector, exists
}

// List returns all registered connectors
func (r *ConnectorRegistry) List() map[string]Connector {
	result := make(map[string]Connector)
	for name, connector := range r.connectors {
		result[name] = connector
	}
	return result
}

// GetEnabled returns all enabled connectors
func (r *ConnectorRegistry) GetEnabled() map[string]Connector {
	result := make(map[string]Connector)
	for name, connector := range r.connectors {
		if connector.IsEnabled() {
			result[name] = connector
		}
	}
	return result
}

// BaseConnector provides common functionality for connectors
type BaseConnector struct {
	name        string
	description string
	enabled     bool
	config      map[string]interface{}
	httpClient  *http.Client
}

// NewBaseConnector creates a new base connector
func NewBaseConnector(name, description string) *BaseConnector {
	// Initialize with default configuration
	baseConfig := make(map[string]interface{})

	// Apply common default values
	commonFields := CommonConfigFields()
	for _, field := range commonFields {
		if field.Default != nil {
			baseConfig[field.Key] = field.Default
		}
	}

	return &BaseConnector{
		name:        name,
		description: description,
		enabled:     false,
		config:      baseConfig,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// IsDebugMode checks if debug mode is enabled through config or environment variables
func (b *BaseConnector) IsDebugMode() bool {
	// Check explicit debug_mode flag first
	if b.GetConfigBool(CommonConfigKeys.DebugMode) {
		return true
	}

	// Check if log_level in config is set to debug
	if logLevel, ok := b.config[CommonConfigKeys.LogLevel].(string); ok {
		if strings.ToLower(logLevel) == "debug" {
			return true
		}
	}

	// Also check environment variables
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if strings.ToLower(strings.TrimSpace(logLevel)) == "debug" {
			return true
		}
	}

	// Check ARKEO_DEBUG environment variable
	if debug := os.Getenv("ARKEO_DEBUG"); debug != "" {
		if strings.ToLower(strings.TrimSpace(debug)) == "1" ||
			strings.ToLower(strings.TrimSpace(debug)) == "true" {
			return true
		}
	}

	return false
}

// GetHTTPClient returns the HTTP client for making requests
func (b *BaseConnector) GetHTTPClient() *http.Client {
	return b.httpClient
}

// CreateRequest creates an HTTP request with context and properly configured headers
func (b *BaseConnector) CreateRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	return req, nil
}

// CreateAuthRequest creates an HTTP request with Authorization header
func (b *BaseConnector) CreateAuthRequest(ctx context.Context, method, url, authType, token string) (*http.Request, error) {
	req, err := b.CreateRequest(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("%s %s", authType, token))
	return req, nil
}

// CreateBearerRequest creates an HTTP request with Bearer token Authorization
func (b *BaseConnector) CreateBearerRequest(ctx context.Context, method, url, token string) (*http.Request, error) {
	return b.CreateAuthRequest(ctx, method, url, "Bearer", token)
}

// Name returns the connector name
func (b *BaseConnector) Name() string {
	return b.name
}

// Description returns the connector description
func (b *BaseConnector) Description() string {
	return b.description
}

// IsEnabled returns whether the connector is enabled
func (b *BaseConnector) IsEnabled() bool {
	return b.enabled
}

// SetEnabled sets the enabled state
func (b *BaseConnector) SetEnabled(enabled bool) {
	b.enabled = enabled
}

// Configure sets the connector configuration
func (b *BaseConnector) Configure(config map[string]interface{}) error {
	// Create a new config map that merges defaults with provided values
	mergedConfig := make(map[string]interface{})

	// Start with defaults
	commonFields := CommonConfigFields()
	for key, field := range commonFields {
		if field.Default != nil {
			mergedConfig[key] = field.Default
		}
	}

	// Override with current config (preserve existing values)
	for key, value := range b.config {
		mergedConfig[key] = value
	}

	// Apply the new config values
	for key, value := range config {
		mergedConfig[key] = value
	}

	// Validate the merged configuration
	if err := b.ValidateConfig(mergedConfig); err != nil {
		return err
	}

	// Update the connector's config
	b.config = mergedConfig

	// Update HTTP client timeout if specified
	if timeout, ok := b.config[CommonConfigKeys.Timeout].(int); ok {
		b.httpClient.Timeout = time.Duration(timeout) * time.Second
	}

	return nil
}

// GetConfig returns the current configuration
func (b *BaseConnector) GetConfig() map[string]interface{} {
	return b.config
}

// GetConfigWithDefaults returns the current configuration with any missing defaults filled in
func (b *BaseConnector) GetConfigWithDefaults() map[string]interface{} {
	// Start with current config
	result := make(map[string]interface{})
	for k, v := range b.config {
		result[k] = v
	}

	// Add common defaults for any missing keys
	commonFields := CommonConfigFields()
	for key, field := range commonFields {
		if _, exists := result[key]; !exists && field.Default != nil {
			result[key] = field.Default
		}
	}

	return result
}

// GetConfigString returns a string configuration value
func (b *BaseConnector) GetConfigString(key string) string {
	if val, ok := b.config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetConfigBool returns a boolean configuration value
func (b *BaseConnector) GetConfigBool(key string) bool {
	if val, ok := b.config[key]; ok {
		if boolean, ok := val.(bool); ok {
			return boolean
		}
	}
	return false
}

// GetConfigInt returns an integer configuration value
func (b *BaseConnector) GetConfigInt(key string) int {
	if val, ok := b.config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return 0
}

// GetRequiredConfig returns common configuration fields
func (b *BaseConnector) GetRequiredConfig() []ConfigField {
	// Return just the common fields that apply to all connectors
	commonFields := CommonConfigFields()
	fields := make([]ConfigField, 0, len(commonFields))

	for _, field := range commonFields {
		fields = append(fields, field)
	}

	return fields
}

// ValidateConfig validates the configuration (default implementation)
func (b *BaseConnector) ValidateConfig(config map[string]interface{}) error {
	// Base implementation performs common validation for standard fields
	return ValidateConfigFields(config, b.GetRequiredConfig())
}

// MergeConfigFields merges a connector's required fields with common fields
func MergeConfigFields(requiredFields []ConfigField) []ConfigField {
	result := make([]ConfigField, 0, len(requiredFields))

	// Copy all required fields
	result = append(result, requiredFields...)

	// Add common fields that aren't already in the required fields
	commonFields := CommonConfigFields()
	existingKeys := make(map[string]bool)

	for _, field := range requiredFields {
		existingKeys[field.Key] = true
	}

	// Add common fields that don't exist in required fields
	for _, field := range commonFields {
		if !existingKeys[field.Key] {
			result = append(result, field)
		}
	}

	return result
}

// ValidateConfigFields validates configuration against required fields
func ValidateConfigFields(config map[string]interface{}, requiredFields []ConfigField) error {
	for _, field := range requiredFields {
		if field.Required {
			val, exists := config[field.Key]
			if !exists || val == nil {
				return fmt.Errorf("required field '%s' is missing", field.Key)
			}

			// Type validation
			switch field.Type {
			case "string":
				if str, ok := val.(string); !ok || str == "" {
					return fmt.Errorf("field '%s' must be a non-empty string", field.Key)
				}
			case "int":
				if _, ok := val.(int); !ok {
					if _, ok := val.(float64); !ok {
						return fmt.Errorf("field '%s' must be an integer", field.Key)
					}
				}
			case "bool":
				if _, ok := val.(bool); !ok {
					return fmt.Errorf("field '%s' must be a boolean", field.Key)
				}
			case "secret":
				if str, ok := val.(string); !ok || str == "" {
					return fmt.Errorf("field '%s' must be a non-empty secret", field.Key)
				}
			}
		}
	}

	return nil
}
