package connectors

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

func TestNewYouTrackConnector(t *testing.T) {
	connector := NewYouTrackConnector()

	if connector.Name() != "youtrack" {
		t.Errorf("Expected name 'youtrack', got %s", connector.Name())
	}

	if connector.Description() != "Fetches activities and issue updates from YouTrack" {
		t.Errorf("Unexpected description: %s", connector.Description())
	}

	if connector.IsEnabled() {
		t.Error("Connector should be disabled by default")
	}
}

func TestYouTrackConnector_GetRequiredConfig(t *testing.T) {
	connector := NewYouTrackConnector()
	config := connector.GetRequiredConfig()

	expectedFields := map[string]bool{
		"base_url":           true,  // required
		"token":              true,  // required
		"username":           false, // not required
		"include_work_items": false, // not required
		"include_comments":   false, // not required
		"include_issues":     false, // not required
		"api_fields":         false, // not required
		"log_level":          false, // not required
	}

	if len(config) != len(expectedFields) {
		t.Errorf("Expected %d config fields, got %d", len(expectedFields), len(config))
	}

	for _, field := range config {
		expectedRequired, exists := expectedFields[field.Key]
		if !exists {
			t.Errorf("Unexpected config field: %s", field.Key)
		}
		if field.Required != expectedRequired {
			t.Errorf("Field %s: expected required=%t, got %t", field.Key, expectedRequired, field.Required)
		}
	}
}

func TestYouTrackConnector_ValidateConfig(t *testing.T) {
	connector := NewYouTrackConnector()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      map[string]interface{}{"base_url": "https://example.youtrack.cloud/", "token": "perm:test123"},
			expectError: false,
		},
		{
			name:        "missing base_url",
			config:      map[string]interface{}{"token": "perm:test123"},
			expectError: true,
			errorMsg:    "youtrack base_url is required",
		},
		{
			name:        "missing token",
			config:      map[string]interface{}{"base_url": "https://example.youtrack.cloud/"},
			expectError: true,
			errorMsg:    "youtrack token is required",
		},
		{
			name:        "invalid base_url format",
			config:      map[string]interface{}{"base_url": "invalid-url", "token": "perm:test123"},
			expectError: true,
			errorMsg:    "youtrack base_url must start with http:// or https://",
		},
		{
			name:        "base_url without trailing slash",
			config:      map[string]interface{}{"base_url": "https://example.youtrack.cloud", "token": "perm:test123"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connector.ValidateConfig(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestYouTrackConnector_ConvertActivity(t *testing.T) {
	connector := NewYouTrackConnector()
	connector.Configure(map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	})

	// Test custom field activity
	ytActivity := youTrackActivity{
		ID:        "activity-123",
		Timestamp: time.Now().Unix() * 1000,
		Author: struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		}{
			Login: "testuser",
			Name:  "Test User",
		},
		Category: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "CustomFieldCategory",
			Name: "Custom Field",
		},
		Target: map[string]interface{}{
			"id":         "1-123",
			"idReadable": "TEST-123",
			"summary":    "Test Issue",
			"project": map[string]interface{}{
				"name":      "Test Project",
				"shortName": "TEST",
			},
		},
		Field: &struct {
			Name string `json:"name"`
		}{
			Name: "State",
		},
	}

	activity := connector.convertActivity(ytActivity)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	if activity.ID != "youtrack-activity-123" {
		t.Errorf("Expected ID 'youtrack-activity-123', got '%s'", activity.ID)
	}

	if activity.Type != timeline.ActivityTypeYouTrack {
		t.Errorf("Expected type ActivityTypeYouTrack, got %s", activity.Type)
	}

	if activity.Source != "youtrack" {
		t.Errorf("Expected source 'youtrack', got '%s'", activity.Source)
	}

	if activity.URL != "https://example.youtrack.cloud/issue/TEST-123" {
		t.Errorf("Expected URL 'https://example.youtrack.cloud/issue/TEST-123', got '%s'", activity.URL)
	}

	// Check metadata
	if activity.Metadata["category"] != "CustomFieldCategory" {
		t.Errorf("Expected category 'CustomFieldCategory', got '%s'", activity.Metadata["category"])
	}

	if activity.Metadata["author"] != "testuser" {
		t.Errorf("Expected author 'testuser', got '%s'", activity.Metadata["author"])
	}

	if activity.Metadata["issue_id"] != "1-123" {
		t.Errorf("Expected issue_id '1-123', got '%s'", activity.Metadata["issue_id"])
	}

	if activity.Metadata["issue_key"] != "TEST-123" {
		t.Errorf("Expected issue_key 'TEST-123', got '%s'", activity.Metadata["issue_key"])
	}
}

func TestYouTrackConnector_ExtractFieldValue(t *testing.T) {
	connector := NewYouTrackConnector()

	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "string value",
			value:    "In Progress",
			expected: "In Progress",
		},
		{
			name:     "nil value",
			value:    nil,
			expected: "",
		},
		{
			name: "object with name",
			value: map[string]interface{}{
				"name": "Open",
				"id":   "state-1",
			},
			expected: "Open",
		},
		{
			name: "object with login",
			value: map[string]interface{}{
				"login": "john.doe",
				"name":  "John Doe",
			},
			expected: "John Doe",
		},
		{
			name: "object with only login",
			value: map[string]interface{}{
				"login": "john.doe",
				"id":    "user-1",
			},
			expected: "john.doe",
		},
		{
			name: "object with presentation",
			value: map[string]interface{}{
				"presentation": "High Priority",
				"id":           "priority-1",
			},
			expected: "High Priority",
		},
		{
			name: "array of strings",
			value: []interface{}{
				"tag1",
				"tag2",
				"tag3",
			},
			expected: "tag1, tag2, tag3",
		},
		{
			name: "array of objects",
			value: []interface{}{
				map[string]interface{}{"name": "Backend"},
				map[string]interface{}{"name": "Frontend"},
			},
			expected: "Backend, Frontend",
		},
		{
			name:     "empty array",
			value:    []interface{}{},
			expected: "",
		},
		{
			name:     "number value",
			value:    42,
			expected: "42",
		},
		{
			name: "object with $type (YouTrack User)",
			value: map[string]interface{}{
				"$type":    "User",
				"login":    "john.doe",
				"fullName": "John Doe",
				"id":       "user-123",
			},
			expected: "John Doe",
		},
		{
			name: "object with $type (YouTrack State)",
			value: map[string]interface{}{
				"$type": "StateBundleElement",
				"name":  "In Progress",
				"id":    "state-2",
			},
			expected: "In Progress",
		},
		{
			name: "object with only $type",
			value: map[string]interface{}{
				"$type": "User",
				"id":    "user-123",
			},
			expected: "",
		},
		{
			name: "realistic YouTrack User assignment",
			value: map[string]interface{}{
				"$type":    "User",
				"login":    "john.doe",
				"fullName": "John Doe",
				"id":       "1-1",
				"name":     "John Doe",
			},
			expected: "John Doe",
		},
		{
			name: "realistic YouTrack State change",
			value: map[string]interface{}{
				"$type":       "StateBundleElement",
				"name":        "In Progress",
				"id":          "1-5",
				"description": "Work is in progress",
			},
			expected: "In Progress",
		},
		{
			name: "YouTrack object with only type info",
			value: map[string]interface{}{
				"$type": "StateBundleElement",
				"id":    "1-5",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := connector.extractFieldValue(tt.value)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestYouTrackConnector_ConvertActivityWithFieldValues(t *testing.T) {
	connector := NewYouTrackConnector()
	connector.Configure(map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	})

	tests := []struct {
		name                string
		added               interface{}
		removed             interface{}
		fieldName           string
		expectedTitle       string
		expectedDescription string
	}{
		{
			name:                "string field change",
			added:               "In Progress",
			removed:             "Open",
			fieldName:           "State",
			expectedTitle:       "Updated State to In Progress in TEST-123",
			expectedDescription: "Changed State from Open to In Progress",
		},
		{
			name:                "object field change",
			added:               map[string]interface{}{"name": "High", "id": "priority-1"},
			removed:             map[string]interface{}{"name": "Normal", "id": "priority-2"},
			fieldName:           "Priority",
			expectedTitle:       "Updated Priority to High in TEST-123",
			expectedDescription: "Changed Priority from Normal to High",
		},
		{
			name:                "field set (no old value)",
			added:               "john.doe",
			removed:             nil,
			fieldName:           "Assignee",
			expectedTitle:       "Updated Assignee to john.doe in TEST-123",
			expectedDescription: "Set Assignee to john.doe",
		},
		{
			name:                "field cleared (no new value)",
			added:               nil,
			removed:             "jane.doe",
			fieldName:           "Assignee",
			expectedTitle:       "Updated Assignee in TEST-123",
			expectedDescription: "Cleared Assignee (was jane.doe)",
		},
		{
			name:                "no values available",
			added:               nil,
			removed:             nil,
			fieldName:           "Custom Field",
			expectedTitle:       "Updated Custom Field in TEST-123",
			expectedDescription: "Modified Custom Field",
		},
		{
			name:                "array field change",
			added:               []interface{}{"backend", "api"},
			removed:             []interface{}{"frontend"},
			fieldName:           "Tags",
			expectedTitle:       "Updated Tags to backend, api in TEST-123",
			expectedDescription: "Changed Tags from frontend to backend, api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ytActivity := youTrackActivity{
				ID:        "activity-123",
				Timestamp: time.Now().Unix() * 1000,
				Author: struct {
					Login string `json:"login"`
					Name  string `json:"name"`
				}{
					Login: "testuser",
					Name:  "Test User",
				},
				Category: struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					ID:   "CustomFieldCategory",
					Name: "Custom Field",
				},
				Target: map[string]interface{}{
					"id":         "1-123",
					"idReadable": "TEST-123",
					"summary":    "Test Issue",
					"project": map[string]interface{}{
						"name":      "Test Project",
						"shortName": "TEST",
					},
				},
				Field: &struct {
					Name string `json:"name"`
				}{
					Name: tt.fieldName,
				},
				Added:   tt.added,
				Removed: tt.removed,
			}

			activity := connector.convertActivity(ytActivity)

			if activity == nil {
				t.Fatal("Expected activity, got nil")
			}

			if activity.Title != tt.expectedTitle {
				t.Errorf("Expected title '%s', got '%s'", tt.expectedTitle, activity.Title)
			}

			if activity.Description != tt.expectedDescription {
				t.Errorf("Expected description '%s', got '%s'", tt.expectedDescription, activity.Description)
			}

			// Check that field metadata is properly set
			if activity.Metadata["field_name"] != tt.fieldName {
				t.Errorf("Expected field_name '%s', got '%s'", tt.fieldName, activity.Metadata["field_name"])
			}

			// Check field values in metadata
			if tt.added != nil {
				expectedNewValue := connector.extractFieldValue(tt.added)
				if activity.Metadata["field_new_value"] != expectedNewValue {
					t.Errorf("Expected field_new_value '%s', got '%s'", expectedNewValue, activity.Metadata["field_new_value"])
				}
			} else {
				if _, exists := activity.Metadata["field_new_value"]; exists {
					t.Error("Expected field_new_value to not be set when added is nil")
				}
			}

			if tt.removed != nil {
				expectedOldValue := connector.extractFieldValue(tt.removed)
				if activity.Metadata["field_old_value"] != expectedOldValue {
					t.Errorf("Expected field_old_value '%s', got '%s'", expectedOldValue, activity.Metadata["field_old_value"])
				}
			} else {
				if _, exists := activity.Metadata["field_old_value"]; exists {
					t.Error("Expected field_old_value to not be set when removed is nil")
				}
			}
		})
	}
}

func TestYouTrackConnector_ConvertActivity_CommentCategory(t *testing.T) {
	connector := NewYouTrackConnector()
	connector.Configure(map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	})

	ytActivity := youTrackActivity{
		ID:        "comment-456",
		Timestamp: time.Now().Unix() * 1000,
		Author: struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		}{
			Login: "testuser",
			Name:  "Test User",
		},
		Category: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "CommentsCategory",
			Name: "Comments",
		},
		Target: map[string]interface{}{
			"id":   "comment-789",
			"text": "This is a comment",
			"issue": map[string]interface{}{
				"id":         "1-456",
				"idReadable": "TEST-456",
				"summary":    "Another Test Issue",
				"project": map[string]interface{}{
					"name":      "Test Project",
					"shortName": "TEST",
				},
			},
		},
	}

	activity := connector.convertActivity(ytActivity)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	if activity.Title != "Commented on TEST-456" {
		t.Errorf("Expected title 'Commented on TEST-456', got '%s'", activity.Title)
	}

	if activity.Description != "Added a comment" {
		t.Errorf("Expected description 'Added a comment', got '%s'", activity.Description)
	}

}

func TestYouTrackConnector_TestConnection_MissingConfig(t *testing.T) {
	connector := NewYouTrackConnector()
	ctx := context.Background()

	err := connector.TestConnection(ctx)
	if err == nil {
		t.Error("Expected error for missing configuration")
	}

	expectedMsg := "base_url and token must be configured"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestYouTrackConnector_DebugMode(t *testing.T) {
	connector := NewYouTrackConnector()

	// Test default debug mode (should be false)
	if connector.isDebugMode() {
		t.Error("Debug mode should be disabled by default")
	}

	// Test debug mode enabled via config
	config := map[string]interface{}{
		"base_url":  "https://example.youtrack.cloud/",
		"token":     "perm:test123",
		"log_level": "debug",
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	if !connector.isDebugMode() {
		t.Error("Debug mode should be enabled when log_level is 'debug'")
	}

	// Test debug mode disabled with different log level
	config["log_level"] = "info"
	err = connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	if connector.isDebugMode() {
		t.Error("Debug mode should be disabled when log_level is 'info'")
	}

	// Test case insensitive debug mode
	config["log_level"] = "DEBUG"
	err = connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	if !connector.isDebugMode() {
		t.Error("Debug mode should be enabled when log_level is 'DEBUG' (case insensitive)")
	}
}

func TestYouTrackConnector_DebugModeEnvironmentVariables(t *testing.T) {
	connector := NewYouTrackConnector()

	// Configure without debug
	config := map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	// Should be disabled by default
	if connector.isDebugMode() {
		t.Error("Debug mode should be disabled by default")
	}

	// Test LOG_LEVEL environment variable
	t.Setenv("LOG_LEVEL", "debug")
	if !connector.isDebugMode() {
		t.Error("Debug mode should be enabled when LOG_LEVEL=debug")
	}

	// Test AUTOTIME_DEBUG environment variable
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("AUTOTIME_DEBUG", "1")
	if !connector.isDebugMode() {
		t.Error("Debug mode should be enabled when AUTOTIME_DEBUG=1")
	}

	// Test AUTOTIME_DEBUG with true
	t.Setenv("AUTOTIME_DEBUG", "true")
	if !connector.isDebugMode() {
		t.Error("Debug mode should be enabled when AUTOTIME_DEBUG=true")
	}

	// Test config override takes precedence
	config["log_level"] = "debug"
	err = connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("AUTOTIME_DEBUG", "false")
	if !connector.isDebugMode() {
		t.Error("Config log_level should override environment variables")
	}
}

func TestYouTrackConnector_APIFieldsConfiguration(t *testing.T) {
	connector := NewYouTrackConnector()

	// Test default API fields
	config := map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	// Should use default fields when none specified
	actualFields := connector.GetConfigString("api_fields")
	if actualFields != "" {
		t.Errorf("Expected empty api_fields (uses default), got '%s'", actualFields)
	}

	// Test custom API fields
	config["api_fields"] = "id,timestamp,author(login),category(id)"
	err = connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector with custom api_fields: %v", err)
	}

	customFields := connector.GetConfigString("api_fields")
	if customFields != "id,timestamp,author(login),category(id)" {
		t.Errorf("Expected custom api_fields 'id,timestamp,author(login),category(id)', got '%s'", customFields)
	}
}

func TestYouTrackConnector_URLValidation(t *testing.T) {
	connector := NewYouTrackConnector()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid youtrack cloud URL",
			config:      map[string]interface{}{"base_url": "https://mycompany.youtrack.cloud/", "token": "perm:test123"},
			expectError: false,
		},
		{
			name:        "valid self-hosted URL",
			config:      map[string]interface{}{"base_url": "https://youtrack.example.com/", "token": "perm:test123"},
			expectError: false,
		},
		{
			name:        "URL without protocol",
			config:      map[string]interface{}{"base_url": "mycompany.youtrack.cloud", "token": "perm:test123"},
			expectError: true,
			errorMsg:    "youtrack base_url must start with http:// or https://",
		},
		{
			name:        "malformed URL",
			config:      map[string]interface{}{"base_url": "ht!tp://invalid url", "token": "perm:test123"},
			expectError: true,
			errorMsg:    "youtrack base_url must start with http:// or https://",
		},
		{
			name:        "URL without host",
			config:      map[string]interface{}{"base_url": "https://", "token": "perm:test123"},
			expectError: true,
			errorMsg:    "YouTrack base_url must include a valid host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connector.ValidateConfig(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestYouTrackConnector_CategoryFallback(t *testing.T) {
	connector := NewYouTrackConnector()

	// Configure with all categories disabled
	config := map[string]interface{}{
		"base_url":           "https://example.youtrack.cloud/",
		"token":              "perm:test123",
		"include_work_items": false,
		"include_comments":   false,
		"include_issues":     false,
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	// Test that configuration is set correctly
	if connector.GetConfigBool("include_work_items") {
		t.Error("include_work_items should be false")
	}
	if connector.GetConfigBool("include_comments") {
		t.Error("include_comments should be false")
	}
	if connector.GetConfigBool("include_issues") {
		t.Error("include_issues should be false")
	}

	// The actual category fallback logic is tested in integration,
	// but we can verify the configuration is properly set
	t.Log("Category fallback logic will be tested when making API calls")
}

func TestYouTrackConnector_BackwardsCompatibility_MissingIdReadable(t *testing.T) {
	connector := NewYouTrackConnector()
	connector.Configure(map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	})

	// Test activity without idReadable field (older YouTrack versions)
	ytActivity := youTrackActivity{
		ID:        "activity-backwards-compat",
		Timestamp: time.Now().Unix() * 1000,
		Author: struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		}{
			Login: "testuser",
			Name:  "Test User",
		},
		Category: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "CustomFieldCategory",
			Name: "Custom Field",
		},
		Target: map[string]interface{}{
			"id":      "internal-456", // Only internal ID, no idReadable
			"summary": "Legacy Issue Format",
			"project": map[string]interface{}{
				"name":      "Legacy Project",
				"shortName": "LEG",
			},
		},
		Field: &struct {
			Name string `json:"name"`
		}{
			Name: "Priority",
		},
	}

	activity := connector.convertActivity(ytActivity)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Should use internal ID when idReadable is not available
	if activity.Title != "Updated Priority in internal-456" {
		t.Errorf("Expected title 'Updated Priority in internal-456', got '%s'", activity.Title)
	}

	if activity.URL != "https://example.youtrack.cloud/issue/internal-456" {
		t.Errorf("Expected URL with internal ID, got '%s'", activity.URL)
	}

	// Should store internal ID in metadata
	if activity.Metadata["issue_id"] != "internal-456" {
		t.Errorf("Expected issue_id 'internal-456', got '%s'", activity.Metadata["issue_id"])
	}

	// Should have empty issue_key since idReadable was not provided
	if activity.Metadata["issue_key"] != "" {
		t.Errorf("Expected empty issue_key, got '%s'", activity.Metadata["issue_key"])
	}
}

func TestYouTrackConnector_CommentActivityIssueKeyExtraction(t *testing.T) {
	connector := NewYouTrackConnector()
	connector.Configure(map[string]interface{}{
		"base_url": "https://example.youtrack.cloud/",
		"token":    "perm:test123",
	})

	// Test comment activity with issue key in target.issue
	ytActivity := youTrackActivity{
		ID:        "comment-activity-test",
		Timestamp: time.Now().Unix() * 1000,
		Author: struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		}{
			Login: "testuser",
			Name:  "Test User",
		},
		Category: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "CommentsCategory",
			Name: "Comments",
		},
		Target: map[string]interface{}{
			"id":   "comment-987",
			"text": "This is a test comment",
			"issue": map[string]interface{}{
				"id":         "2-654",
				"idReadable": "DEMO-321",
				"summary":    "Comment Test Issue",
				"project": map[string]interface{}{
					"name":      "Demo Project",
					"shortName": "DEMO",
				},
			},
		},
	}

	activity := connector.convertActivity(ytActivity)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Should use issue key from target.issue.idReadable
	if activity.Title != "Commented on DEMO-321" {
		t.Errorf("Expected title 'Commented on DEMO-321', got '%s'", activity.Title)
	}

	if activity.URL != "https://example.youtrack.cloud/issue/DEMO-321" {
		t.Errorf("Expected URL with issue key 'DEMO-321', got '%s'", activity.URL)
	}

	// Should store both internal ID and issue key in metadata
	if activity.Metadata["issue_id"] != "2-654" {
		t.Errorf("Expected issue_id '2-654', got '%s'", activity.Metadata["issue_id"])
	}

	if activity.Metadata["issue_key"] != "DEMO-321" {
		t.Errorf("Expected issue_key 'DEMO-321', got '%s'", activity.Metadata["issue_key"])
	}

	if activity.Metadata["issue_summary"] != "Comment Test Issue" {
		t.Errorf("Expected issue_summary 'Comment Test Issue', got '%s'", activity.Metadata["issue_summary"])
	}

	if activity.Metadata["project"] != "Demo Project" {
		t.Errorf("Expected project 'Demo Project', got '%s'", activity.Metadata["project"])
	}
}
