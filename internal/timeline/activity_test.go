package timeline

import (
	"testing"
	"time"
)

func TestNewTimeline(t *testing.T) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	timeline := NewTimeline(date)

	if timeline == nil {
		t.Fatal("NewTimeline returned nil")
	}

	if !timeline.Date.Equal(date) {
		t.Errorf("Expected date %v, got %v", date, timeline.Date)
	}

	if timeline.Activities == nil {
		t.Error("Activities slice should be initialized")
	}

	if len(timeline.Activities) != 0 {
		t.Errorf("Expected empty activities slice, got %d items", len(timeline.Activities))
	}
}

func TestActivity_String(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	activity := Activity{
		ID:          "test-1",
		Type:        ActivityTypeGitCommit,
		Title:       "Fix bug in authentication",
		Description: "Updated OAuth flow",
		Timestamp:   timestamp,
		Source:      "github",
	}

	expected := "[14:30] git_commit - Fix bug in authentication (github)"
	result := activity.String()

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestActivity_FormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration *time.Duration
		expected string
	}{
		{
			name:     "nil duration",
			duration: nil,
			expected: "N/A",
		},
		{
			name:     "seconds only",
			duration: durationPtr(30 * time.Second),
			expected: "30s",
		},
		{
			name:     "minutes only",
			duration: durationPtr(5 * time.Minute),
			expected: "5m",
		},
		{
			name:     "minutes and seconds",
			duration: durationPtr(5*time.Minute + 30*time.Second),
			expected: "5m",
		},
		{
			name:     "hours only",
			duration: durationPtr(2 * time.Hour),
			expected: "2h 0m",
		},
		{
			name:     "hours and minutes",
			duration: durationPtr(2*time.Hour + 30*time.Minute),
			expected: "2h 30m",
		},
		{
			name:     "hours, minutes, and seconds",
			duration: durationPtr(2*time.Hour + 30*time.Minute + 45*time.Second),
			expected: "2h 30m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := Activity{Duration: tt.duration}
			result := activity.FormatDuration()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTimeline_AddActivity(t *testing.T) {
	timeline := NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

	// Add activities in non-chronological order
	activity2 := Activity{
		ID:        "test-2",
		Type:      ActivityTypeGitCommit,
		Title:     "Second commit",
		Timestamp: time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC),
		Source:    "github",
	}

	activity1 := Activity{
		ID:        "test-1",
		Type:      ActivityTypeCalendar,
		Title:     "First meeting",
		Timestamp: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
		Source:    "calendar",
	}

	timeline.AddActivity(activity2)
	timeline.AddActivity(activity1)

	if len(timeline.Activities) != 2 {
		t.Errorf("Expected 2 activities, got %d", len(timeline.Activities))
	}

	// Check that activities are sorted by timestamp
	if timeline.Activities[0].ID != "test-1" {
		t.Error("First activity should be the one with earliest timestamp")
	}

	if timeline.Activities[1].ID != "test-2" {
		t.Error("Second activity should be the one with latest timestamp")
	}
}

func TestTimeline_AddActivities(t *testing.T) {
	timeline := NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

	activities := []Activity{
		{
			ID:        "test-3",
			Type:      ActivityTypeJira,
			Title:     "Third task",
			Timestamp: time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC),
			Source:    "jira",
		},
		{
			ID:        "test-1",
			Type:      ActivityTypeCalendar,
			Title:     "First meeting",
			Timestamp: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			Source:    "calendar",
		},
		{
			ID:        "test-2",
			Type:      ActivityTypeGitCommit,
			Title:     "Second commit",
			Timestamp: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			Source:    "github",
		},
	}

	timeline.AddActivities(activities)

	if len(timeline.Activities) != 3 {
		t.Errorf("Expected 3 activities, got %d", len(timeline.Activities))
	}

	// Verify chronological order
	expectedOrder := []string{"test-1", "test-2", "test-3"}
	for i, expectedID := range expectedOrder {
		if timeline.Activities[i].ID != expectedID {
			t.Errorf("Activity at position %d should have ID %s, got %s", i, expectedID, timeline.Activities[i].ID)
		}
	}
}

func TestTimeline_Sort(t *testing.T) {
	timeline := &Timeline{
		Date: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Activities: []Activity{
			{
				ID:        "test-3",
				Timestamp: time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC),
			},
			{
				ID:        "test-1",
				Timestamp: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			},
			{
				ID:        "test-2",
				Timestamp: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	timeline.Sort()

	expectedOrder := []string{"test-1", "test-2", "test-3"}
	for i, expectedID := range expectedOrder {
		if timeline.Activities[i].ID != expectedID {
			t.Errorf("Activity at position %d should have ID %s, got %s", i, expectedID, timeline.Activities[i].ID)
		}
	}
}

func TestTimeline_FilterByType(t *testing.T) {
	timeline := createTestTimeline()

	filtered := timeline.FilterByType(ActivityTypeGitCommit)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 git commit activities, got %d", len(filtered))
	}

	for _, activity := range filtered {
		if activity.Type != ActivityTypeGitCommit {
			t.Errorf("Expected ActivityTypeGitCommit, got %s", activity.Type)
		}
	}

	// Test with non-existent type
	filtered = timeline.FilterByType(ActivityTypeSlack)
	if len(filtered) != 0 {
		t.Errorf("Expected 0 slack activities, got %d", len(filtered))
	}
}

func TestTimeline_FilterBySource(t *testing.T) {
	timeline := createTestTimeline()

	filtered := timeline.FilterBySource("github")

	if len(filtered) != 2 {
		t.Errorf("Expected 2 github activities, got %d", len(filtered))
	}

	for _, activity := range filtered {
		if activity.Source != "github" {
			t.Errorf("Expected source 'github', got %s", activity.Source)
		}
	}

	// Test with non-existent source
	filtered = timeline.FilterBySource("nonexistent")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 nonexistent source activities, got %d", len(filtered))
	}
}

func TestTimeline_FilterByTimeRange(t *testing.T) {
	timeline := createTestTimeline()

	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

	filtered := timeline.FilterByTimeRange(start, end)

	if len(filtered) != 1 {
		t.Errorf("Expected 1 activity in time range, got %d", len(filtered))
	}

	if filtered[0].ID != "test-2" {
		t.Errorf("Expected activity with ID 'test-2', got %s", filtered[0].ID)
	}
}

func TestTimeline_GetTimeRange(t *testing.T) {
	// Test empty timeline
	emptyTimeline := NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
	start, end := emptyTimeline.GetTimeRange()

	if !start.IsZero() || !end.IsZero() {
		t.Error("Empty timeline should return zero times")
	}

	// Test timeline with activities
	timeline := createTestTimeline()
	start, end = timeline.GetTimeRange()

	expectedStart := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	expectedEnd := time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC)

	if !start.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, start)
	}

	if !end.Equal(expectedEnd) {
		t.Errorf("Expected end time %v, got %v", expectedEnd, end)
	}
}

func TestTimeline_GetSummary(t *testing.T) {
	timeline := createTestTimeline()
	summary := timeline.GetSummary()

	expectedDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !summary.Date.Equal(expectedDate) {
		t.Errorf("Expected summary date %v, got %v", expectedDate, summary.Date)
	}

	if summary.TotalActivities != 4 {
		t.Errorf("Expected 4 total activities, got %d", summary.TotalActivities)
	}

	// Check type counts
	expectedTypeCounts := map[ActivityType]int{
		ActivityTypeGitCommit: 2,
		ActivityTypeCalendar:  1,
		ActivityTypeJira:      1,
	}

	for actType, expectedCount := range expectedTypeCounts {
		if summary.ByType[actType] != expectedCount {
			t.Errorf("Expected %d activities of type %s, got %d", expectedCount, actType, summary.ByType[actType])
		}
	}

	// Check source counts
	expectedSourceCounts := map[string]int{
		"github":   2,
		"calendar": 1,
		"jira":     1,
	}

	for source, expectedCount := range expectedSourceCounts {
		if summary.BySource[source] != expectedCount {
			t.Errorf("Expected %d activities from source %s, got %d", expectedCount, source, summary.BySource[source])
		}
	}

	// Check time range
	expectedStart := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	expectedEnd := time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC)

	if !summary.TimeRange.Start.Equal(expectedStart) {
		t.Errorf("Expected summary start time %v, got %v", expectedStart, summary.TimeRange.Start)
	}

	if !summary.TimeRange.End.Equal(expectedEnd) {
		t.Errorf("Expected summary end time %v, got %v", expectedEnd, summary.TimeRange.End)
	}
}

func TestActivityTypes(t *testing.T) {
	expectedTypes := []ActivityType{
		ActivityTypeGitCommit,
		ActivityTypeCalendar,
		ActivityTypeSlack,
		ActivityTypeJira,
		ActivityTypeYouTrack,
		ActivityTypeCustom,
		ActivityTypeFile,
		ActivityTypeBrowser,
		ActivityTypeApplication,
	}

	expectedValues := []string{
		"git_commit",
		"calendar",
		"slack",
		"jira",
		"youtrack",
		"custom",
		"file",
		"browser",
		"application",
	}

	for i, actType := range expectedTypes {
		if string(actType) != expectedValues[i] {
			t.Errorf("Expected activity type %s to have value %s, got %s",
				actType, expectedValues[i], string(actType))
		}
	}
}

// Helper functions

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func createTestTimeline() *Timeline {
	timeline := NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

	activities := []Activity{
		{
			ID:          "test-1",
			Type:        ActivityTypeCalendar,
			Title:       "Morning standup",
			Description: "Daily team meeting",
			Timestamp:   time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			Duration:    durationPtr(30 * time.Minute),
			Source:      "calendar",
			URL:         "https://calendar.example.com/event/1",
			Metadata:    map[string]string{"meeting_id": "123"},
		},
		{
			ID:          "test-2",
			Type:        ActivityTypeGitCommit,
			Title:       "Fix authentication bug",
			Description: "Updated OAuth flow to handle edge cases",
			Timestamp:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			Source:      "github",
			URL:         "https://github.com/example/repo/commit/abc123",
		},
		{
			ID:          "test-3",
			Type:        ActivityTypeJira,
			Title:       "Complete user story XYZ-123",
			Description: "Implemented new feature for user dashboard",
			Timestamp:   time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC),
			Duration:    durationPtr(2 * time.Hour),
			Source:      "jira",
			URL:         "https://jira.example.com/browse/XYZ-123",
		},
		{
			ID:          "test-4",
			Type:        ActivityTypeGitCommit,
			Title:       "Add unit tests",
			Description: "Increased test coverage to 85%",
			Timestamp:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			Source:      "github",
			URL:         "https://github.com/example/repo/commit/def456",
		},
	}

	timeline.AddActivities(activities)
	return timeline
}
