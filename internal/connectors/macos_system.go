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

// MacOSSystemConnector implements the Connector interface for macOS system events
type MacOSSystemConnector struct {
	*BaseConnector
}

// NewMacOSSystemConnector creates a new macOS system events connector
func NewMacOSSystemConnector() *MacOSSystemConnector {
	return &MacOSSystemConnector{
		BaseConnector: NewBaseConnector(
			"macos_system",
			"Fetches macOS system events (lock/unlock) using system logs",
		),
	}
}

// GetRequiredConfig returns the required configuration for macOS system events
func (c *MacOSSystemConnector) GetRequiredConfig() []ConfigField {
	return []ConfigField{
		{
			Key:         "enabled",
			Type:        "bool",
			Required:    false,
			Description: "Enable macOS system events connector",
			Default:     "true",
		},
	}
}

// ValidateConfig validates the macOS system events configuration
func (c *MacOSSystemConnector) ValidateConfig(config map[string]interface{}) error {
	// Check if we're running on macOS
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macOS system events connector only works on macOS")
	}

	// Check if log command is available
	if _, err := exec.LookPath("log"); err != nil {
		return fmt.Errorf("log command not available: %v", err)
	}

	return nil
}

// TestConnection tests the macOS system events connection
func (c *MacOSSystemConnector) TestConnection(ctx context.Context) error {
	// Check if we're running on macOS
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macOS system events connector only works on macOS")
	}

	// Test if we can execute the log command with a simple query
	cmd := exec.CommandContext(ctx, "log", "show", "--predicate", "(process == \"loginwindow\")", "--last", "1m", "--info")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute log command: %v", err)
	}

	return nil
}

// GetActivities retrieves macOS system events for the specified date
func (c *MacOSSystemConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	// Check if we're running on macOS
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("macOS system events connector only works on macOS")
	}

	// Format times for the log command (log show expects YYYY-MM-DD HH:MM:SS format)
	startStr := fmt.Sprintf("%s %s", date.Format("2006-01-02"), "00:00:00")
	endStr := fmt.Sprintf("%s %s", date.Format("2006-01-02"), "23:59:59")

	// Build the log command with the specified date range
	cmd := exec.CommandContext(ctx,
		"log", "show",
		"--start", startStr,
		"--end", endStr,
		"--predicate", "(process == \"loginwindow\" AND eventMessage CONTAINS \"setScreenIsLocked\")",
		"--info",
	)

	// Execute the command
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute log command: %v", err)
	}

	// Parse the output
	activities, err := c.parseLogOutput(string(output), date)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log output: %v", err)
	}

	return activities, nil
}

// parseLogOutput parses the output from the log command and extracts activities
func (c *MacOSSystemConnector) parseLogOutput(output string, date time.Time) ([]timeline.Activity, error) {
	var activities []timeline.Activity
	lines := strings.Split(output, "\n")

	// Regex to parse log lines
	// Expected format: 2024-01-01 12:34:56.789000-0800  0x12345    Default     0x0      12345   0          loginwindow: (libxpc.dylib) [com.apple.xpc:activity] entering com.apple.coreservices.appleid.authentication setScreenIsLocked, setting to 0
	logRegex := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+[+-]\d{4})\s+.*?loginwindow:.*?setScreenIsLocked.*?setting to (\d+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := logRegex.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		// Parse timestamp
		timestampStr := matches[1]
		timestamp, err := time.Parse("2006-01-02 15:04:05.000000-0700", timestampStr)
		if err != nil {
			// Try alternative format without microseconds
			timestamp, err = time.Parse("2006-01-02 15:04:05-0700", timestampStr)
			if err != nil {
				continue
			}
		}

		//the logs from macos do not filter the end date properly sometimes, so we need to check if the timestamp is within the date range
		if timestamp.Format("2006-01-02") != date.Format("2006-01-02") {
			continue
		}

		// Parse lock state
		lockState := matches[2]
		var title, description string
		var activityType timeline.ActivityType

		switch lockState {
		case "0":
			title = "Computer is active, user started working"
			description = "Screen unlocked - computer became active"
			activityType = timeline.ActivityTypeSystem
		case "1":
			title = "Computer is idle, user left the computer"
			description = "Screen locked - computer became idle"
			activityType = timeline.ActivityTypeSystem
		default:
			continue // Skip unknown states
		}

		// Create activity
		activity := timeline.Activity{
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

		activities = append(activities, activity)
	}

	return activities, nil
}

// IsEnabled returns whether this connector is enabled
func (c *MacOSSystemConnector) IsEnabled() bool {
	// Only enable on macOS
	if runtime.GOOS != "darwin" {
		return false
	}

	// Check if explicitly disabled in config
	if !c.GetConfigBool("enabled") {
		return false
	}

	return c.BaseConnector.IsEnabled()
}
