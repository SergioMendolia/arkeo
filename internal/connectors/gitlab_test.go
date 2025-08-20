package connectors

import (
	"strings"
	"testing"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

func TestNewGitLabConnector(t *testing.T) {
	connector := NewGitLabConnector()

	if connector.Name() != "gitlab" {
		t.Errorf("Expected name 'gitlab', got %s", connector.Name())
	}

	if connector.Description() != "Fetches user activities from GitLab Events API (all branches, merge requests, issues)" {
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
		"gitlab_url":   false, // not required
		"username":     true,  // required
		"access_token": true,  // required
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
				"gitlab_url":   "https://gitlab.com",
				"username":     "testuser",
				"access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
			},
			expectError: false,
		},
		{
			name: "missing username",
			config: map[string]interface{}{
				"gitlab_url":   "https://gitlab.com",
				"access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
			},
			expectError: true,
			errorMsg:    "gitlab username is required",
		},
		{
			name: "missing access token",
			config: map[string]interface{}{
				"gitlab_url": "https://gitlab.com",
				"username":   "testuser",
			},
			expectError: true,
			errorMsg:    "gitlab access token is required",
		},
		{
			name: "invalid gitlab url",
			config: map[string]interface{}{
				"gitlab_url":   "://invalid-url",
				"username":     "testuser",
				"access_token": "glpat-xxxxxxxxxxxxxxxxxxxx",
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
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}

func TestGitLabConnector_StripHTMLTags(t *testing.T) {
	connector := NewGitLabConnector()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple_html_tags",
			input:    "<p>This is a <strong>test</strong> message</p>",
			expected: "This is a test message",
		},
		{
			name:     "multiple_tags",
			input:    "<div><h1>Title</h1><p>Content with <a href='#'>link</a></p></div>",
			expected: "Title Content with link",
		},
		{
			name:     "no_html_tags",
			input:    "Plain text without any tags",
			expected: "Plain text without any tags",
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "",
		},
		{
			name:     "only_whitespace",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "html_with_whitespace",
			input:    "<p>  Text with  \n  extra   spaces  </p>",
			expected: "Text with extra spaces",
		},
		{
			name:     "self_closing_tags",
			input:    "Line 1<br/>Line 2<hr/>Line 3",
			expected: "Line 1 Line 2 Line 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := connector.stripHTMLTags(tt.input)
			if result != tt.expected {
				t.Errorf("stripHTMLTags() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGitLabConnector_ConvertPushEventToActivity(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - single commit push event
	event := GitLabEvent{
		ID:         123,
		ActionName: "pushed to",
		ProjectID:  456,
		CreatedAt:  "2024-01-15T10:30:00Z",
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		PushData: &GitLabPushData{
			CommitCount: 1,
			Action:      "pushed",
			RefType:     "branch",
			CommitFrom:  "abc123",
			CommitTo:    "def456",
			Ref:         "feature-branch",
			CommitTitle: "Fix authentication bug",
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	eventTime, _ := time.Parse(time.RFC3339, event.CreatedAt)
	activity := connector.convertPushEventToActivity(event, eventTime)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Verify activity properties
	if activity.ID != "gitlab-push-123" {
		t.Errorf("Expected ID 'gitlab-push-123', got '%s'", activity.ID)
	}

	if activity.Title != "Fix authentication bug" {
		t.Errorf("Expected title 'Fix authentication bug', got '%s'", activity.Title)
	}

	if activity.Source != "gitlab" {
		t.Errorf("Expected source 'gitlab', got '%s'", activity.Source)
	}

	expectedURL := "https://gitlab.com/testuser/test-project/-/commit/def456"
	if activity.URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, activity.URL)
	}

	expectedDescription := "Pushed to feature-branch branch in testuser/test-project"
	if activity.Description != expectedDescription {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription, activity.Description)
	}

	// Check metadata
	if activity.Metadata["event_id"] != "123" {
		t.Errorf("Expected event_id '123', got '%s'", activity.Metadata["event_id"])
	}

	if activity.Metadata["ref"] != "feature-branch" {
		t.Errorf("Expected ref 'feature-branch', got '%s'", activity.Metadata["ref"])
	}

	if activity.Metadata["commit_count"] != "1" {
		t.Errorf("Expected commit_count '1', got '%s'", activity.Metadata["commit_count"])
	}

	if activity.Metadata["author"] != "testuser" {
		t.Errorf("Expected author 'testuser', got '%s'", activity.Metadata["author"])
	}
}

func TestGitLabConnector_ConvertMultiCommitPushEvent(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - multi-commit push event
	event := GitLabEvent{
		ID:         124,
		ActionName: "pushed to",
		ProjectID:  456,
		CreatedAt:  "2024-01-15T11:00:00Z",
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		PushData: &GitLabPushData{
			CommitCount: 3,
			Action:      "pushed",
			RefType:     "branch",
			CommitFrom:  "abc123",
			CommitTo:    "ghi789",
			Ref:         "main",
			CommitTitle: "", // Empty title for multi-commit
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	eventTime, _ := time.Parse(time.RFC3339, event.CreatedAt)
	activity := connector.convertPushEventToActivity(event, eventTime)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Should generate a default title for multi-commit push
	expectedTitle := "Pushed 3 commits to main"
	if activity.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, activity.Title)
	}

	// Should include commit count in description
	expectedDescription := "Pushed to main branch in testuser/test-project (3 commits)"
	if activity.Description != expectedDescription {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription, activity.Description)
	}

	// Should link to compare view for multiple commits
	expectedURL := "https://gitlab.com/testuser/test-project/-/compare/abc123...ghi789"
	if activity.URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, activity.URL)
	}
}

func TestGitLabConnector_TestConnection_MissingToken(t *testing.T) {
	connector := NewGitLabConnector()

	// Configure with missing access token - should fail in validation
	config := map[string]interface{}{
		"gitlab_url": "https://gitlab.com",
		"username":   "testuser",
		"log_level":  "info",
	}

	err := connector.Configure(config)
	if err == nil {
		t.Fatal("Expected error when configuring without access token")
	}

	expectedErrorMsg := "gitlab access token is required"
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

func TestGitLabConnector_ConvertMergeRequestEvent(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - merge request opened event
	event := GitLabEvent{
		ID:          125,
		ActionName:  "opened",
		ProjectID:   456,
		CreatedAt:   "2024-01-15T12:30:00Z",
		TargetType:  stringPtr("MergeRequest"),
		TargetID:    intPtr(789),
		TargetIID:   intPtr(42),
		TargetTitle: stringPtr("Add new feature for user authentication"),
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	eventTime, _ := time.Parse(time.RFC3339, event.CreatedAt)
	activity := connector.convertMergeRequestEventToActivity(event, eventTime)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Verify activity properties
	if activity.ID != "gitlab-mr-125" {
		t.Errorf("Expected ID 'gitlab-mr-125', got '%s'", activity.ID)
	}

	expectedTitle := "Opened merge request: Add new feature for user authentication"
	if activity.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, activity.Title)
	}

	if activity.Type != timeline.ActivityTypeJira {
		t.Errorf("Expected type 'jira', got '%s'", activity.Type)
	}

	expectedURL := "https://gitlab.com/testuser/test-project/-/merge_requests/42"
	if activity.URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, activity.URL)
	}

	expectedDescription := "Opened merge request in testuser/test-project (!42)"
	if activity.Description != expectedDescription {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription, activity.Description)
	}

	// Check metadata
	if activity.Metadata["merge_request_iid"] != "42" {
		t.Errorf("Expected merge_request_iid '42', got '%s'", activity.Metadata["merge_request_iid"])
	}
}

func TestGitLabConnector_ConvertIssueEvent(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - issue created event
	event := GitLabEvent{
		ID:          126,
		ActionName:  "created",
		ProjectID:   456,
		CreatedAt:   "2024-01-15T13:00:00Z",
		TargetType:  stringPtr("Issue"),
		TargetID:    intPtr(101),
		TargetIID:   intPtr(25),
		TargetTitle: stringPtr("Bug in login form validation"),
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	eventTime, _ := time.Parse(time.RFC3339, event.CreatedAt)
	activity := connector.convertIssueEventToActivity(event, eventTime)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Verify activity properties
	if activity.ID != "gitlab-issue-126" {
		t.Errorf("Expected ID 'gitlab-issue-126', got '%s'", activity.ID)
	}

	expectedTitle := "Opened issue: Bug in login form validation"
	if activity.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, activity.Title)
	}

	if activity.Type != timeline.ActivityTypeJira {
		t.Errorf("Expected type 'jira', got '%s'", activity.Type)
	}

	expectedURL := "https://gitlab.com/testuser/test-project/-/issues/25"
	if activity.URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, activity.URL)
	}

	expectedDescription := "Opened issue in testuser/test-project (#25)"
	if activity.Description != expectedDescription {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription, activity.Description)
	}

	// Check metadata
	if activity.Metadata["issue_iid"] != "25" {
		t.Errorf("Expected issue_iid '25', got '%s'", activity.Metadata["issue_iid"])
	}
}

func TestGitLabConnector_ConvertCommentEvent(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - comment on merge request event
	event := GitLabEvent{
		ID:          127,
		ActionName:  "commented on",
		ProjectID:   456,
		CreatedAt:   "2024-01-15T14:15:00Z",
		TargetType:  stringPtr("MergeRequest"),
		TargetID:    intPtr(789),
		TargetIID:   intPtr(42),
		TargetTitle: stringPtr("Add new feature for user authentication"),
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	eventTime, _ := time.Parse(time.RFC3339, event.CreatedAt)
	activity := connector.convertCommentEventToActivity(event, eventTime)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Verify activity properties
	if activity.ID != "gitlab-comment-127" {
		t.Errorf("Expected ID 'gitlab-comment-127', got '%s'", activity.ID)
	}

	expectedTitle := "Commented on merge request: Add new feature for user authentication"
	if activity.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, activity.Title)
	}

	if activity.Type != timeline.ActivityTypeCustom {
		t.Errorf("Expected type 'custom', got '%s'", activity.Type)
	}

	expectedURL := "https://gitlab.com/testuser/test-project/-/merge_requests/42"
	if activity.URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, activity.URL)
	}

	expectedDescription := "Commented on merge request in testuser/test-project"
	if activity.Description != expectedDescription {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription, activity.Description)
	}

	// Check metadata
	if activity.Metadata["target_type"] != "merge request" {
		t.Errorf("Expected target_type 'merge request', got '%s'", activity.Metadata["target_type"])
	}
}

func TestGitLabConnector_ConvertEventToActivity_UnhandledType(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - unknown event type
	event := GitLabEvent{
		ID:          128,
		ActionName:  "some_unknown_action",
		ProjectID:   456,
		CreatedAt:   "2024-01-15T15:00:00Z",
		TargetType:  stringPtr("UnknownType"),
		TargetTitle: stringPtr("Some unknown target"),
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	activity := connector.convertEventToActivity(event)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Should fall back to generic event handling
	if activity.ID != "gitlab-generic-128" {
		t.Errorf("Expected ID 'gitlab-generic-128', got '%s'", activity.ID)
	}

	expectedTitle := "Some_unknown_action: Some unknown target"
	if activity.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, activity.Title)
	}

	if activity.Type != timeline.ActivityTypeCustom {
		t.Errorf("Expected type 'custom', got '%s'", activity.Type)
	}
}

func TestGitLabConnector_ConvertEventToActivity_PushEvent(t *testing.T) {
	connector := NewGitLabConnector()

	// Test data - push event
	event := GitLabEvent{
		ID:         129,
		ActionName: "pushed to",
		ProjectID:  456,
		CreatedAt:  "2024-01-15T16:00:00Z",
		Author: GitLabEventAuthor{
			ID:       789,
			Username: "testuser",
			Name:     "Test User",
		},
		PushData: &GitLabPushData{
			CommitCount: 2,
			Action:      "pushed",
			RefType:     "branch",
			CommitFrom:  "abc123",
			CommitTo:    "def456",
			Ref:         "main",
			CommitTitle: "Update README",
		},
		Project: &GitLabEventProject{
			ID:                456,
			Name:              "test-project",
			Path:              "test-project",
			PathWithNamespace: "testuser/test-project",
			WebURL:            "https://gitlab.com/testuser/test-project",
		},
	}

	activity := connector.convertEventToActivity(event)

	if activity == nil {
		t.Fatal("Expected activity, got nil")
	}

	// Should be handled as push event
	if activity.ID != "gitlab-push-129" {
		t.Errorf("Expected ID 'gitlab-push-129', got '%s'", activity.ID)
	}

	if activity.Type != timeline.ActivityTypeGitCommit {
		t.Errorf("Expected type 'git_commit', got '%s'", activity.Type)
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
