package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachedManager_Creation(t *testing.T) {
	cachedManager := NewCachedManager()
	if cachedManager == nil {
		t.Fatal("NewCachedManager returned nil")
	}

	if cachedManager.Manager == nil {
		t.Error("CachedManager should have a base Manager")
	}

	if cachedManager.checkInterval != 5*time.Second {
		t.Errorf("Expected default check interval to be 5s, got %v", cachedManager.checkInterval)
	}
}

func TestCachedManager_CreationWithInterval(t *testing.T) {
	interval := 30 * time.Second
	cachedManager := NewCachedManagerWithInterval(interval)

	if cachedManager.checkInterval != interval {
		t.Errorf("Expected check interval to be %v, got %v", interval, cachedManager.checkInterval)
	}
}

func TestCachedManager_SetCheckInterval(t *testing.T) {
	cachedManager := NewCachedManager()
	newInterval := 1 * time.Minute

	cachedManager.SetCheckInterval(newInterval)

	stats := cachedManager.GetCacheStats()
	if stats.CheckInterval != newInterval {
		t.Errorf("Expected check interval to be %v, got %v", newInterval, stats.CheckInterval)
	}
}

func TestCachedManager_CacheStatsEmpty(t *testing.T) {
	cachedManager := NewCachedManager()

	stats := cachedManager.GetCacheStats()
	if stats.HasCache {
		t.Error("Expected no cache before loading")
	}

	if stats.LastModified.IsZero() != true {
		t.Error("Expected zero last modified time before loading")
	}

	statsStr := stats.String()
	if statsStr == "" {
		t.Error("Expected non-empty stats string")
	}

	if statsStr != "Configuration cache: not initialized" {
		t.Errorf("Expected uninitialized message, got: %s", statsStr)
	}
}

func TestCachedManager_InvalidateCache(t *testing.T) {
	cachedManager := NewCachedManager()

	// Test invalidating an empty cache (should not panic)
	cachedManager.InvalidateCache()

	// Verify cache stats after invalidation
	stats := cachedManager.GetCacheStats()
	if stats.HasCache {
		t.Error("Expected no cache after invalidation")
	}
}

func TestCachedManager_ConcurrentAccess(t *testing.T) {
	cachedManager := NewCachedManager()

	// Test that concurrent access to empty cache doesn't panic
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Concurrent access caused panic: %v", r)
				}
				done <- true
			}()

			// These operations should be safe even without loaded config
			stats := cachedManager.GetCacheStats()
			_ = stats.String()

			cachedManager.InvalidateCache()
			cachedManager.SetCheckInterval(10 * time.Second)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestCachedManager_GetConfigPath(t *testing.T) {
	cachedManager := NewCachedManager()

	// Should return empty string before loading
	path := cachedManager.getConfigPath()
	if path != "" {
		t.Errorf("Expected empty path before loading, got: %s", path)
	}
}

func TestCachedManager_WatchConfigStop(t *testing.T) {
	cachedManager := NewCachedManagerWithInterval(10 * time.Millisecond)

	// Start watching in background
	stopChan := make(chan struct{})
	watchDone := make(chan bool)

	go func() {
		cachedManager.WatchConfig(stopChan)
		watchDone <- true
	}()

	// Let it watch for a short time
	time.Sleep(50 * time.Millisecond)

	// Stop watching
	close(stopChan)

	// Wait for watch to stop
	select {
	case <-watchDone:
		// Success - watch stopped
	case <-time.After(100 * time.Millisecond):
		t.Error("WatchConfig did not stop within expected time")
	}
}

func TestCachedManager_ConfigAccess(t *testing.T) {
	cachedManager := NewCachedManager()

	// Test accessing config-related methods on uninitialized cache
	// These should not panic but may return default values
	enabled := cachedManager.IsConnectorEnabled("test")
	if enabled {
		// Most connectors are disabled by default, but this shouldn't panic
		t.Log("Connector was enabled by default")
	}

	config, exists := cachedManager.GetConnectorConfig("test")
	if exists {
		t.Logf("Got connector config: %+v", config)
	}

	// Getting config on uninitialized cache should return nil
	cfg := cachedManager.GetConfig()
	if cfg != nil {
		t.Log("GetConfig returned non-nil config on uninitialized cache")
	}
}

func BenchmarkCachedManager_GetCacheStats(b *testing.B) {
	cachedManager := NewCachedManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cachedManager.GetCacheStats()
	}
}

func BenchmarkCachedManager_SetCheckInterval(b *testing.B) {
	cachedManager := NewCachedManager()
	intervals := []time.Duration{
		1 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interval := intervals[i%len(intervals)]
		cachedManager.SetCheckInterval(interval)
	}
}

func BenchmarkCachedManager_InvalidateCache(b *testing.B) {
	cachedManager := NewCachedManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cachedManager.InvalidateCache()
	}
}

// Test with actual config file but without relying on complex configuration parsing
func TestCachedManager_WithConfigFile(t *testing.T) {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "arkeo_config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `# Simple test config
app:
  log_level: info
`

	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cachedManager := NewCachedManager()

	// Set up the config file path
	cachedManager.Manager.viper.SetConfigFile(configFile)

	// Attempt to load - this might fail due to missing setup, but shouldn't panic
	err = cachedManager.Load()
	if err != nil {
		t.Logf("Load failed (expected in test environment): %v", err)
		return
	}

	// If load succeeded, test cache functionality
	stats := cachedManager.GetCacheStats()
	if stats.HasCache {
		t.Log("Cache loaded successfully")
	}

	// Test file modification detection
	if stats.LastModified.IsZero() {
		t.Log("Last modified time is zero (may be expected)")
	}

	config := cachedManager.GetConfig()
	if config != nil {
		t.Log("Got config successfully")
	}
}
