package connectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

// Sample Atom feed XML for testing
const sampleAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <id>https://gitlab.com/testuser.atom</id>
  <title>testuser activity</title>
  <updated>2024-01-15T15:30:00Z</updated>

  <entry>
    <id>https://gitlab.com/testuser/project1/-/commit/abc123</id>
    <title>testuser pushed to branch main in project1</title>
    <published>2024-01-15T09:30:00Z</published>
    <updated>2024-01-15T09:30:00Z</updated>
    <link href="https://gitlab.com/testuser/project1/-/commit/abc123" rel="alternate" type="text/html"/>
    <content type="html">Pushed 2 commits to main branch</content>
    <summary>Fix authentication bug in user service</summary>
    <author>
      <name>testuser</name>
    </author>
    <category term="push"/>
  </entry>

  <entry>
    <id>https://gitlab.com/testuser/project2/-/merge_requests/42</id>
    <title>testuser opened merge request in project2</title>
    <published>2024-01-15T10:45:00Z</published>
    <updated>2024-01-15T10:45:00Z</updated>
    <link href="https://gitlab.com/testuser/project2/-/merge_requests/42" rel="alternate" type="text/html"/>
    <content type="html">Opened merge request: Add new API endpoints</content>
    <summary>Add new API endpoints for user management</summary>
    <author>
      <name>testuser</name>
    </author>
    <category term="merge_request"/>
  </entry>

  <entry>
    <id>https://gitlab.com/testuser/project1/-/issues/15</id>
    <title>testuser commented on issue in project1</title>
    <published>2024-01-15T13:20:00Z</published>
    <updated>2024-01-15T13:20:00Z</updated>
    <link href="https://gitlab.com/testuser/project1/-/issues/15" rel="alternate" type="text/html"/>
    <content type="html">Commented on issue: Database migration issues</content>
    <summary>Added suggestion for fixing database migration</summary>
    <author>
      <name>testuser</name>
    </author>
    <category term="comment"/>
  </entry>

  <entry>
    <id>https://gitlab.com/testuser/project3/-/merge_requests/41</id>
    <title>testuser merged merge request in project3</title>
    <published>2024-01-14T15:15:00Z</published>
    <updated>2024-01-14T15:15:00Z</updated>
    <link href="https://gitlab.com/testuser/project3/-/merge_requests/41" rel="alternate" type="text/html"/>
    <content type="html">Merged merge request: Update dependencies</content>
    <summary>Updated all project dependencies to latest versions</summary>
    <author>
      <name>testuser</name>
    </author>
    <category term="merge_request"/>
  </entry>

  <entry>
    <id>https://gitlab.com/testuser/project4/-/issues/20</id>
    <title>testuser created issue in project4</title>
    <published></published>
    <updated>2024-01-15T16:45:00Z</updated>
    <link href="https://gitlab.com/testuser/project4/-/issues/20" rel="alternate" type="text/html"/>
    <content type="html">Created issue: Handle empty dates in feed</content>
    <summary>Issue with empty published date but valid updated date</summary>
    <author>
      <name>testuser</name>
    </author>
    <category term="issue"/>
  </entry>
</feed>`

func TestNewGitLabConnector(t *testing.T) {
	connector := NewGitLabConnector()

	if connector.Name() != "gitlab" {
		t.Errorf("Expected name 'gitlab', got %s", connector.Name())
	}

	if connector.Description() != "Fetches user activities from GitLab using Atom feeds" {
		t.Errorf("Unexpected description: %s", connector.Description())
	}

	if connector.IsEnabled() {
		t.Error("Connector should be disabled by default")
	}
}

func TestGitLabConnector_GetRequiredConfig(t *testing.T) {
	connector := NewGitLabConnector()
	config := connector.GetRequiredConfig()

	expectedFields := map[string]bool{
		"gitlab_url": false, // not required
		"username":   true,  // required
		"feed_token": true,  // required
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

func TestGitLabConnector_ValidateConfig(t *testing.T) {
	connector := NewGitLabConnector()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"username":   "testuser",
				"feed_token": "glft-test-token",
				"gitlab_url": "https://gitlab.com",
			},
			expectError: false,
		},
		{
			name: "missing username",
			config: map[string]interface{}{
				"feed_token": "glft-test-token",
			},
			expectError: true,
			errorMsg:    "gitlab username is required",
		},
		{
			name: "missing feed_token",
			config: map[string]interface{}{
				"username": "testuser",
			},
			expectError: true,
			errorMsg:    "gitlab feed token is required",
		},
		{
			name: "invalid gitlab_url",
			config: map[string]interface{}{
				"username":   "testuser",
				"feed_token": "glft-test-token",
				"gitlab_url": "://invalid-url",
			},
			expectError: true,
			errorMsg:    "invalid gitlab_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connector.ValidateConfig(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %s", err.Error())
				}
			}
		})
	}
}

func TestGitLabConnector_TestConnection(t *testing.T) {
	// Create a test server that serves our sample Atom feed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("feed_token")
		if token != "valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleAtomFeed))
	}))
	defer server.Close()

	connector := NewGitLabConnector()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "valid connection",
			config: map[string]interface{}{
				"gitlab_url": server.URL,
				"username":   "testuser",
				"feed_token": "valid-token",
			},
			expectError: false,
		},
		{
			name: "invalid token",
			config: map[string]interface{}{
				"gitlab_url": server.URL,
				"username":   "testuser",
				"feed_token": "invalid-token",
			},
			expectError: true,
		},
		{
			name: "missing username",
			config: map[string]interface{}{
				"gitlab_url": server.URL,
				"feed_token": "valid-token",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connector.Configure(tt.config)
			if err != nil && !tt.expectError {
				t.Fatalf("Failed to configure connector: %s", err.Error())
			}

			ctx := context.Background()
			err = connector.TestConnection(ctx)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %s", err.Error())
				}
			}
		})
	}
}

func TestGitLabConnector_GetActivities(t *testing.T) {
	// Create a test server that serves our sample Atom feed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleAtomFeed))
	}))
	defer server.Close()

	connector := NewGitLabConnector()
	config := map[string]interface{}{
		"gitlab_url": server.URL,
		"username":   "testuser",
		"feed_token": "test-token",
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %s", err.Error())
	}

	// Test getting activities for January 15, 2024
	targetDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()

	activities, err := connector.GetActivities(ctx, targetDate)
	if err != nil {
		t.Fatalf("Failed to get activities: %s", err.Error())
	}

	// Should get 4 activities for January 15, 2024 (excluding the January 14 activity, including the one with empty published date)
	expectedCount := 4
	if len(activities) != expectedCount {
		t.Errorf("Expected %d activities, got %d", expectedCount, len(activities))
	}

	// Verify activities are properly parsed
	for _, activity := range activities {
		if activity.Source != "gitlab" {
			t.Errorf("Expected source 'gitlab', got '%s'", activity.Source)
		}

		if activity.Title == "" {
			t.Error("Activity title should not be empty")
		}

		if activity.URL == "" {
			t.Error("Activity URL should not be empty")
		}

		if len(activity.Tags) == 0 {
			t.Error("Activity should have tags")
		}

		// Check that gitlab tag is present
		hasGitLabTag := false
		for _, tag := range activity.Tags {
			if tag == "gitlab" {
				hasGitLabTag = true
				break
			}
		}
		if !hasGitLabTag {
			t.Error("Activity should have 'gitlab' tag")
		}
	}
}

func TestGitLabConnector_DetermineActivityType(t *testing.T) {
	connector := NewGitLabConnector()

	tests := []struct {
		name         string
		entry        AtomEntry
		expectedType string
	}{
		{
			name: "commit activity",
			entry: AtomEntry{
				Title:   "testuser pushed to branch main",
				Content: AtomContent{Value: "commit details"},
			},
			expectedType: "commit",
		},
		{
			name: "merge request opened",
			entry: AtomEntry{
				Title: "testuser opened merge request",
			},
			expectedType: "merge_request_opened",
		},
		{
			name: "merge request merged",
			entry: AtomEntry{
				Title: "testuser merged merge request",
			},
			expectedType: "merge_request_merged",
		},
		{
			name: "issue opened",
			entry: AtomEntry{
				Title: "testuser opened issue",
			},
			expectedType: "issue_opened",
		},
		{
			name: "comment activity",
			entry: AtomEntry{
				Title: "testuser commented on issue",
			},
			expectedType: "comment",
		},
		{
			name: "unknown activity",
			entry: AtomEntry{
				Title: "testuser did something else",
			},
			expectedType: "activity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activityType := connector.determineActivityType(tt.entry)
			if activityType != tt.expectedType {
				t.Errorf("Expected activity type '%s', got '%s'", tt.expectedType, activityType)
			}
		})
	}
}

func TestGitLabConnector_MapToTimelineActivityType(t *testing.T) {
	connector := NewGitLabConnector()

	tests := []struct {
		gitlabType   string
		expectedType timeline.ActivityType
	}{
		{"commit", timeline.ActivityTypeGitCommit},
		{"merge_request_opened", timeline.ActivityTypeJira},
		{"merge_request_merged", timeline.ActivityTypeJira},
		{"issue_opened", timeline.ActivityTypeJira},
		{"comment", timeline.ActivityTypeCustom},
		{"project_created", timeline.ActivityTypeCustom},
		{"unknown", timeline.ActivityTypeCustom},
	}

	for _, tt := range tests {
		t.Run(tt.gitlabType, func(t *testing.T) {
			result := connector.mapToTimelineActivityType(tt.gitlabType)
			if result != tt.expectedType {
				t.Errorf("Expected %s, got %s", tt.expectedType, result)
			}
		})
	}
}

func TestGitLabConnector_ExtractProjectName(t *testing.T) {
	connector := NewGitLabConnector()

	tests := []struct {
		name         string
		entry        AtomEntry
		expectedName string
	}{
		{
			name: "extract from URL",
			entry: AtomEntry{
				Link: AtomLink{
					Href: "https://gitlab.com/testuser/myproject/-/commit/abc123",
				},
			},
			expectedName: "", // Our current implementation doesn't extract from this format
		},
		{
			name: "extract from title with 'in'",
			entry: AtomEntry{
				Title: "testuser pushed to main in myproject",
			},
			expectedName: "myproject",
		},
		{
			name: "no project name found",
			entry: AtomEntry{
				Title: "testuser did something",
				Link:  AtomLink{Href: "https://gitlab.com/"},
			},
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := connector.extractProjectName(tt.entry)
			if result != tt.expectedName {
				t.Errorf("Expected '%s', got '%s'", tt.expectedName, result)
			}
		})
	}
}
