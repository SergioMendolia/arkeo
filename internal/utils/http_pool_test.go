package utils

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPClientPool_GetClient(t *testing.T) {
	pool := &HTTPClientPool{}

	config := ClientConfig{
		Timeout:         30 * time.Second,
		SkipTLSVerify:   false,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}

	// Test that we get a client
	client1 := pool.GetClient(config)
	if client1 == nil {
		t.Fatal("GetClient returned nil")
	}

	// Test that we get the same client for the same config
	client2 := pool.GetClient(config)
	if client1 != client2 {
		t.Error("Expected same client instance for identical config")
	}

	// Test that we get a different client for different config
	config2 := config
	config2.Timeout = 60 * time.Second

	client3 := pool.GetClient(config2)
	if client1 == client3 {
		t.Error("Expected different client instance for different config")
	}
}

func TestHTTPClientPool_ConfigKey(t *testing.T) {
	pool := &HTTPClientPool{}

	config1 := ClientConfig{
		Timeout:         30 * time.Second,
		SkipTLSVerify:   false,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}

	config2 := ClientConfig{
		Timeout:         30 * time.Second,
		SkipTLSVerify:   false,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	}

	key1 := pool.configKey(config1)
	key2 := pool.configKey(config2)

	if key1 != key2 {
		t.Error("Expected same key for identical configs")
	}

	// Test different config produces different key
	config3 := config1
	config3.Timeout = 60 * time.Second

	key3 := pool.configKey(config3)
	if key1 == key3 {
		t.Error("Expected different key for different configs")
	}
}

func TestGetDefaultHTTPClient(t *testing.T) {
	client1 := GetDefaultHTTPClient()
	client2 := GetDefaultHTTPClient()

	if client1 == nil {
		t.Fatal("GetDefaultHTTPClient returned nil")
	}

	if client1 != client2 {
		t.Error("Expected same client instance from GetDefaultHTTPClient")
	}

	if client1.Timeout != DefaultClientConfig.Timeout {
		t.Errorf("Expected timeout %v, got %v", DefaultClientConfig.Timeout, client1.Timeout)
	}
}

func TestGetClientWithTimeout(t *testing.T) {
	timeout := 45 * time.Second
	client := GetClientWithTimeout(timeout)

	if client == nil {
		t.Fatal("GetClientWithTimeout returned nil")
	}

	if client.Timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, client.Timeout)
	}
}

func TestGetInsecureClient(t *testing.T) {
	client := GetInsecureClient()

	if client == nil {
		t.Fatal("GetInsecureClient returned nil")
	}

	// Check if TLS verification is disabled
	if transport, ok := client.Transport.(*http.Transport); ok {
		if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
			t.Error("Expected TLS verification to be disabled")
		}
	} else {
		t.Error("Expected http.Transport")
	}
}

func TestGetClientWithTimeoutAndTLS(t *testing.T) {
	timeout := 25 * time.Second
	skipTLS := true

	client := GetClientWithTimeoutAndTLS(timeout, skipTLS)

	if client == nil {
		t.Fatal("GetClientWithTimeoutAndTLS returned nil")
	}

	if client.Timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, client.Timeout)
	}

	if transport, ok := client.Transport.(*http.Transport); ok {
		if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
			t.Error("Expected TLS verification to be disabled")
		}
	} else {
		t.Error("Expected http.Transport")
	}
}

func TestHTTPClientPool_Close(t *testing.T) {
	pool := &HTTPClientPool{}

	config := DefaultClientConfig
	client1 := pool.GetClient(config)

	config.Timeout = 60 * time.Second
	client2 := pool.GetClient(config)

	// Ensure we have clients in the pool
	if client1 == nil || client2 == nil {
		t.Fatal("Failed to get clients from pool")
	}

	// Close the pool
	pool.Close()

	// After close, getting clients with same config should create new instances
	client3 := pool.GetClient(DefaultClientConfig)
	if client1 == client3 {
		t.Error("Expected new client instance after pool close")
	}
}

func BenchmarkHTTPClientPool_GetClient(b *testing.B) {
	pool := &HTTPClientPool{}
	config := DefaultClientConfig

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.GetClient(config)
	}
}

func BenchmarkHTTPClientPool_GetClientDifferentConfigs(b *testing.B) {
	pool := &HTTPClientPool{}
	configs := []ClientConfig{
		{Timeout: 10 * time.Second, SkipTLSVerify: false, MaxIdleConns: 5, IdleConnTimeout: 30 * time.Second},
		{Timeout: 20 * time.Second, SkipTLSVerify: false, MaxIdleConns: 10, IdleConnTimeout: 60 * time.Second},
		{Timeout: 30 * time.Second, SkipTLSVerify: true, MaxIdleConns: 15, IdleConnTimeout: 90 * time.Second},
		{Timeout: 40 * time.Second, SkipTLSVerify: false, MaxIdleConns: 20, IdleConnTimeout: 120 * time.Second},
		{Timeout: 50 * time.Second, SkipTLSVerify: true, MaxIdleConns: 25, IdleConnTimeout: 150 * time.Second},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := configs[i%len(configs)]
		_ = pool.GetClient(config)
	}
}

func BenchmarkNewHTTPClientEveryTime(b *testing.B) {
	// Benchmark creating a new HTTP client every time (old approach)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
}

func TestHTTPClientPool_ConcurrentAccess(t *testing.T) {
	pool := &HTTPClientPool{}
	config := DefaultClientConfig

	const numGoroutines = 100
	const numRequestsPerGoroutine = 10

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines that access the pool concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numRequestsPerGoroutine; j++ {
				client := pool.GetClient(config)
				if client == nil {
					t.Error("GetClient returned nil")
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
