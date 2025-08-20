package display

import (
	"fmt"
	"sort"
	"strings"

	"github.com/autotime/autotime/internal/timeline"
)

// TimelineOptions controls how the timeline is displayed
type TimelineOptions struct {
	ShowDetails    bool
	ShowTimestamps bool
	GroupByHour    bool
	FilterType     string
	FilterSource   string
	MaxItems       int
	Format         string // "table", "json", "csv"
}

// DefaultTimelineOptions returns sensible defaults for timeline display
func DefaultTimelineOptions() TimelineOptions {
	return TimelineOptions{
		ShowDetails:    false,
		ShowTimestamps: true,
		GroupByHour:    true,
		MaxItems:       50,
		Format:         "table",
	}
}

// DisplayTimeline renders a timeline to the console
func DisplayTimeline(tl *timeline.Timeline, opts TimelineOptions) error {
	if len(tl.Activities) == 0 {
		fmt.Printf("No activities found for %s\n", tl.Date.Format("January 2, 2006"))
		return nil
	}

	activities := tl.Activities

	// Apply filters
	if opts.FilterType != "" {
		activities = filterByType(activities, opts.FilterType)
	}
	if opts.FilterSource != "" {
		activities = filterBySource(activities, opts.FilterSource)
	}

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
	default:
		return displayTable(tl, activities, opts)
	}
}

// displayTable shows the timeline in a formatted table
func displayTable(tl *timeline.Timeline, activities []timeline.Activity, opts TimelineOptions) error {
	fmt.Printf("Timeline for %s\n", tl.Date.Format("Monday, January 2, 2006"))
	fmt.Printf("Found %d activities\n\n", len(activities))

	if opts.GroupByHour {
		displayGroupedByHour(activities, opts)
	} else {
		displayChronological(activities, opts)
	}

	return nil
}

// displayGroupedByHour groups activities by hour
func displayGroupedByHour(activities []timeline.Activity, opts TimelineOptions) {
	groups := make(map[string][]timeline.Activity)

	for _, activity := range activities {
		hour := activity.Timestamp.Format("15:00")
		groups[hour] = append(groups[hour], activity)
	}

	// Sort hours
	var hours []string
	for hour := range groups {
		hours = append(hours, hour)
	}
	sort.Strings(hours)

	for _, hour := range hours {
		activities := groups[hour]
		fmt.Printf("📅 %s (%d activities)\n", hour, len(activities))
		fmt.Println(strings.Repeat("─", 50))

		for _, activity := range activities {
			displayActivity(activity, opts, "  ")
		}
		fmt.Println()
	}
}

// displayChronological shows activities in chronological order
func displayChronological(activities []timeline.Activity, opts TimelineOptions) {
	fmt.Println("Activities (chronological order):")
	fmt.Println(strings.Repeat("═", 60))

	for i, activity := range activities {
		displayActivity(activity, opts, "")

		if i < len(activities)-1 {
			fmt.Println(strings.Repeat("─", 40))
		}
	}
}

// displayActivity shows a single activity
func displayActivity(activity timeline.Activity, opts TimelineOptions, prefix string) {
	icon := getActivityIcon(activity.Type)

	// Basic info line
	if opts.ShowTimestamps {
		fmt.Printf("%s%s %s [%s] %s\n",
			prefix,
			activity.Timestamp.Format("15:04"),
			icon,
			activity.Source,
			activity.Title)
	} else {
		fmt.Printf("%s%s [%s] %s\n",
			prefix,
			icon,
			activity.Source,
			activity.Title)
	}

	// Show description if available and details requested
	if opts.ShowDetails && activity.Description != "" {
		fmt.Printf("%s   📝 %s\n", prefix, activity.Description)
	}

	// Show duration if available
	if activity.Duration != nil {
		fmt.Printf("%s   ⏱️  %s\n", prefix, activity.FormatDuration())
	}

	// Show URL if available and details requested
	if opts.ShowDetails && activity.URL != "" {
		fmt.Printf("%s   🔗 %s\n", prefix, activity.URL)
	}

	// Show tags if details requested
	if opts.ShowDetails && len(activity.Tags) > 0 {
		fmt.Printf("%s   🏷️  %s\n", prefix, strings.Join(activity.Tags, ", "))
	}
}

// getActivityIcon returns an appropriate icon for the activity type
func getActivityIcon(activityType timeline.ActivityType) string {
	switch activityType {
	case timeline.ActivityTypeGitCommit:
		return "💻"
	case timeline.ActivityTypeCalendar:
		return "📅"
	case timeline.ActivityTypeSlack:
		return "💬"
	case timeline.ActivityTypeJira:
		return "🎫"
	case timeline.ActivityTypeFile:
		return "📁"
	case timeline.ActivityTypeBrowser:
		return "🌐"
	case timeline.ActivityTypeApplication:
		return "🖥️"
	default:
		return "📋"
	}
}

// filterByType filters activities by type
func filterByType(activities []timeline.Activity, filterType string) []timeline.Activity {
	var filtered []timeline.Activity
	for _, activity := range activities {
		if string(activity.Type) == filterType {
			filtered = append(filtered, activity)
		}
	}
	return filtered
}

// filterBySource filters activities by source
func filterBySource(activities []timeline.Activity, filterSource string) []timeline.Activity {
	var filtered []timeline.Activity
	for _, activity := range activities {
		if activity.Source == filterSource {
			filtered = append(filtered, activity)
		}
	}
	return filtered
}

// displayJSON outputs timeline as JSON
func displayJSON(tl *timeline.Timeline, activities []timeline.Activity) error {
	// Note: In a full implementation, you'd use json.MarshalIndent here
	fmt.Printf("JSON output not fully implemented. Use --format=table instead.\n")
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

// DisplaySummary shows a summary of the timeline
func DisplaySummary(tl *timeline.Timeline) {
	summary := tl.GetSummary()

	fmt.Printf("Timeline Summary for %s\n", summary.Date.Format("January 2, 2006"))
	fmt.Println(strings.Repeat("═", 40))

	fmt.Printf("📊 Total Activities: %d\n", summary.TotalActivities)

	if summary.TotalActivities > 0 {
		fmt.Printf("⏰ Time Range: %s - %s\n",
			summary.TimeRange.Start.Format("15:04"),
			summary.TimeRange.End.Format("15:04"))

		fmt.Println("\n📈 By Activity Type:")
		for actType, count := range summary.ByType {
			icon := getActivityIcon(actType)
			fmt.Printf("   %s %-15s %d\n", icon, actType, count)
		}

		fmt.Println("\n🔗 By Source:")
		for source, count := range summary.BySource {
			fmt.Printf("   📋 %-15s %d\n", source, count)
		}
	}

	fmt.Println()
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
