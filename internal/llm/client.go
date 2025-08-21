package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// Client represents an OpenAI-compatible LLM client
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// Config holds configuration for the LLM client
type Config struct {
	BaseURL       string  `yaml:"base_url"`
	APIKey        string  `yaml:"api_key"`
	Model         string  `yaml:"model"`
	MaxTokens     int     `yaml:"max_tokens"`
	Temperature   float64 `yaml:"temperature"`
	SkipTLSVerify bool    `yaml:"skip_tls_verify"`
}

// ChatMessage represents a message in the chat completion
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents the request to the chat completion API
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

// ChatCompletionResponse represents the response from the chat completion API
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewClient creates a new LLM client
func NewClient(config Config) *Client {
	// Configure HTTP client with optional TLS verification skip
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	if config.SkipTLSVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return &Client{
		baseURL:    config.BaseURL,
		apiKey:     config.APIKey,
		model:      config.Model,
		httpClient: httpClient,
	}
}

// AnalyzeTimeline sends a timeline to the LLM for analysis
func (c *Client) AnalyzeTimeline(ctx context.Context, tl *timeline.Timeline, prompt string, config Config) (string, error) {
	// Format timeline for LLM consumption
	timelineText := c.formatTimelineForLLM(tl)

	// Create the full prompt
	systemPrompt := `You are an AI assistant that converts daily activity timelines into timesheets.
You will receive a chronological list of activities from various sources (GitHub commits, calendar events, etc.).
You need to convert the timeline data into a list of timesheets by grouping similar activities together.
`

	userPrompt := `
Provide me a list of timesheets based on the timeline data one timesheet per line.
Rules:
- IMPORTANT: you MUST group all timeline data related to the same issue key into a single timesheet.
- When issue tracker keys are present, they need to be part of the timesheet message.
- Timesheets have a minimal duration of 0.25h and a maximum duration of 8h. if they are longer than 8h, discard them. if they are shorter than 0.25h, discard them or group them with the next timesheet.
- Present the timesheets in a list, one timesheet per line and for each timesheet: Date (formatted as YYYY-MM-DD), duration, issue key if available, message. Do not include the field name.
- Ensure each timesheet is self-contained and includes all relevant details.
- Use clear and concise language.

`

	fullUserPrompt := fmt.Sprintf("%s\n\nTimeline Data:\n%s", userPrompt, timelineText)

	// Create chat completion request
	req := ChatCompletionRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fullUserPrompt},
		},
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
		Stream:      false,
	}

	// Send request
	response, err := c.sendChatCompletionRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to LLM: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from LLM")
	}

	return response.Choices[0].Message.Content, nil
}

// sendChatCompletionRequest sends a chat completion request to the LLM API
func (c *Client) sendChatCompletionRequest(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Marshal request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		// Try to determine if response is HTML or JSON for better error messages
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(strings.ToLower(contentType), "html") || strings.HasPrefix(string(body), "<") {
			return nil, fmt.Errorf("API request failed with status %d. Server returned HTML instead of JSON. This usually means:\n- Incorrect API endpoint URL\n- Authentication failed\n- Server error\nResponse body (first 200 chars): %s", resp.StatusCode, truncateString(string(body), 200))
		}
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check if response looks like JSON before parsing
	bodyStr := string(body)
	if !strings.HasPrefix(strings.TrimSpace(bodyStr), "{") {
		return nil, fmt.Errorf("API returned non-JSON response. This usually means:\n- Incorrect API endpoint URL (check base_url)\n- Server returned HTML error page\n- Authentication failed\nContent-Type: %s\nResponse body (first 200 chars): %s",
			resp.Header.Get("Content-Type"), truncateString(bodyStr, 200))
	}

	// Parse response
	var response ChatCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w\nResponse body (first 200 chars): %s", err, truncateString(bodyStr, 200))
	}

	return &response, nil
}

// SendDebugRequest sends a debug request and returns the raw response for troubleshooting
func (c *Client) SendDebugRequest(ctx context.Context, req ChatCompletionRequest) (string, error) {
	// Marshal request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Return raw response for debugging
	return fmt.Sprintf("Status: %d %s\nHeaders: %v\nBody: %s",
		resp.StatusCode, resp.Status, resp.Header, string(body)), nil
}

// formatTimelineForLLM formats a timeline in a readable format for the LLM
func (c *Client) formatTimelineForLLM(tl *timeline.Timeline) string {
	if len(tl.Activities) == 0 {
		return "No activities found for this date."
	}

	var result bytes.Buffer

	result.WriteString(fmt.Sprintf("Date: %s\n", tl.Date.Format("Monday, January 2, 2006")))
	result.WriteString(fmt.Sprintf("Total Activities: %d\n\n", len(tl.Activities)))

	result.WriteString("Activities:\n")
	result.WriteString("=====================================\n\n")

	for i, activity := range tl.Activities {
		result.WriteString(fmt.Sprintf("%d. [%s] %s\n",
			i+1,
			activity.Timestamp.Format("15:04"),
			activity.Title))
		//if activity.Description != "" {
		//	result.WriteString(fmt.Sprintf("   Description: %s\n", activity.Description))
		//}

		if activity.Duration != nil {
			result.WriteString(fmt.Sprintf("   Duration: %s\n", activity.FormatDuration()))
		}

		result.WriteString("\n")
	}

	return result.String()
}

// TestConnection tests the connection to the LLM API
func (c *Client) TestConnection(ctx context.Context) error {
	// Create a simple test request
	req := ChatCompletionRequest{
		Model: c.model,
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello, this is a test message. Please respond with 'Connection successful'."},
		},
		MaxTokens:   50,
		Temperature: 0,
		Stream:      false,
	}

	_, err := c.sendChatCompletionRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	return nil
}

// truncateString truncates a string to maxLen characters and adds "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
