package display

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

func TestDefaultTimelineOptions(t *testing.T) {
	opts := DefaultTimelineOptions()

	if opts.ShowDetails {
		t.Error("ShowDetails should be false by default")
	}

	if !opts.ShowTimestamps {
		t.Error("ShowTimestamps should be true by default")
	}

	if opts.GroupByHour {
		t.Error("GroupByHour should be false by default")
	}

	if opts.MaxItems != 500 {
		t.Errorf("Expected MaxItems to be 500, got %d", opts.MaxItems)
	}

	if opts.Format != "table" {
		t.Errorf("Expected Format to be 'table', got %s", opts.Format)
	}
}

func TestDisplayTimeline_EmptyTimeline(t *testing.T) {
	// Capture stdout
	output := captureOutput(func() {
		tl := timeline.NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
		opts := DefaultTimelineOptions()

		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	expected := "No activities found for January 15, 2024\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestDisplayTimeline_TableFormat(t *testing.T) {
	tl := createTestTimeline()
	opts := DefaultTimelineOptions()
	opts.Format = "table"

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// Check for expected content
	expectedStrings := []string{
		"Timeline for Monday, January 15, 2024",
		"Found 3 activities",
		"Activities (chronological order):",
		"09:00 [calendar] Morning standup",
		"12:00 [github] Fix authentication bug",
		"14:30 [github] Add unit tests",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q, got:\n%s", expected, output)
		}
	}
}

func TestDisplayTimeline_WithDetails(t *testing.T) {
	tl := createTestTimeline()
	opts := DefaultTimelineOptions()
	opts.ShowDetails = true

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// Check for details
	expectedStrings := []string{
		"📝 Daily team meeting",
		"⏱️  30m",
		"🔗 https://calendar.example.com/event/1",
		"📝 Updated OAuth flow to handle edge cases",
		"🔗 https://github.com/example/repo/commit/abc123",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q when ShowDetails is true, got:\n%s", expected, output)
		}
	}
}

func TestDisplayTimeline_WithoutTimestamps(t *testing.T) {
	tl := createTestTimeline()
	opts := DefaultTimelineOptions()
	opts.ShowTimestamps = false

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// Should not contain timestamps
	if strings.Contains(output, "09:00") {
		t.Error("Output should not contain timestamps when ShowTimestamps is false")
	}

	// Should still contain activity info
	expectedStrings := []string{
		"[calendar] Morning standup",
		"[github] Fix authentication bug",
		"[github] Add unit tests",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q, got:\n%s", expected, output)
		}
	}
}

func TestDisplayTimeline_GroupByHour(t *testing.T) {
	tl := createTestTimeline()
	opts := DefaultTimelineOptions()
	opts.GroupByHour = true

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// Check for hour groupings
	expectedStrings := []string{
		"📅 09:00 (1 activities)",
		"📅 12:00 (1 activities)",
		"📅 14:00 (1 activities)",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q when GroupByHour is true, got:\n%s", expected, output)
		}
	}
}

func TestDisplayTimeline_MaxItems(t *testing.T) {
	tl := createTestTimelineWithManyActivities()
	opts := DefaultTimelineOptions()
	opts.MaxItems = 2

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// Should show only 2 activities
	if strings.Contains(output, "Found 2 activities") {
		// Count the number of activity entries
		activityCount := strings.Count(output, "[calendar]") + strings.Count(output, "[github]") + strings.Count(output, "[jira]")
		if activityCount > 2 {
			t.Errorf("Should display at most 2 activities, but found %d", activityCount)
		}
	}
}

func TestDisplayTimeline_CSVFormat(t *testing.T) {
	tl := createTestTimeline()
	opts := DefaultTimelineOptions()
	opts.Format = "csv"

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// Check CSV header
	if !strings.Contains(output, "timestamp,type,source,title,description,duration,url") {
		t.Error("CSV output should contain header row")
	}

	// Check CSV data rows
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 { // 1 header + 3 data rows
		t.Errorf("Expected 4 CSV lines, got %d", len(lines))
	}

	// Check first data row
	expectedFirstRow := "2024-01-15 09:00:00,calendar,calendar,Morning standup,Daily team meeting,30m,https://calendar.example.com/event/1"
	if lines[1] != expectedFirstRow {
		t.Errorf("Expected first CSV row %q, got %q", expectedFirstRow, lines[1])
	}
}

func TestDisplayTimeline_JSONFormat(t *testing.T) {
	tl := createTestTimeline()
	opts := DefaultTimelineOptions()
	opts.Format = "json"

	output := captureOutput(func() {
		err := DisplayTimeline(tl, opts)
		if err != nil {
			t.Errorf("DisplayTimeline failed: %v", err)
		}
	})

	// JSON output is not fully implemented, should show message
	expected := "JSON output not fully implemented. Use --format=table instead."
	if !strings.Contains(output, expected) {
		t.Errorf("Expected JSON not implemented message, got: %s", output)
	}
}

func TestDisplaySummary(t *testing.T) {
	tl := createTestTimeline()

	output := captureOutput(func() {
		DisplaySummary(tl)
	})

	expectedStrings := []string{
		"Timeline Summary for January 15, 2024",
		"📊 Total Activities: 3",
		"⏰ Time Range: 09:00 - 14:30",
		"📈 By Activity Type:",
		"calendar        1",
		"git_commit      2",
		"🔗 By Source:",
		"📋 calendar        1",
		"📋 github          2",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Summary should contain %q, got:\n%s", expected, output)
		}
	}
}

func TestDisplaySummary_EmptyTimeline(t *testing.T) {
	tl := timeline.NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

	output := captureOutput(func() {
		DisplaySummary(tl)
	})

	expectedStrings := []string{
		"Timeline Summary for January 15, 2024",
		"📊 Total Activities: 0",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Empty summary should contain %q, got:\n%s", expected, output)
		}
	}

	// Should not contain activity type or source sections for empty timeline
	unexpectedStrings := []string{
		"📈 By Activity Type:",
		"🔗 By Source:",
	}

	for _, unexpected := range unexpectedStrings {
		if strings.Contains(output, unexpected) {
			t.Errorf("Empty summary should not contain %q, got:\n%s", unexpected, output)
		}
	}
}

func TestDisplayConnectorStatus(t *testing.T) {
	connectors := map[string]bool{
		"github":   true,
		"calendar": false,
		"jira":     true,
		"slack":    false,
	}

	output := captureOutput(func() {
		DisplayConnectorStatus(connectors)
	})

	expectedStrings := []string{
		"Connector Status",
		"github          ✅ Enabled",
		"calendar        ❌ Disabled",
		"jira            ✅ Enabled",
		"slack           ❌ Disabled",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Connector status should contain %q, got:\n%s", expected, output)
		}
	}
}

func TestCSVEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple text",
			expected: "simple text",
		},
		{
			name:     "contains comma",
			input:    "hello, world",
			expected: `"hello, world"`,
		},
		{
			name:     "contains quote",
			input:    `hello "world"`,
			expected: `"hello ""world"""`,
		},
		{
			name:     "contains newline",
			input:    "hello\nworld",
			expected: "\"hello\nworld\"",
		},
		{
			name:     "contains comma and quote",
			input:    `hello, "world"`,
			expected: `"hello, ""world"""`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := csvEscape(tt.input)
			if result != tt.expected {
				t.Errorf("csvEscape(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper functions

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func createTestTimeline() *timeline.Timeline {
	tl := timeline.NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

	duration30m := 30 * time.Minute

	activities := []timeline.Activity{
		{
			ID:          "test-1",
			Type:        timeline.ActivityTypeCalendar,
			Title:       "Morning standup",
			Description: "Daily team meeting",
			Timestamp:   time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			Duration:    &duration30m,
			Source:      "calendar",
			URL:         "https://calendar.example.com/event/1",
			Metadata:    map[string]string{"meeting_id": "123"},
		},
		{
			ID:          "test-2",
			Type:        timeline.ActivityTypeGitCommit,
			Title:       "Fix authentication bug",
			Description: "Updated OAuth flow to handle edge cases",
			Timestamp:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			Source:      "github",
			URL:         "https://github.com/example/repo/commit/abc123",
		},
		{
			ID:          "test-3",
			Type:        timeline.ActivityTypeGitCommit,
			Title:       "Add unit tests",
			Description: "Increased test coverage to 85%",
			Timestamp:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			Source:      "github",
			URL:         "https://github.com/example/repo/commit/def456",
		},
	}

	tl.AddActivities(activities)
	return tl
}

func createTestTimelineWithManyActivities() *timeline.Timeline {
	tl := timeline.NewTimeline(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

	activities := []timeline.Activity{
		{
			ID:        "test-1",
			Type:      timeline.ActivityTypeCalendar,
			Title:     "Meeting 1",
			Timestamp: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			Source:    "calendar",
		},
		{
			ID:        "test-2",
			Type:      timeline.ActivityTypeGitCommit,
			Title:     "Commit 1",
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Source:    "github",
		},
		{
			ID:        "test-3",
			Type:      timeline.ActivityTypeJira,
			Title:     "Task 1",
			Timestamp: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			Source:    "jira",
		},
		{
			ID:        "test-4",
			Type:      timeline.ActivityTypeCalendar,
			Title:     "Meeting 2",
			Timestamp: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			Source:    "calendar",
		},
		{
			ID:        "test-5",
			Type:      timeline.ActivityTypeGitCommit,
			Title:     "Commit 2",
			Timestamp: time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC),
			Source:    "github",
		},
	}

	tl.AddActivities(activities)
	return tl
}
