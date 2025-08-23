package connectors

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// Error constants for reusable error messages
const (
	errMacOSOnly             = "macOS system events connector only works on macOS"
	errLogCommandFailed      = "failed to execute log command: %v"
	errLogCommandUnavailable = "log command not available: %v"
	errParseLogOutput        = "failed to parse log output: %v"
)

// Compile regex once at package level for performance
var (
	// Regex to parse log lines
	// Expected format: 2024-01-01 12:34:56.789000-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0
	logRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+[+-]\d{4})\s+.*?loginwindow:.*?setScreenIsLocked.*?setting to (\d+)`)
)

// MacOSSystemConnector implements the Connector interface for macOS system events
type MacOSSystemConnector struct {
	*BaseConnector
	isCompatible bool // Cache macOS detection to avoid repeated runtime checks
}

// NewMacOSSystemConnector creates a new macOS system events connector
func NewMacOSSystemConnector() *MacOSSystemConnector {
	return &MacOSSystemConnector{
		BaseConnector: NewBaseConnector(
			"macos_system",
			"Fetches macOS system events (lock/unlock) using system logs",
		),
		isCompatible: runtime.GOOS == "darwin",
	}
}

// GetRequiredConfig returns the required configuration for macOS system events
func (c *MacOSSystemConnector) GetRequiredConfig() []ConfigField {
	// No additional configuration needed beyond base connector
	// The redundant "enabled" field has been removed as it's handled by BaseConnector
	return []ConfigField{}
}

// ValidateConfig validates the macOS system events configuration
func (c *MacOSSystemConnector) ValidateConfig(config map[string]interface{}) error {
	// Use cached compatibility check instead of runtime.GOOS
	if !c.isCompatible {
		return fmt.Errorf(errMacOSOnly)
	}

	// Check if log command is available
	if _, err := exec.LookPath("log"); err != nil {
		return fmt.Errorf(errLogCommandUnavailable, err)
	}

	return nil
}

// TestConnection tests the macOS system events connection
func (c *MacOSSystemConnector) TestConnection(ctx context.Context) error {
	// Use cached compatibility check
	if !c.isCompatible {
		return fmt.Errorf(errMacOSOnly)
	}

	// Test if we can execute the log command with a simple query
	cmd := exec.CommandContext(ctx, "log", "show", "--predicate", "(process == \"loginwindow\")", "--last", "1m", "--info")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(errLogCommandFailed, err)
	}

	return nil
}

// buildLogCommand creates a log command for the specified date
func (c *MacOSSystemConnector) buildLogCommand(ctx context.Context, date time.Time) *exec.Cmd {
	// More efficient date formatting - avoid multiple sprintf calls
	dateStr := date.Format("2006-01-02")

	return exec.CommandContext(ctx,
		"log", "show",
		"--start", dateStr+" 00:00:00",
		"--end", dateStr+" 23:59:59",
		"--predicate", "(process == \"loginwindow\" AND eventMessage CONTAINS \"setScreenIsLocked\")",
		"--info",
	)
}

// GetActivities retrieves macOS system events for the specified date
func (c *MacOSSystemConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	// Use cached compatibility check
	if !c.isCompatible {
		return nil, fmt.Errorf(errMacOSOnly)
	}

	// Use extracted helper method for command building
	cmd := c.buildLogCommand(ctx, date)

	// Execute the command
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(errLogCommandFailed, err)
	}

	// Parse the output
	activities, err := c.parseLogOutput(string(output), date)
	if err != nil {
		return nil, fmt.Errorf(errParseLogOutput, err)
	}

	return activities, nil
}

// parseLogOutput parses the output from the log command and extracts activities
func (c *MacOSSystemConnector) parseLogOutput(output string, date time.Time) ([]timeline.Activity, error) {
	if output == "" {
		return []timeline.Activity{}, nil
	}

	lines := strings.Split(output, "\n")
	// Pre-allocate slice with estimated capacity to reduce reallocations
	activities := make([]timeline.Activity, 0, len(lines)/10) // Rough estimate

	dateStr := date.Format("2006-01-02") // Cache date string for comparison

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Use pre-compiled regex for better performance
		matches := logRegex.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		// Parse timestamp
		timestampStr := matches[1]
		timestamp, err := c.parseTimestamp(timestampStr)
		if err != nil {
			continue // Skip invalid timestamps
		}

		// The logs from macOS do not filter the end date properly sometimes, so we need to check if the timestamp is within the date range
		if timestamp.Format("2006-01-02") != dateStr {
			continue
		}

		// Parse lock state and create activity
		lockState := matches[2]
		activity := c.createActivityFromLockState(lockState, timestamp, timestampStr)
		if activity != nil {
			activities = append(activities, *activity)
		}
	}

	return activities, nil
}

// parseTimestamp parses timestamp string with fallback for different formats
func (c *MacOSSystemConnector) parseTimestamp(timestampStr string) (time.Time, error) {
	// Try primary format first (with microseconds)
	if timestamp, err := time.Parse("2006-01-02 15:04:05.000000-0700", timestampStr); err == nil {
		return timestamp, nil
	}

	// Fall back to format without microseconds
	return time.Parse("2006-01-02 15:04:05-0700", timestampStr)
}

// createActivityFromLockState creates an activity based on the lock state
func (c *MacOSSystemConnector) createActivityFromLockState(lockState string, timestamp time.Time, timestampStr string) *timeline.Activity {
	var title, description string
	var activityType timeline.ActivityType = timeline.ActivityTypeSystem

	switch lockState {
	case "0":
		title = "Computer is active, user started working"
		description = "Screen unlocked - computer became active"
	case "1":
		title = "Computer is idle, user left the computer"
		description = "Screen locked - computer became idle"
	default:
		return nil // Skip unknown states
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("macos-system-%d", timestamp.Unix()),
		Type:        activityType,
		Title:       title,
		Description: fmt.Sprintf("On %s : %s", timestampStr, description),
		Timestamp:   timestamp,
		Source:      "macos_system",
		Metadata: map[string]string{
			"lock_state": lockState,
			"process":    "loginwindow",
			"event_type": "screen_lock_change",
		},
	}
}

// IsEnabled returns whether this connector is enabled
func (c *MacOSSystemConnector) IsEnabled() bool {
	// Simplified logic: only available on macOS and when base connector is enabled
	// No need to check config "enabled" field since it's been removed
	return c.isCompatible && c.BaseConnector.IsEnabled()
}
