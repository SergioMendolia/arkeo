package connectors

import (
	"context"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// Connector defines the interface for activity connectors
type Connector interface {
	// Name returns the name of the connector
	Name() string

	// Description returns a description of what this connector does
	Description() string

	// IsEnabled returns whether this connector is enabled
	IsEnabled() bool

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
	Key         string `json:"key"`
	Type        string `json:"type"` // "string", "int", "bool", "secret"
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
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
}

// NewBaseConnector creates a new base connector
func NewBaseConnector(name, description string) *BaseConnector {
	return &BaseConnector{
		name:        name,
		description: description,
		enabled:     false,
		config:      make(map[string]interface{}),
	}
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
	if err := b.ValidateConfig(config); err != nil {
		return err
	}
	b.config = config
	return nil
}

// GetConfig returns the current configuration
func (b *BaseConnector) GetConfig() map[string]interface{} {
	return b.config
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

// ValidateConfig validates the configuration (default implementation)
func (b *BaseConnector) ValidateConfig(config map[string]interface{}) error {
	// Base implementation - can be overridden by specific connectors
	return nil
}
