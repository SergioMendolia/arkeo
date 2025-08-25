package display

import (
	"fmt"
	"sort"
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
	UseColors    bool
	ShowProgress bool // Progress indicators
	ShowGaps     bool // Highlight time gaps
}

// DefaultEnhancedOptions returns sensible defaults
func DefaultEnhancedOptions() EnhancedTimelineOptions {
	return EnhancedTimelineOptions{
		TimelineOptions: DefaultTimelineOptions(),
		UseColors:       true,
		ShowProgress:    true,
		ShowGaps:        true,
	}
}

// DisplayEnhancedTimeline renders an enhanced timeline with colors and visual improvements
func DisplayEnhancedTimeline(tl *timeline.Timeline, opts EnhancedTimelineOptions) error {
	if len(tl.Activities) == 0 {
		fmt.Printf("No activities found for %s\n", colorize(tl.Date.Format("January 2, 2006"), Bold, opts.UseColors))
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
		if opts.GroupByHour {
			return displayEnhancedGroupedByHour(activities, opts)
		}
		return displayEnhancedChronological(activities, opts)
	}
}

// displayEnhancedHeader shows an enhanced header with summary info
func displayEnhancedHeader(tl *timeline.Timeline, activities []timeline.Activity, opts EnhancedTimelineOptions) {
	title := fmt.Sprintf("Timeline for %s", tl.Date.Format("Monday, January 2, 2006"))
	fmt.Printf("%s\n", colorize(title, Bold+Blue, opts.UseColors))

	if len(activities) > 0 {
		start := activities[0].Timestamp.Format("15:04")
		end := activities[len(activities)-1].Timestamp.Format("15:04")
		duration := activities[len(activities)-1].Timestamp.Sub(activities[0].Timestamp)

		fmt.Printf("%s activities from %s to %s (span: %s)\n\n",
			colorize(fmt.Sprintf("%d", len(activities)), Bold, opts.UseColors),
			colorize(start, Green, opts.UseColors),
			colorize(end, Green, opts.UseColors),
			colorize(formatDuration(duration), Cyan, opts.UseColors))
	}
}

// displayEnhancedChronological shows activities in chronological order with enhancements
func displayEnhancedChronological(activities []timeline.Activity, opts EnhancedTimelineOptions) error {
	fmt.Printf("Activities (chronological order):\n")
	fmt.Println(colorize(strings.Repeat("═", 60), Gray, opts.UseColors))

	var lastTime time.Time
	for i, activity := range activities {
		// Show time gaps
		if opts.ShowGaps && i > 0 {
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

// displayEnhancedGroupedByHour groups activities by hour with visual enhancements
func displayEnhancedGroupedByHour(activities []timeline.Activity, opts EnhancedTimelineOptions) error {
	groups := make(map[string][]timeline.Activity)

	for _, activity := range activities {
		hour := activity.Timestamp.Format("15:00")
		groups[hour] = append(groups[hour], activity)
	}

	var hours []string
	for hour := range groups {
		hours = append(hours, hour)
	}
	sort.Strings(hours)

	for i, hour := range hours {
		hourActivities := groups[hour]
		fmt.Printf("%s %s\n",
			colorize(hour, Bold+Blue, opts.UseColors),
			colorize(fmt.Sprintf("(%d activities)", len(hourActivities)), Gray, opts.UseColors))

		fmt.Println(colorize(strings.Repeat("─", 50), Gray, opts.UseColors))

		for j, activity := range hourActivities {
			displayEnhancedActivity(activity, opts, "  ", j == len(hourActivities)-1)
		}

		if i < len(hours)-1 {
			fmt.Println()
		}
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
		colorize(timeStr, Bold+Green, opts.UseColors),
		colorize(sourceStr, DarkGray, opts.UseColors),
		colorize(title, getActivityColor(activity), opts.UseColors))

	// Show description if available and details requested
	if opts.ShowDetails && activity.Description != "" {
		fmt.Printf("%s   Description: %s\n", prefix,
			colorize(activity.Description, Gray, opts.UseColors))
	}

	// Show duration if available
	if opts.ShowDetails && activity.Duration != nil {
		fmt.Printf("%s   Duration: %s\n", prefix,
			colorize(activity.FormatDuration(), Cyan, opts.UseColors))
	}

	// Show URL if available
	if opts.ShowDetails && activity.URL != "" {
		fmt.Printf("%s   URL: %s\n", prefix,
			colorize(activity.URL, Blue, opts.UseColors))
	}
}

// displayTimeGap shows a visual indicator for time gaps
func displayTimeGap(gap time.Duration, opts EnhancedTimelineOptions) {
	gapStr := fmt.Sprintf("── %s gap ──", formatDuration(gap))
	fmt.Printf("     %s\n", colorize(gapStr, Gray, opts.UseColors))
}

// Helper functions

// colorize adds color codes if colors are enabled
func colorize(text, color string, useColors bool) string {
	if !useColors || color == "" {
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

// createProgressBar creates a visual progress bar
func createProgressBar(percentage, width int, useColors bool) string {
	filled := percentage * width / 100
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	if useColors {
		if percentage >= 50 {
			return Green + bar + Reset
		} else if percentage >= 25 {
			return Yellow + bar + Reset
		}
		return Red + bar + Reset
	}

	return bar
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

// DisplayEnhancedSummary shows an enhanced summary with visual elements
func DisplayEnhancedSummary(tl *timeline.Timeline, opts EnhancedTimelineOptions) {
	summary := tl.GetSummary()

	fmt.Printf("%s\n",
		colorize(fmt.Sprintf("Timeline Summary for %s", summary.Date.Format("January 2, 2006")), Bold+Blue, opts.UseColors))
	fmt.Println(colorize(strings.Repeat("═", 50), Gray, opts.UseColors))

	fmt.Printf("Total Activities: %s\n",
		colorize(fmt.Sprintf("%d", summary.TotalActivities), Bold+Green, opts.UseColors))

	if summary.TotalActivities > 0 {
		fmt.Printf("Active Time: %s - %s\n",
			colorize(summary.TimeRange.Start.Format("15:04"), Green, opts.UseColors),
			colorize(summary.TimeRange.End.Format("15:04"), Green, opts.UseColors))

		duration := summary.TimeRange.End.Sub(summary.TimeRange.Start)
		fmt.Printf("Time Span: %s\n",
			colorize(formatDuration(duration), Cyan, opts.UseColors))

	}

	fmt.Println()
}
