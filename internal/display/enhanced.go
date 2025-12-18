package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// ANSI color codes
const (
	Reset    = "\033[0m"
	Bold     = "\033[1m"
	Red      = "\033[31m"
	Green    = "\033[32m"
	Yellow   = "\033[33m"
	Blue     = "\033[34m"
	Magenta  = "\033[35m"
	Cyan     = "\033[36m"
	Gray     = "\033[37m"
	DarkGray = "\033[90m"
	White    = "\033[97m"
)

// Color mapping for different activity types and sources
var typeColors = map[timeline.ActivityType]string{
	timeline.ActivityTypeGitCommit:   Green,
	timeline.ActivityTypeCalendar:    Blue,
	timeline.ActivityTypeSlack:       Magenta,
	timeline.ActivityTypeJira:        Yellow,
	timeline.ActivityTypeYouTrack:    Cyan,
	timeline.ActivityTypeSystem:      Gray,
	timeline.ActivityTypeCustom:      White,
	timeline.ActivityTypeFile:        Yellow,
	timeline.ActivityTypeBrowser:     Blue,
	timeline.ActivityTypeApplication: Cyan,
}

var sourceLabels = map[string]string{
	"github":       "GH",
	"gitlab":       "GL",
	"calendar":     "CAL",
	"youtrack":     "YT",
	"slack":        "SLK",
	"jira":         "JRA",
	"macos_system": "MAC",
	"system":       "SYS",
	"file":         "FILE",
	"browser":      "WEB",
}

// EnhancedTimelineOptions extends TimelineOptions with visual enhancements
type EnhancedTimelineOptions struct {
	TimelineOptions
}

// DisplayEnhancedTimeline renders an enhanced timeline with colors and visual improvements
func DisplayEnhancedTimeline(tl *timeline.Timeline, opts EnhancedTimelineOptions) error {
	if len(tl.Activities) == 0 {
		fmt.Printf("No activities found for %s\n", colorize(tl.Date.Format("January 2, 2006"), Bold))
		return nil
	}

	activities := tl.Activities
	if opts.MaxItems > 0 && len(activities) > opts.MaxItems {
		activities = activities[:opts.MaxItems]
	}

	// Display header
	displayEnhancedHeader(tl, activities, opts)

	// Display timeline based on format
	switch opts.Format {
	default:
		return displayEnhancedChronological(activities, opts)
	}
}

// displayEnhancedHeader shows an enhanced header with summary info
func displayEnhancedHeader(tl *timeline.Timeline, activities []timeline.Activity, opts EnhancedTimelineOptions) {
	title := fmt.Sprintf("Timeline for %s", tl.Date.Format("Monday, January 2, 2006"))
	fmt.Printf("%s\n", colorize(title, Bold+Blue))

	if len(activities) > 0 {
		start := activities[0].Timestamp.Format("15:04")
		end := activities[len(activities)-1].Timestamp.Format("15:04")
		duration := activities[len(activities)-1].Timestamp.Sub(activities[0].Timestamp)

		fmt.Printf("%s activities from %s to %s (span: %s)\n\n",
			colorize(fmt.Sprintf("%d", len(activities)), Bold),
			colorize(start, Green),
			colorize(end, Green),
			colorize(formatDuration(duration), Cyan))
	}
}

// displayEnhancedChronological shows activities in chronological order with enhancements
func displayEnhancedChronological(activities []timeline.Activity, opts EnhancedTimelineOptions) error {
	fmt.Printf("Activities (chronological order):\n")
	fmt.Println(colorize(strings.Repeat("═", 60), Gray))

	var lastTime time.Time
	for i, activity := range activities {
		// Show time gaps (always enabled)
		if i > 0 {
			gap := activity.Timestamp.Sub(lastTime)
			if gap > 1*time.Hour {
				displayTimeGap(gap, opts)
			}
		}

		displayEnhancedActivity(activity, opts, "", i == len(activities)-1)
		lastTime = activity.Timestamp
	}

	return nil
}

// displayEnhancedActivity shows a single activity with visual enhancements
func displayEnhancedActivity(activity timeline.Activity, opts EnhancedTimelineOptions, prefix string, isLast bool) {
	label := sourceLabels[activity.Source]
	if label == "" {
		label = "SRC"
	}

	// Time and connector
	timeStr := activity.Timestamp.Format("15:04")
	sourceStr := fmt.Sprintf("%s:", activity.Source)

	// Build title with duration if available
	title := activity.Title
	if activity.Duration != nil {
		title = fmt.Sprintf("%s (%s)", activity.Title, activity.FormatDuration())
	}

	// Full format with details
	fmt.Printf("%s%s %s %s\n",
		prefix,
		colorize(timeStr, Bold+Green),
		colorize(sourceStr, DarkGray),
		colorize(title, getActivityColor(activity)))

	// Show description only in JSON format
	if opts.Format == "json" && activity.Description != "" {
		fmt.Printf("%s   Description: %s\n", prefix,
			colorize(activity.Description, Gray))
	}

	// Show duration only in JSON format
	if opts.Format == "json" && activity.Duration != nil {
		fmt.Printf("%s   Duration: %s\n", prefix,
			colorize(activity.FormatDuration(), Cyan))
	}

	// Show URL only in JSON format
	if opts.Format == "json" && activity.URL != "" {
		fmt.Printf("%s   URL: %s\n", prefix,
			colorize(activity.URL, Blue))
	}
}

// displayTimeGap shows a visual indicator for time gaps
func displayTimeGap(gap time.Duration, opts EnhancedTimelineOptions) {
	gapStr := fmt.Sprintf("── %s gap ──", formatDuration(gap))
	fmt.Printf("     %s\n", colorize(gapStr, Gray))
}

// Helper functions

// colorize adds color codes (colors are always enabled)
func colorize(text, color string) string {
	if color == "" {
		return text
	}
	return color + text + Reset
}

// getActivityColor returns the appropriate color for an activity
func getActivityColor(activity timeline.Activity) string {
	if color, exists := typeColors[activity.Type]; exists {
		return color
	}
	return White
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}
