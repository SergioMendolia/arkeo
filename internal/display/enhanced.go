package display

import (
	"encoding/json"
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
	WeekMode bool        // If true, display activities grouped by day for week view
	WeekDays []time.Time // List of days in the week (Monday-Friday)
}

// DisplayEnhancedTimeline renders an enhanced timeline with colors and visual improvements
func DisplayEnhancedTimeline(tl *timeline.Timeline, opts EnhancedTimelineOptions) error {
	activities := tl.Activities
	if opts.MaxItems > 0 && len(activities) > opts.MaxItems {
		activities = activities[:opts.MaxItems]
	}

	// Handle JSON and CSV formats (always return valid JSON/CSV even if empty)
	if opts.Format == "json" {
		return displayJSON(tl, activities)
	} else if opts.Format == "csv" {
		return displayCSV(activities, opts.TimelineOptions)
	}

	// Table format
	if len(tl.Activities) == 0 {
		fmt.Printf("No activities found for %s\n", colorize(tl.Date.Format("January 2, 2006"), Bold))
		return nil
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

// DisplayEnhancedWeekTimeline renders a week view with activities grouped by day
func DisplayEnhancedWeekTimeline(activities []timeline.Activity, opts EnhancedTimelineOptions) error {
	if len(opts.WeekDays) == 0 {
		return fmt.Errorf("week days must be provided for week view")
	}

	// For JSON and CSV formats, output all activities in chronological order
	if opts.Format == "json" || opts.Format == "csv" {
		// Sort all activities by timestamp
		sortActivitiesByTime(activities)

		// Apply max items limit if specified
		if opts.MaxItems > 0 && len(activities) > opts.MaxItems {
			activities = activities[:opts.MaxItems]
		}

		if opts.Format == "json" {
			// Create a week-specific JSON structure
			return displayWeekJSON(activities, opts.WeekDays)
		} else {
			return displayCSV(activities, opts.TimelineOptions)
		}
	}

	// Table format: group activities by day
	activitiesByDay := groupActivitiesByDay(activities, opts.WeekDays)

	// Display week header
	monday := opts.WeekDays[0]
	title := fmt.Sprintf("Timeline for Week of %s", monday.Format("Monday, January 2, 2006"))
	fmt.Printf("%s\n", colorize(title, Bold+Blue))

	totalActivities := 0
	for _, dayActivities := range activitiesByDay {
		totalActivities += len(dayActivities)
	}

	if totalActivities == 0 {
		fmt.Printf("No activities found for the week.\n")
		return nil
	}

	fmt.Printf("%s activities across %d days\n\n",
		colorize(fmt.Sprintf("%d", totalActivities), Bold),
		len(opts.WeekDays))

	// Display each day
	for _, day := range opts.WeekDays {
		dayActivities := activitiesByDay[day]
		if len(dayActivities) == 0 {
			continue
		}

		// Sort activities for this day
		sortActivitiesByTime(dayActivities)

		// Apply max items limit per day if specified
		if opts.MaxItems > 0 && len(dayActivities) > opts.MaxItems {
			dayActivities = dayActivities[:opts.MaxItems]
		}

		displayDayHeader(day, dayActivities)
		displayDayActivities(dayActivities, opts)
		fmt.Println()
	}

	return nil
}

// groupActivitiesByDay groups activities by their date (ignoring time)
func groupActivitiesByDay(activities []timeline.Activity, weekDays []time.Time) map[time.Time][]timeline.Activity {
	activitiesByDay := make(map[time.Time][]timeline.Activity)

	// Initialize map with all week days
	for _, day := range weekDays {
		activitiesByDay[day] = make([]timeline.Activity, 0)
	}

	// Group activities by day
	for _, activity := range activities {
		activityDate := activity.Timestamp.Truncate(24 * time.Hour)
		// Find which day in the week this activity belongs to
		for _, day := range weekDays {
			if activityDate.Equal(day) {
				activitiesByDay[day] = append(activitiesByDay[day], activity)
				break
			}
		}
	}

	return activitiesByDay
}

// sortActivitiesByTime sorts activities by timestamp
func sortActivitiesByTime(activities []timeline.Activity) {
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.Before(activities[j].Timestamp)
	})
}

// displayDayHeader shows the header for a specific day
func displayDayHeader(day time.Time, activities []timeline.Activity) {
	dayTitle := day.Format("Monday, January 2, 2006")
	fmt.Printf("%s\n", colorize(dayTitle, Bold+Cyan))
	fmt.Println(colorize(strings.Repeat("─", 60), Gray))

	if len(activities) > 0 {
		start := activities[0].Timestamp.Format("15:04")
		end := activities[len(activities)-1].Timestamp.Format("15:04")
		fmt.Printf("%s activities from %s to %s\n",
			colorize(fmt.Sprintf("%d", len(activities)), Bold),
			colorize(start, Green),
			colorize(end, Green))
		fmt.Println()
	}
}

// displayDayActivities shows activities for a specific day
func displayDayActivities(activities []timeline.Activity, opts EnhancedTimelineOptions) {
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
}

// WeekTimelineJSON represents a week timeline in JSON format
type WeekTimelineJSON struct {
	WeekStart  time.Time                      `json:"week_start"`  // Monday
	WeekEnd    time.Time                      `json:"week_end"`    // Friday
	Days       []time.Time                    `json:"days"`        // All days in the week (Monday-Friday)
	Activities []timeline.Activity            `json:"activities"`  // All activities in chronological order
	ByDay      map[string][]timeline.Activity `json:"by_day"`      // Activities grouped by day (YYYY-MM-DD format)
	TotalCount int                            `json:"total_count"` // Total number of activities
}

// displayWeekJSON outputs week timeline as JSON
func displayWeekJSON(activities []timeline.Activity, weekDays []time.Time) error {
	if len(weekDays) == 0 {
		return fmt.Errorf("week days must be provided")
	}

	monday := weekDays[0]
	friday := weekDays[len(weekDays)-1]

	// Group activities by day
	activitiesByDay := groupActivitiesByDay(activities, weekDays)

	// Convert activitiesByDay to map with string keys (YYYY-MM-DD format)
	byDayMap := make(map[string][]timeline.Activity)
	for day, dayActivities := range activitiesByDay {
		dayKey := day.Format("2006-01-02")
		// Sort activities for this day
		sortActivitiesByTime(dayActivities)
		byDayMap[dayKey] = dayActivities
	}

	// Create week timeline JSON structure
	output := WeekTimelineJSON{
		WeekStart:  monday,
		WeekEnd:    friday,
		Days:       weekDays,
		Activities: activities,
		ByDay:      byDayMap,
		TotalCount: len(activities),
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal week timeline to JSON: %v", err)
	}

	fmt.Print(string(jsonData))
	return nil
}
