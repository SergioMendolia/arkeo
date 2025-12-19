package display

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/arkeo/arkeo/internal/display/colors"
	"github.com/arkeo/arkeo/internal/display/formatters"
	"github.com/arkeo/arkeo/internal/timeline"
)

// TimelineOptions controls how the timeline is displayed
type TimelineOptions struct {
	MaxItems int
	Format   string      // "table", "json", "csv", "taxi"
	Dates    []time.Time // Empty or single date = single day mode, multiple dates = week mode
}

// DefaultTimelineOptions returns sensible defaults for timeline display
func DefaultTimelineOptions() TimelineOptions {
	return TimelineOptions{
		MaxItems: 500,
		Format:   "table",
		Dates:    []time.Time{},
	}
}

// DisplayTimeline renders a timeline to the console
// Handles both single day and multiple days based on opts.Dates
func DisplayTimeline(activities []timeline.Activity, opts TimelineOptions) error {
	// If no dates specified, return error (should have at least one date)
	if len(opts.Dates) == 0 {
		return fmt.Errorf("at least one date must be provided")
	}

	// Single day mode
	if len(opts.Dates) == 1 {
		return displaySingleDay(activities, opts.Dates[0], opts)
	}

	// Multiple days mode (week view)
	return displayMultipleDays(activities, opts.Dates, opts)
}

// displaySingleDay handles display for a single day
func displaySingleDay(activities []timeline.Activity, date time.Time, opts TimelineOptions) error {
	// Filter activities for this date
	dateStart := date.Truncate(24 * time.Hour)
	dateEnd := dateStart.Add(24 * time.Hour)
	dayActivities := filterActivitiesByDate(activities, dateStart, dateEnd)

	// Apply max items limit
	if opts.MaxItems > 0 && len(dayActivities) > opts.MaxItems {
		dayActivities = dayActivities[:opts.MaxItems]
	}

	// Create timeline
	tl := timeline.NewTimeline(dateStart)
	tl.AddActivitiesUnsorted(dayActivities)
	tl.EnsureSorted()

	// Delegate to formatters based on format
	switch opts.Format {
	case "json":
		return formatters.DisplayJSON(tl, dayActivities)
	case "csv":
		return formatters.DisplayCSV(dayActivities)
	case "taxi":
		return formatters.DisplayTaxi(tl, dayActivities)
	default:
		return formatters.DisplayTable(tl, dayActivities, opts.Format)
	}
}

// displayMultipleDays handles display for multiple days (week view)
func displayMultipleDays(activities []timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	// Group activities by day
	activitiesByDay := groupActivitiesByDay(activities, dates)

	// Handle different formats
	switch opts.Format {
	case "json":
		return displayMultipleDaysJSON(activitiesByDay, dates, opts)
	case "csv":
		return displayMultipleDaysCSV(activitiesByDay, dates, opts)
	case "taxi":
		return displayMultipleDaysTaxi(activitiesByDay, dates, opts)
	default:
		return displayMultipleDaysTable(activitiesByDay, dates, opts)
	}
}

// displayMultipleDaysJSON outputs JSON for multiple days
func displayMultipleDaysJSON(activitiesByDay map[time.Time][]timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	// Build map with dates as keys
	result := make(map[string]*timeline.Timeline)

	for _, date := range dates {
		dayActivities := activitiesByDay[date]
		if len(dayActivities) == 0 {
			continue
		}

		// Apply max items limit per day
		if opts.MaxItems > 0 && len(dayActivities) > opts.MaxItems {
			dayActivities = dayActivities[:opts.MaxItems]
		}

		// Sort activities for this day
		sortActivitiesByTime(dayActivities)

		// Create timeline for this day
		tl := timeline.NewTimeline(date.Truncate(24 * time.Hour))
		tl.AddActivitiesUnsorted(dayActivities)
		tl.EnsureSorted()

		// Use date as key (YYYY-MM-DD format)
		dateKey := date.Format("2006-01-02")
		result[dateKey] = tl
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal timeline to JSON: %v", err)
	}

	fmt.Print(string(jsonData))
	return nil
}

// displayMultipleDaysCSV outputs CSV for multiple days
func displayMultipleDaysCSV(activitiesByDay map[time.Time][]timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	// CSV header
	fmt.Println("date,timestamp,type,source,title,description,duration,url")

	for _, date := range dates {
		dayActivities := activitiesByDay[date]
		if len(dayActivities) == 0 {
			continue
		}

		// Apply max items limit per day
		if opts.MaxItems > 0 && len(dayActivities) > opts.MaxItems {
			dayActivities = dayActivities[:opts.MaxItems]
		}

		// Sort activities for this day
		sortActivitiesByTime(dayActivities)

		// Output date line + CSV for each day
		dateStr := date.Format("2006-01-02")
		for _, activity := range dayActivities {
			duration := ""
			if activity.Duration != nil {
				duration = activity.FormatDuration()
			}

			fmt.Printf("%s,%s,%s,%s,%s,%s,%s,%s\n",
				dateStr,
				activity.Timestamp.Format("2006-01-02 15:04:05"),
				activity.Type,
				activity.Source,
				formatters.CSVEscape(activity.Title),
				formatters.CSVEscape(activity.Description),
				duration,
				activity.URL,
			)
		}
	}

	return nil
}

// displayMultipleDaysTaxi outputs taxi format for multiple days
func displayMultipleDaysTaxi(activitiesByDay map[time.Time][]timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	for i, date := range dates {
		dayActivities := activitiesByDay[date]
		if len(dayActivities) == 0 {
			continue // Skip empty days
		}

		// Apply max items limit per day
		if opts.MaxItems > 0 && len(dayActivities) > opts.MaxItems {
			dayActivities = dayActivities[:opts.MaxItems]
		}

		// Sort activities for this day
		sortActivitiesByTime(dayActivities)

		// Create timeline for this day
		tl := timeline.NewTimeline(date.Truncate(24 * time.Hour))
		tl.AddActivitiesUnsorted(dayActivities)
		tl.EnsureSorted()

		// Output this day in taxi format (formatter already includes date header)
		if err := formatters.DisplayTaxi(tl, dayActivities); err != nil {
			return err
		}

		// Add blank line between days (except after last day)
		if i < len(dates)-1 {
			fmt.Println()
		}
	}

	return nil
}

// displayMultipleDaysTable outputs table format for multiple days
func displayMultipleDaysTable(activitiesByDay map[time.Time][]timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	// Display week header
	if len(dates) > 0 {
		monday := dates[0]
		title := fmt.Sprintf("Timeline for Week of %s", monday.Format("Monday, January 2, 2006"))
		fmt.Printf("%s\n", colors.Colorize(title, colors.Bold+colors.Blue))

		totalActivities := 0
		for _, dayActivities := range activitiesByDay {
			totalActivities += len(dayActivities)
		}

		if totalActivities == 0 {
			fmt.Printf("No activities found for the week.\n")
			return nil
		}

		fmt.Printf("%s activities across %d days\n\n",
			colors.Colorize(fmt.Sprintf("%d", totalActivities), colors.Bold),
			len(dates))
	}

	// Display each day
	for _, date := range dates {
		dayActivities := activitiesByDay[date]
		if len(dayActivities) == 0 {
			continue
		}

		// Apply max items limit per day
		if opts.MaxItems > 0 && len(dayActivities) > opts.MaxItems {
			dayActivities = dayActivities[:opts.MaxItems]
		}

		// Sort activities for this day
		sortActivitiesByTime(dayActivities)

		// Create timeline for this day
		tl := timeline.NewTimeline(date.Truncate(24 * time.Hour))
		tl.AddActivitiesUnsorted(dayActivities)
		tl.EnsureSorted()

		// Display this day (formatter handles header + timeline)
		if err := formatters.DisplayTable(tl, dayActivities, opts.Format); err != nil {
			return err
		}

		// Add blank line between days
		fmt.Println()
	}

	return nil
}

// Helper functions

// groupActivitiesByDay groups activities by their date (ignoring time)
func groupActivitiesByDay(activities []timeline.Activity, dates []time.Time) map[time.Time][]timeline.Activity {
	activitiesByDay := make(map[time.Time][]timeline.Activity)

	// Initialize map with all dates
	for _, date := range dates {
		dateStart := date.Truncate(24 * time.Hour)
		activitiesByDay[dateStart] = make([]timeline.Activity, 0)
	}

	// Group activities by day
	for _, activity := range activities {
		activityDate := activity.Timestamp.Truncate(24 * time.Hour)
		// Find which date this activity belongs to
		for _, date := range dates {
			dateStart := date.Truncate(24 * time.Hour)
			if activityDate.Equal(dateStart) {
				activitiesByDay[dateStart] = append(activitiesByDay[dateStart], activity)
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

// filterActivitiesByDate filters activities for a specific date range
func filterActivitiesByDate(activities []timeline.Activity, start, end time.Time) []timeline.Activity {
	filtered := make([]timeline.Activity, 0)
	for _, activity := range activities {
		activityDate := activity.Timestamp.Truncate(24 * time.Hour)
		if !activityDate.Before(start) && activityDate.Before(end) {
			filtered = append(filtered, activity)
		}
	}
	return filtered
}

// DisplayConnectorStatus shows the status of all connectors
func DisplayConnectorStatus(connectors map[string]bool) {
	fmt.Println("Connector Status")
	fmt.Println("══════════════════════════════════")

	for name, enabled := range connectors {
		status := "❌ Disabled"
		if enabled {
			status = "✅ Enabled"
		}
		fmt.Printf("%-15s %s\n", name, status)
	}
	fmt.Println()
}
