package display

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arkeo/arkeo/internal/timeline"
)

// TimelineOptions controls how the timeline is displayed
type TimelineOptions struct {
	MaxItems int
	Format   string // "table", "json", "csv"
}

// DefaultTimelineOptions returns sensible defaults for timeline display
func DefaultTimelineOptions() TimelineOptions {
	return TimelineOptions{
		MaxItems: 500,
		Format:   "table",
	}
}

// DisplayTimeline renders a timeline to the console
func DisplayTimeline(tl *timeline.Timeline, opts TimelineOptions) error {
	if len(tl.Activities) == 0 {
		fmt.Printf("No activities found for %s\n", tl.Date.Format("January 2, 2006"))
		return nil
	}

	activities := tl.Activities

	// Limit number of items
	if opts.MaxItems > 0 && len(activities) > opts.MaxItems {
		activities = activities[:opts.MaxItems]
	}

	// Display based on format
	switch opts.Format {
	case "json":
		return displayJSON(tl, activities)
	case "csv":
		return displayCSV(activities, opts)
	case "taxi":
		return displayTaxi(tl, activities)
	default:
		return displayTable(tl, activities, opts)
	}
}

// displayTable shows the timeline in a formatted table
func displayTable(tl *timeline.Timeline, activities []timeline.Activity, opts TimelineOptions) error {
	fmt.Printf("Timeline for %s\n", tl.Date.Format("Monday, January 2, 2006"))
	fmt.Printf("Found %d activities\n\n", len(activities))

	displayChronological(activities, opts)

	return nil
}

// displayChronological shows activities in chronological order
func displayChronological(activities []timeline.Activity, opts TimelineOptions) {
	fmt.Println("Activities (chronological order):")
	fmt.Println(strings.Repeat("═", 60))

	for _, activity := range activities {
		displayActivity(activity, opts, "")

	}
}

// displayActivity shows a single activity
func displayActivity(activity timeline.Activity, opts TimelineOptions, prefix string) {

	// Build title with duration if available
	title := activity.Title
	if activity.Duration != nil {
		title = fmt.Sprintf("%s (%s)", activity.Title, activity.FormatDuration())
	}

	// Basic info line with timestamps (always shown)
	fmt.Printf("%s%s [\033[90m%s\033[0m] %s\n",
		prefix,
		activity.Timestamp.Format("15:04"),
		activity.Source,
		title)

	// Show description only in JSON format
	if opts.Format == "json" && activity.Description != "" {
		fmt.Printf("%s   📝 %s\n", prefix, activity.Description)
	}

	// Show duration only in JSON format
	if opts.Format == "json" && activity.Duration != nil {
		fmt.Printf("%s   ⏱️  %s\n", prefix, activity.FormatDuration())
	}

	// Show URL only in JSON format
	if opts.Format == "json" && activity.URL != "" {
		fmt.Printf("%s   🔗 %s\n", prefix, activity.URL)
	}

}

// displayJSON outputs timeline as JSON
func displayJSON(tl *timeline.Timeline, activities []timeline.Activity) error {
	// Create a timeline copy with the limited activities
	output := &timeline.Timeline{
		Date:       tl.Date,
		Activities: activities,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal timeline to JSON: %v", err)
	}

	fmt.Print(string(jsonData))
	return nil
}

// displayCSV outputs timeline as CSV
func displayCSV(activities []timeline.Activity, opts TimelineOptions) error {
	// CSV header
	fmt.Println("timestamp,type,source,title,description,duration,url")

	for _, activity := range activities {
		duration := ""
		if activity.Duration != nil {
			duration = activity.FormatDuration()
		}

		fmt.Printf("%s,%s,%s,%s,%s,%s,%s\n",
			activity.Timestamp.Format("2006-01-02 15:04:05"),
			activity.Type,
			activity.Source,
			csvEscape(activity.Title),
			csvEscape(activity.Description),
			duration,
			activity.URL,
		)
	}

	return nil
}

// csvEscape escapes CSV fields
func csvEscape(field string) string {
	if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
		return "\"" + strings.ReplaceAll(field, "\"", "\"\"") + "\""
	}
	return field
}

// DisplayConnectorStatus shows the status of all connectors
func DisplayConnectorStatus(connectors map[string]bool) {
	fmt.Println("Connector Status")
	fmt.Println(strings.Repeat("═", 30))

	for name, enabled := range connectors {
		status := "❌ Disabled"
		if enabled {
			status = "✅ Enabled"
		}
		fmt.Printf("%-15s %s\n", name, status)
	}
	fmt.Println()
}
