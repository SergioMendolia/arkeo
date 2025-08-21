package connectors

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

func TestNewMacOSSystemConnector(t *testing.T) {
	connector := NewMacOSSystemConnector()

	if connector.Name() != "macos_system" {
		t.Errorf("Expected name 'macos_system', got '%s'", connector.Name())
	}

	if connector.Description() == "" {
		t.Error("Expected non-empty description")
	}
}

func TestMacOSSystemConnector_ValidateConfig(t *testing.T) {
	connector := NewMacOSSystemConnector()

	// Test with valid config
	config := map[string]interface{}{
		"enabled": true,
	}

	// Only test validation on macOS
	if runtime.GOOS == "darwin" {
		err := connector.ValidateConfig(config)
		if err != nil {
			t.Errorf("Expected no error for valid config, got: %v", err)
		}
	} else {
		// On non-macOS systems, should return error
		err := connector.ValidateConfig(config)
		if err == nil {
			t.Error("Expected error on non-macOS systems")
		}
	}
}

func TestMacOSSystemConnector_IsEnabled(t *testing.T) {
	connector := NewMacOSSystemConnector()

	// Configure with enabled=true
	config := map[string]interface{}{
		"enabled": true,
	}
	connector.Configure(config)
	connector.SetEnabled(true)

	// Should only be enabled on macOS
	if runtime.GOOS == "darwin" {
		if !connector.IsEnabled() {
			t.Error("Expected connector to be enabled on macOS")
		}
	} else {
		if connector.IsEnabled() {
			t.Error("Expected connector to be disabled on non-macOS systems")
		}
	}
}

func TestMacOSSystemConnector_GetRequiredConfig(t *testing.T) {
	connector := NewMacOSSystemConnector()
	config := connector.GetRequiredConfig()

	if len(config) == 0 {
		t.Error("Expected at least one config field")
	}

	// Check for enabled field
	found := false
	for _, field := range config {
		if field.Key == "enabled" {
			found = true
			if field.Type != "bool" {
				t.Errorf("Expected enabled field to be bool, got %s", field.Type)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find 'enabled' config field")
	}
}

func TestMacOSSystemConnector_parseLogOutput(t *testing.T) {
	connector := NewMacOSSystemConnector()

	// Test with sample log output
	logOutput := `2024-01-15 10:30:45.123456-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0
2024-01-15 11:45:30.789012-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 1
2024-01-15 12:15:22.456789-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0`

	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	activities, err := connector.parseLogOutput(logOutput, testDate)
	if err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	expectedCount := 3
	if len(activities) != expectedCount {
		t.Errorf("Expected %d activities, got %d", expectedCount, len(activities))
	}

	// Test first activity (unlock)
	if len(activities) > 0 {
		activity := activities[0]
		if activity.Title != "Computer is active, user started working" {
			t.Errorf("Expected title 'Computer is active, user started working', got '%s'", activity.Title)
		}
		if activity.Type != timeline.ActivityTypeSystem {
			t.Errorf("Expected type 'system', got '%s'", activity.Type)
		}
		if activity.Source != "macos_system" {
			t.Errorf("Expected source 'macos_system', got '%s'", activity.Source)
		}
		if activity.Metadata["lock_state"] != "0" {
			t.Errorf("Expected lock_state '0', got '%s'", activity.Metadata["lock_state"])
		}
	}

	// Test second activity (lock)
	if len(activities) > 1 {
		activity := activities[1]
		if activity.Title != "Computer is idle, user left the computer" {
			t.Errorf("Expected title 'Computer is idle, user left the computer', got '%s'", activity.Title)
		}
		if activity.Metadata["lock_state"] != "1" {
			t.Errorf("Expected lock_state '1', got '%s'", activity.Metadata["lock_state"])
		}
	}
}

func TestMacOSSystemConnector_parseLogOutput_EmptyInput(t *testing.T) {
	connector := NewMacOSSystemConnector()

	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	activities, err := connector.parseLogOutput("", testDate)
	if err != nil {
		t.Errorf("Expected no error for empty input, got: %v", err)
	}

	if len(activities) != 0 {
		t.Errorf("Expected 0 activities for empty input, got %d", len(activities))
	}
}

func TestMacOSSystemConnector_parseLogOutput_InvalidFormat(t *testing.T) {
	connector := NewMacOSSystemConnector()

	// Test with invalid log format
	logOutput := `invalid log line
another invalid line`

	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	activities, err := connector.parseLogOutput(logOutput, testDate)
	if err != nil {
		t.Errorf("Expected no error for invalid format, got: %v", err)
	}

	if len(activities) != 0 {
		t.Errorf("Expected 0 activities for invalid format, got %d", len(activities))
	}
}

func TestMacOSSystemConnector_TestConnection(t *testing.T) {
	connector := NewMacOSSystemConnector()
	ctx := context.Background()

	err := connector.TestConnection(ctx)

	// Only test actual connection on macOS
	if runtime.GOOS == "darwin" {
		// On macOS, the test might succeed or fail depending on system permissions
		// We'll just verify that the method doesn't panic and returns some result
		t.Logf("TestConnection result on macOS: %v", err)
	} else {
		// On non-macOS systems, should return error
		if err == nil {
			t.Error("Expected error on non-macOS systems")
		}
	}
}

func TestMacOSSystemConnector_GetActivities_NonMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping non-macOS test on macOS system")
	}

	connector := NewMacOSSystemConnector()
	ctx := context.Background()
	date := time.Now()

	_, err := connector.GetActivities(ctx, date)
	if err == nil {
		t.Error("Expected error on non-macOS systems")
	}
}

func TestMacOSSystemConnector_ActivityMetadata(t *testing.T) {
	connector := NewMacOSSystemConnector()

	// Test log output with setting to 0
	logOutput := `2024-01-15 10:30:45.123456-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0`

	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	activities, err := connector.parseLogOutput(logOutput, testDate)
	if err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if len(activities) != 1 {
		t.Fatalf("Expected 1 activity, got %d", len(activities))
	}

	activity := activities[0]
	expectedMetadata := map[string]string{
		"lock_state": "0",
		"process":    "loginwindow",
		"event_type": "screen_lock_change",
	}

	for key, expectedValue := range expectedMetadata {
		if actualValue, exists := activity.Metadata[key]; !exists {
			t.Errorf("Expected metadata key '%s' to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected metadata '%s' to be '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func TestMacOSSystemConnector_DateRangeCalculation(t *testing.T) {
	// This test verifies that the date range is calculated correctly
	// We can't easily test the actual command execution, but we can verify
	// the logic by checking the expected behavior

	// Test date: 2025-01-15
	testDate := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)

	// Expected start: 2025-01-15 00:00:00
	expectedStartTime := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	// Expected end: 2025-01-15 23:59:59.999999999
	expectedEndTime := time.Date(2025, 1, 15, 23, 59, 59, 999999999, time.UTC)

	// Simulate the date calculation logic from GetActivities
	startTime := time.Date(testDate.Year(), testDate.Month(), testDate.Day(), 0, 0, 0, 0, testDate.Location())
	endTime := time.Date(testDate.Year(), testDate.Month(), testDate.Day(), 23, 59, 59, 999999999, testDate.Location())

	if !startTime.Equal(expectedStartTime) {
		t.Errorf("Expected start time %v, got %v", expectedStartTime, startTime)
	}

	if !endTime.Equal(expectedEndTime) {
		t.Errorf("Expected end time %v, got %v", expectedEndTime, endTime)
	}

	// Verify the formatted strings are correct
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")

	expectedStartStr := "2025-01-15 00:00:00"
	expectedEndStr := "2025-01-15 23:59:59"

	if startStr != expectedStartStr {
		t.Errorf("Expected start string '%s', got '%s'", expectedStartStr, startStr)
	}

	if endStr != expectedEndStr {
		t.Errorf("Expected end string '%s', got '%s'", expectedEndStr, endStr)
	}
}

func TestMacOSSystemConnector_parseLogOutput_DateFiltering(t *testing.T) {
	connector := NewMacOSSystemConnector()

	// Test log output with events from different dates
	logOutput := `2024-01-14 23:30:45.123456-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0
2024-01-15 10:30:45.123456-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 1
2024-01-16 08:15:22.456789-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0`

	// Filter for January 15th only
	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	activities, err := connector.parseLogOutput(logOutput, testDate)
	if err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Should only get the January 15th event
	expectedCount := 1
	if len(activities) != expectedCount {
		t.Errorf("Expected %d activities after date filtering, got %d", expectedCount, len(activities))
	}

	if len(activities) > 0 {
		activity := activities[0]
		if activity.Title != "Computer is idle, user left the computer" {
			t.Errorf("Expected title 'Computer is idle, user left the computer', got '%s'", activity.Title)
		}
		if activity.Metadata["lock_state"] != "1" {
			t.Errorf("Expected lock_state '1', got '%s'", activity.Metadata["lock_state"])
		}

		// Verify the timestamp is on the correct date
		expectedDate := "2024-01-15"
		if activity.Timestamp.Format("2006-01-02") != expectedDate {
			t.Errorf("Expected timestamp date '%s', got '%s'", expectedDate, activity.Timestamp.Format("2006-01-02"))
		}
	}

}
