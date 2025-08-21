package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

func TestNewClient(t *testing.T) {
	config := Config{
		BaseURL:       "https://api.openai.com/v1",
		APIKey:        "test-key",
		Model:         "gpt-3.5-turbo",
		MaxTokens:     1000,
		Temperature:   0.7,
		SkipTLSVerify: false,
	}

	client := NewClient(config)

	if client.baseURL != config.BaseURL {
		t.Errorf("Expected baseURL %s, got %s", config.BaseURL, client.baseURL)
	}
	if client.apiKey != config.APIKey {
		t.Errorf("Expected apiKey %s, got %s", config.APIKey, client.apiKey)
	}
	if client.model != config.Model {
		t.Errorf("Expected model %s, got %s", config.Model, client.model)
	}
}

func TestFormatTimelineForLLM(t *testing.T) {
	// Create test timeline
	date := time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC)
	tl := timeline.NewTimeline(date)

	// Add test activities
	duration := 30 * time.Minute
	activities := []timeline.Activity{
		{
			ID:          "1",
			Type:        timeline.ActivityTypeGitCommit,
			Title:       "Fix bug in user authentication",
			Description: "Updated password validation logic",
			Timestamp:   time.Date(2023, 12, 25, 9, 30, 0, 0, time.UTC),
			Duration:    &duration,
			Source:      "github",
			URL:         "https://github.com/user/repo/commit/abc123",
			Metadata: map[string]string{
				"repository": "user/repo",
				"branch":     "main",
			},
		},
		{
			ID:        "2",
			Type:      timeline.ActivityTypeCalendar,
			Title:     "Team standup meeting",
			Timestamp: time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC),
			Source:    "calendar",
		},
	}

	tl.AddActivities(activities)

	client := &Client{}
	formatted := client.formatTimelineForLLM(tl)

	// Check that the formatted text contains expected elements
	expectedElements := []string{
		"Date: Monday, December 25, 2023",
		"Total Activities: 2",
		"Fix bug in user authentication",
		"Team standup meeting",
		"09:30",
		"10:00",
	}

	for _, element := range expectedElements {
		if !strings.Contains(formatted, element) {
			t.Errorf("Expected formatted text to contain '%s', but it didn't. Got: %s", element, formatted)
		}
	}
}

func TestFormatTimelineForLLM_EmptyTimeline(t *testing.T) {
	date := time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC)
	tl := timeline.NewTimeline(date)

	client := &Client{}
	formatted := client.formatTimelineForLLM(tl)

	expected := "No activities found for this date."
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}
}

func TestSendChatCompletionRequest_Success(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Expected Bearer token in Authorization header, got %s", r.Header.Get("Authorization"))
		}

		// Mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-3.5-turbo",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "This is a test response from the AI."
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 20,
				"total_tokens": 30
			}
		}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		model:      "gpt-3.5-turbo",
		httpClient: &http.Client{},
	}

	req := ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	ctx := context.Background()
	response, err := client.sendChatCompletionRequest(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.ID != "chatcmpl-123" {
		t.Errorf("Expected ID 'chatcmpl-123', got '%s'", response.ID)
	}

	if len(response.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(response.Choices))
	}

	if response.Choices[0].Message.Content != "This is a test response from the AI." {
		t.Errorf("Unexpected response content: %s", response.Choices[0].Message.Content)
	}
}

func TestSendChatCompletionRequest_HTTPError(t *testing.T) {
	// Mock HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "invalid-key",
		model:      "gpt-3.5-turbo",
		httpClient: &http.Client{},
	}

	req := ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := client.sendChatCompletionRequest(ctx, req)

	if err == nil {
		t.Fatal("Expected error for HTTP 401, got nil")
	}

	if !strings.Contains(err.Error(), "API request failed with status 401") {
		t.Errorf("Expected error message about status 401, got: %v", err)
	}
}

func TestAnalyzeTimeline_Integration(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-3.5-turbo",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Based on your timeline, you had a productive day with good focus on development tasks. I notice you started early with commits and had regular meetings. Consider blocking more focused coding time."
				},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()

	config := Config{
		BaseURL:       server.URL,
		APIKey:        "test-key",
		Model:         "gpt-3.5-turbo",
		MaxTokens:     1000,
		Temperature:   0.7,
		SkipTLSVerify: false,
	}

	client := NewClient(config)

	// Create test timeline
	date := time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC)
	tl := timeline.NewTimeline(date)
	tl.AddActivity(timeline.Activity{
		ID:        "1",
		Type:      timeline.ActivityTypeGitCommit,
		Title:     "Implement new feature",
		Timestamp: time.Date(2023, 12, 25, 9, 0, 0, 0, time.UTC),
		Source:    "github",
	})

	ctx := context.Background()
	result, err := client.AnalyzeTimeline(ctx, tl, "Analyze my productivity", config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedContent := "Based on your timeline, you had a productive day"
	if !strings.Contains(result, expectedContent) {
		t.Errorf("Expected result to contain '%s', got: %s", expectedContent, result)
	}
}

func TestTestConnection_Success(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-3.5-turbo",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Connection successful"
				},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		model:      "gpt-3.5-turbo",
		httpClient: &http.Client{},
	}

	ctx := context.Background()
	err := client.TestConnection(ctx)

	if err != nil {
		t.Fatalf("Expected successful connection test, got error: %v", err)
	}
}

func TestTestConnection_Failure(t *testing.T) {
	// Mock HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "invalid-key",
		model:      "gpt-3.5-turbo",
		httpClient: &http.Client{},
	}

	ctx := context.Background()
	err := client.TestConnection(ctx)

	if err == nil {
		t.Fatal("Expected connection test to fail, got nil error")
	}

	if !strings.Contains(err.Error(), "connection test failed") {
		t.Errorf("Expected error message about connection test failure, got: %v", err)
	}
}

func TestNewClient_SkipTLSVerify(t *testing.T) {
	config := Config{
		BaseURL:       "https://api.openai.com/v1",
		APIKey:        "test-key",
		Model:         "gpt-3.5-turbo",
		MaxTokens:     1000,
		Temperature:   0.7,
		SkipTLSVerify: true,
	}

	client := NewClient(config)

	// Check that TLS verification is disabled
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected custom transport to be set when SkipTLSVerify is true")
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLS config to be set when SkipTLSVerify is true")
	}

	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true when SkipTLSVerify is true")
	}
}

func TestNewClient_DefaultTLSVerify(t *testing.T) {
	config := Config{
		BaseURL:       "https://api.openai.com/v1",
		APIKey:        "test-key",
		Model:         "gpt-3.5-turbo",
		MaxTokens:     1000,
		Temperature:   0.7,
		SkipTLSVerify: false,
	}

	client := NewClient(config)

	// Check that default transport is used when SkipTLSVerify is false
	if client.httpClient.Transport != nil {
		t.Error("Expected default transport when SkipTLSVerify is false")
	}
}
