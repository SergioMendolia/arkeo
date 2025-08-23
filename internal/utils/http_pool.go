package utils

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HTTPClientPool manages a pool of reusable HTTP clients for different configurations
type HTTPClientPool struct {
	clients sync.Map // map[string]*http.Client keyed by configuration hash
}

// ClientConfig represents HTTP client configuration options
type ClientConfig struct {
	Timeout         time.Duration
	SkipTLSVerify   bool
	MaxIdleConns    int
	IdleConnTimeout time.Duration
}

var (
	// DefaultClientConfig provides sensible defaults for HTTP clients
	DefaultClientConfig = ClientConfig{
		Timeout:         30 * time.Second,
		SkipTLSVerify:   false,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}

	// Global HTTP client pool instance
	globalHTTPPool = &HTTPClientPool{}
)

// GetHTTPClient returns a pooled HTTP client with the specified configuration
func GetHTTPClient(config ClientConfig) *http.Client {
	return globalHTTPPool.GetClient(config)
}

// GetDefaultHTTPClient returns a pooled HTTP client with default configuration
func GetDefaultHTTPClient() *http.Client {
	return globalHTTPPool.GetClient(DefaultClientConfig)
}

// GetClient returns a pooled HTTP client for the given configuration
func (p *HTTPClientPool) GetClient(config ClientConfig) *http.Client {
	// Create a unique key based on configuration
	key := p.configKey(config)

	// Check if client already exists
	if client, ok := p.clients.Load(key); ok {
		return client.(*http.Client)
	}

	// Create new client if not found
	client := p.createClient(config)

	// Store in pool (LoadOrStore handles race conditions)
	if existing, loaded := p.clients.LoadOrStore(key, client); loaded {
		return existing.(*http.Client)
	}

	return client
}

// createClient creates a new HTTP client with the specified configuration
func (p *HTTPClientPool) createClient(config ClientConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: config.MaxIdleConns,
		IdleConnTimeout:     config.IdleConnTimeout,
		DisableKeepAlives:   false,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
		},
	}

	return &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}
}

// configKey generates a unique string key for the client configuration
func (p *HTTPClientPool) configKey(config ClientConfig) string {
	// Simple string concatenation for configuration hash
	// In a production system, you might want to use a proper hash function
	return fmt.Sprintf("timeout=%v_skip_tls=%v_max_idle=%d_idle_timeout=%v",
		config.Timeout,
		config.SkipTLSVerify,
		config.MaxIdleConns,
		config.IdleConnTimeout,
	)
}

// Close closes all pooled HTTP clients and cleans up their connections
func (p *HTTPClientPool) Close() {
	p.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*http.Client); ok {
			if transport, ok := client.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		}
		p.clients.Delete(key)
		return true
	})
}

// CloseGlobalPool closes the global HTTP client pool
func CloseGlobalPool() {
	globalHTTPPool.Close()
}

// GetClientWithTimeout returns a pooled HTTP client with a specific timeout
func GetClientWithTimeout(timeout time.Duration) *http.Client {
	config := DefaultClientConfig
	config.Timeout = timeout
	return GetHTTPClient(config)
}

// GetInsecureClient returns a pooled HTTP client that skips TLS verification
func GetInsecureClient() *http.Client {
	config := DefaultClientConfig
	config.SkipTLSVerify = true
	return GetHTTPClient(config)
}

// GetClientWithTimeoutAndTLS returns a pooled HTTP client with custom timeout and TLS settings
func GetClientWithTimeoutAndTLS(timeout time.Duration, skipTLS bool) *http.Client {
	config := DefaultClientConfig
	config.Timeout = timeout
	config.SkipTLSVerify = skipTLS
	return GetHTTPClient(config)
}
