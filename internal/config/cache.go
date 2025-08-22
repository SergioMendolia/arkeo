package config

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// CachedManager wraps the regular Manager with caching capabilities
type CachedManager struct {
	*Manager
	mutex         sync.RWMutex
	lastModified  time.Time
	cachedConfig  *Config
	configPath    string
	checkInterval time.Duration
	lastCheck     time.Time
}

// NewCachedManager creates a new cached configuration manager
func NewCachedManager() *CachedManager {
	return &CachedManager{
		Manager:       NewManager(),
		checkInterval: 5 * time.Second, // Check for config changes every 5 seconds
	}
}

// NewCachedManagerWithInterval creates a new cached manager with custom check interval
func NewCachedManagerWithInterval(interval time.Duration) *CachedManager {
	return &CachedManager{
		Manager:       NewManager(),
		checkInterval: interval,
	}
}

// Load loads the configuration and initializes the cache
func (cm *CachedManager) Load() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Load configuration using the base manager
	if err := cm.Manager.Load(); err != nil {
		return err
	}

	// Initialize cache
	cm.cachedConfig = cm.Manager.GetConfig()
	cm.configPath = cm.Manager.GetConfigPath()

	// Get initial modification time
	if cm.configPath != "" {
		if info, err := os.Stat(cm.configPath); err == nil {
			cm.lastModified = info.ModTime()
		}
	}
	cm.lastCheck = time.Now()

	return nil
}

// GetConfig returns the cached configuration, reloading if necessary
func (cm *CachedManager) GetConfig() *Config {
	cm.mutex.RLock()

	// Check if we should check for file changes
	now := time.Now()
	shouldCheck := now.Sub(cm.lastCheck) >= cm.checkInterval

	if !shouldCheck {
		// Return cached config without checking file
		defer cm.mutex.RUnlock()
		return cm.cachedConfig
	}

	// Upgrade to write lock to check and potentially reload
	cm.mutex.RUnlock()
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Update last check time
	cm.lastCheck = now

	// Check if file has been modified or cache was invalidated
	shouldReload := cm.cachedConfig == nil

	if cm.configPath != "" {
		if info, err := os.Stat(cm.configPath); err == nil {
			if info.ModTime().After(cm.lastModified) || shouldReload {
				// File was modified or cache invalidated, reload configuration
				if err := cm.Manager.Load(); err == nil {
					cm.cachedConfig = cm.Manager.GetConfig()
					cm.lastModified = info.ModTime()
				}
				// If reload fails, continue using cached config if available
			}
		}
	}

	// If still no cached config, try to load
	if cm.cachedConfig == nil {
		if err := cm.Manager.Load(); err == nil {
			cm.cachedConfig = cm.Manager.GetConfig()
			// Update config path if it wasn't set before
			if cm.configPath == "" {
				cm.configPath = cm.Manager.GetConfigPath()
			}
		}
	}

	return cm.cachedConfig
}

// getConfigPath returns the configuration file path from the base manager
func (cm *CachedManager) getConfigPath() string {
	return cm.Manager.GetConfigPath()
}

// IsConnectorEnabled returns whether a connector is enabled (cached)
func (cm *CachedManager) IsConnectorEnabled(name string) bool {
	config := cm.GetConfig()
	if connectorConfig, exists := config.Connectors[name]; exists {
		return connectorConfig.Enabled
	}
	return false
}

// GetConnectorConfig returns the configuration for a specific connector (cached)
func (cm *CachedManager) GetConnectorConfig(name string) (ConnectorConfig, bool) {
	config := cm.GetConfig()
	connectorConfig, exists := config.Connectors[name]
	return connectorConfig, exists
}

// InvalidateCache forces a reload on the next GetConfig call
func (cm *CachedManager) InvalidateCache() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.lastModified = time.Time{} // Force reload on next access
	cm.lastCheck = time.Time{}
	cm.cachedConfig = nil // Clear cached config
}

// SetCheckInterval updates the file modification check interval
func (cm *CachedManager) SetCheckInterval(interval time.Duration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.checkInterval = interval
}

// GetCacheStats returns cache statistics
func (cm *CachedManager) GetCacheStats() CacheStats {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var configAge time.Duration
	if !cm.lastModified.IsZero() {
		configAge = time.Since(cm.lastModified)
	}

	return CacheStats{
		LastModified:  cm.lastModified,
		LastCheck:     cm.lastCheck,
		CheckInterval: cm.checkInterval,
		ConfigAge:     configAge,
		HasCache:      cm.cachedConfig != nil,
	}
}

// CacheStats contains statistics about the configuration cache
type CacheStats struct {
	LastModified  time.Time     `json:"last_modified"`
	LastCheck     time.Time     `json:"last_check"`
	CheckInterval time.Duration `json:"check_interval"`
	ConfigAge     time.Duration `json:"config_age"`
	HasCache      bool          `json:"has_cache"`
}

// String returns a formatted string representation of cache stats
func (cs CacheStats) String() string {
	if !cs.HasCache {
		return "Configuration cache: not initialized"
	}

	return fmt.Sprintf(
		"Configuration cache: last modified %v ago, last checked %v ago, check interval %v",
		cs.ConfigAge.Round(time.Second),
		time.Since(cs.LastCheck).Round(time.Second),
		cs.CheckInterval,
	)
}

// PrewarmCache loads the configuration into cache without waiting for first access
func (cm *CachedManager) PrewarmCache() error {
	return cm.Load()
}

// WatchConfig starts a goroutine that watches for configuration file changes
// and automatically invalidates the cache when changes are detected
func (cm *CachedManager) WatchConfig(stopChan <-chan struct{}) {
	ticker := time.NewTicker(cm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Trigger cache check by calling GetConfig
			_ = cm.GetConfig()
		case <-stopChan:
			return
		}
	}
}
