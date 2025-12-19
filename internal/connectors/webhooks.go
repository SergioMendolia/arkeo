package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
	"github.com/arkeo/arkeo/internal/utils"
)

// WebhooksConnector implements the Connector interface for webhook endpoints
type WebhooksConnector struct {
	*BaseConnector
}

// WebhookConfig represents a single webhook configuration
type WebhookConfig struct {
	Name                string `json:"name"`                  // Display name for activities from this webhook
	URL                 string `json:"url"`                   // Webhook endpoint URL
	Token               string `json:"token"`                 // Bearer token for authentication
	SkipTLSVerification bool   `json:"skip_tls_verification"` // Skip TLS certificate verification
}

// WebhookActivity represents an activity returned by a webhook
type WebhookActivity struct {
	Timestamp   string                 `json:"timestamp"`   // RFC3339 format timestamp
	Title       string                 `json:"title"`       // Activity title
	Description string                 `json:"description"` // Optional description
	Type        string                 `json:"type"`        // Activity type (optional, defaults to "webhook")
	Metadata    map[string]interface{} `json:"metadata"`    // Optional metadata
}

// NewWebhooksConnector creates a new webhooks connector
func NewWebhooksConnector() *WebhooksConnector {
	return &WebhooksConnector{
		BaseConnector: NewBaseConnector(
			"webhooks",
			"Fetches activities from webhook endpoints with configurable authentication",
		),
	}
}

// GetRequiredConfig returns the required configuration for webhooks
func (w *WebhooksConnector) GetRequiredConfig() []ConfigField {
	requiredFields := []ConfigField{
		{
			Key:         "webhooks",
			Type:        "array",
			Required:    true,
			Description: "Array of webhook configurations (name, url, token)",
		},
	}

	return MergeConfigFields(requiredFields)
}

// ValidateConfig validates the webhooks configuration
func (w *WebhooksConnector) ValidateConfig(config map[string]interface{}) error {
	if err := ValidateConfigFields(config, w.GetRequiredConfig()); err != nil {
		return err
	}

	// Validate webhooks array structure
	webhooksRaw, exists := config["webhooks"]
	if !exists {
		return fmt.Errorf("webhooks configuration is required")
	}

	webhooks, err := w.parseWebhooksConfig(webhooksRaw)
	if err != nil {
		return fmt.Errorf("invalid webhooks configuration: %w", err)
	}

	if len(webhooks) == 0 {
		return fmt.Errorf("at least one webhook must be configured")
	}

	// Validate each webhook configuration
	for i, webhook := range webhooks {
		if webhook.Name == "" {
			return fmt.Errorf("webhook %d: name is required", i)
		}
		if webhook.URL == "" {
			return fmt.Errorf("webhook %d (%s): url is required", i, webhook.Name)
		}
		if webhook.Token == "" {
			return fmt.Errorf("webhook %d (%s): token is required", i, webhook.Name)
		}

		// Validate URL format
		if _, err := url.Parse(webhook.URL); err != nil {
			return fmt.Errorf("webhook %d (%s): invalid URL format: %w", i, webhook.Name, err)
		}
	}

	return nil
}

// TestConnection tests the webhook connections
func (w *WebhooksConnector) TestConnection(ctx context.Context) error {
	webhooks, err := w.getWebhookConfigs()
	if err != nil {
		return fmt.Errorf("failed to get webhook configs: %w", err)
	}

	for _, webhook := range webhooks {
		if err := w.testSingleWebhook(ctx, webhook); err != nil {
			return fmt.Errorf("webhook '%s' failed: %w", webhook.Name, err)
		}
	}

	return nil
}

// GetActivities retrieves activities from all configured webhooks for the specified date
func (w *WebhooksConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	webhooks, err := w.getWebhookConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook configs: %w", err)
	}

	var allActivities []timeline.Activity
	dateStr := date.Format("2006-01-02")

	for _, webhook := range webhooks {
		activities, err := w.fetchActivitiesFromWebhook(ctx, webhook, dateStr)
		if err != nil {
			if w.IsDebugMode() {
				fmt.Printf("Warning: webhook '%s' failed: %v\n", webhook.Name, err)
			}
			continue // Continue with other webhooks even if one fails
		}

		allActivities = append(allActivities, activities...)
	}

	return allActivities, nil
}

// getWebhookConfigs parses and returns the webhook configurations
func (w *WebhooksConnector) getWebhookConfigs() ([]WebhookConfig, error) {
	webhooksRaw := w.GetConfig()["webhooks"]
	if webhooksRaw == nil {
		return nil, fmt.Errorf("no webhooks configured")
	}

	return w.parseWebhooksConfig(webhooksRaw)
}

// parseWebhooksConfig parses the webhooks configuration from various input formats
func (w *WebhooksConnector) parseWebhooksConfig(webhooksRaw interface{}) ([]WebhookConfig, error) {
	var webhooks []WebhookConfig

	switch v := webhooksRaw.(type) {
	case []interface{}:
		// Array of webhook configs
		for i, item := range v {
			webhook, err := w.parseWebhookItem(item)
			if err != nil {
				return nil, fmt.Errorf("webhook %d: %w", i, err)
			}
			webhooks = append(webhooks, webhook)
		}
	case []WebhookConfig:
		// Already parsed
		webhooks = v
	default:
		return nil, fmt.Errorf("webhooks must be an array, got %T", webhooksRaw)
	}

	return webhooks, nil
}

// parseWebhookItem parses a single webhook configuration item
func (w *WebhooksConnector) parseWebhookItem(item interface{}) (WebhookConfig, error) {
	var webhook WebhookConfig

	switch v := item.(type) {
	case map[string]interface{}:
		// Extract fields from map
		if name, ok := v["name"].(string); ok {
			webhook.Name = name
		}
		if url, ok := v["url"].(string); ok {
			webhook.URL = url
		}
		if token, ok := v["token"].(string); ok {
			webhook.Token = token
		}
		if skipTLS, ok := v["skip_tls_verification"].(bool); ok {
			webhook.SkipTLSVerification = skipTLS
		}
	case map[interface{}]interface{}:
		// Handle YAML-style map with interface{} keys
		if name, ok := v["name"].(string); ok {
			webhook.Name = name
		}
		if url, ok := v["url"].(string); ok {
			webhook.URL = url
		}
		if token, ok := v["token"].(string); ok {
			webhook.Token = token
		}
		if skipTLS, ok := v["skip_tls_verification"].(bool); ok {
			webhook.SkipTLSVerification = skipTLS
		}
	case WebhookConfig:
		webhook = v
	default:
		return webhook, fmt.Errorf("invalid webhook config format: %T", item)
	}

	return webhook, nil
}

// getHTTPClientForWebhook returns an appropriate HTTP client based on webhook configuration
func (w *WebhooksConnector) getHTTPClientForWebhook(webhook WebhookConfig) *http.Client {
	if webhook.SkipTLSVerification {
		// Use insecure client that skips TLS verification
		config := utils.ClientConfig{
			Timeout:         30 * time.Second,
			SkipTLSVerify:   true,
			MaxIdleConns:    10,
			IdleConnTimeout: 90 * time.Second,
		}
		return utils.GetHTTPClient(config)
	}

	// Use default secure client
	return utils.GetDefaultHTTPClient()
}

// testSingleWebhook tests a single webhook connection
func (w *WebhooksConnector) testSingleWebhook(ctx context.Context, webhook WebhookConfig) error {
	// Use a recent date for testing (yesterday)
	testDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	reqURL := fmt.Sprintf("%s?date=%s", webhook.URL, testDate)
	req, err := w.CreateBearerRequest(ctx, "GET", reqURL, webhook.Token)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	// Use webhook-specific HTTP client (supports TLS skip verification)
	httpClient := w.getHTTPClientForWebhook(webhook)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// fetchActivitiesFromWebhook fetches activities from a single webhook
func (w *WebhooksConnector) fetchActivitiesFromWebhook(ctx context.Context, webhook WebhookConfig, dateStr string) ([]timeline.Activity, error) {
	reqURL := fmt.Sprintf("%s?date=%s", webhook.URL, dateStr)

	if w.IsDebugMode() {
		fmt.Printf("Fetching activities from webhook: %s (%s)\n", webhook.Name, reqURL)
	}

	req, err := w.CreateBearerRequest(ctx, "GET", reqURL, webhook.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	// Use webhook-specific HTTP client (supports TLS skip verification)
	httpClient := w.getHTTPClientForWebhook(webhook)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var webhookActivities []WebhookActivity
	if err := json.Unmarshal(body, &webhookActivities); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Convert webhook activities to timeline activities
	var activities []timeline.Activity
	for _, wa := range webhookActivities {
		activity, err := w.convertWebhookActivity(wa, webhook.Name)
		if err != nil {
			if w.IsDebugMode() {
				fmt.Printf("Warning: skipping invalid activity: %v\n", err)
			}
			continue
		}
		activities = append(activities, activity)
	}

	if w.IsDebugMode() {
		fmt.Printf("Retrieved %d activities from webhook: %s\n", len(activities), webhook.Name)
	}

	return activities, nil
}

// convertWebhookActivity converts a WebhookActivity to a timeline.Activity
func (w *WebhooksConnector) convertWebhookActivity(wa WebhookActivity, webhookName string) (timeline.Activity, error) {
	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, wa.Timestamp)
	if err != nil {
		// Try alternative formats
		if timestamp, err = time.Parse("2006-01-02T15:04:05Z", wa.Timestamp); err != nil {
			if timestamp, err = time.Parse("2006-01-02 15:04:05", wa.Timestamp); err != nil {
				return timeline.Activity{}, fmt.Errorf("invalid timestamp format: %s", wa.Timestamp)
			}
		}
	}

	activityType := wa.Type
	if activityType == "" {
		activityType = "webhook"
	}

	title := wa.Title
	if title == "" {
		title = "Webhook Activity"
	}

	// Create the activity
	activity := timeline.Activity{
		Timestamp:   timestamp,
		Title:       title,
		Description: wa.Description,
		Type:        timeline.ActivityType(activityType),
		Source:      w.Name(),
		Metadata: map[string]string{
			"webhook_name": webhookName,
		},
	}

	// Merge additional metadata
	if wa.Metadata != nil {
		for k, v := range wa.Metadata {
			if strVal, ok := v.(string); ok {
				activity.Metadata[k] = strVal
			} else {
				activity.Metadata[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return activity, nil
}
