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
	Format   string      // "table" or "json"
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

	// Sort before truncating so that MaxItems keeps the earliest activities
	// consistently (matches the behaviour of the multi-day paths).
	sortActivitiesByTime(dayActivities)

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
	default:
		return displayMultipleDaysTable(activitiesByDay, dates, opts)
	}
}

// displayMultipleDaysJSON outputs JSON for multiple days (without metadata)
func displayMultipleDaysJSON(activitiesByDay map[time.Time][]timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	// Build map with dates as keys, using the metadata-less projection
	type jsonActivity struct {
		ID          string                `json:"id"`
		Type        timeline.ActivityType `json:"type"`
		Title       string                `json:"title"`
		Description string                `json:"description"`
		Timestamp   time.Time             `json:"timestamp"`
		Duration    *time.Duration        `json:"duration,omitempty"`
		Source      string                `json:"source"`
		URL         string                `json:"url,omitempty"`
	}
	type jsonTimeline struct {
		Date       time.Time      `json:"date"`
		Activities []jsonActivity `json:"activities"`
	}

	result := make(map[string]*jsonTimeline)

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

		// Build metadata-less projection
		activitiesOut := make([]jsonActivity, len(dayActivities))
		for i, a := range dayActivities {
			activitiesOut[i] = jsonActivity{
				ID:          a.ID,
				Type:        a.Type,
				Title:       a.Title,
				Description: a.Description,
				Timestamp:   a.Timestamp,
				Duration:    a.Duration,
				Source:      a.Source,
				URL:         a.URL,
			}
		}

		dateKey := date.Format("2006-01-02")
		result[dateKey] = &jsonTimeline{
			Date:       date.Truncate(24 * time.Hour),
			Activities: activitiesOut,
		}
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal timeline to JSON: %v", err)
	}

	fmt.Print(string(jsonData))
	return nil
}

// displayMultipleDaysTable outputs table format for multiple days
func displayMultipleDaysTable(activitiesByDay map[time.Time][]timeline.Activity, dates []time.Time, opts TimelineOptions) error {
	// Display header
	if len(dates) > 0 {
		firstDate := dates[0]
		lastDate := dates[len(dates)-1]
		var title string
		if len(dates) == 5 && dates[0].Weekday() == time.Monday {
			title = fmt.Sprintf("Timeline for Week of %s", firstDate.Format("Monday, January 2, 2006"))
		} else {
			title = fmt.Sprintf("Timeline for %s – %s", firstDate.Format("January 2, 2006"), lastDate.Format("January 2, 2006"))
		}
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

// groupActivitiesByDay groups activities by their date (ignoring time and timezone).
// Comparison is done by date string (YYYY-MM-DD) to correctly handle activities
// in different timezones than the requested dates.
func groupActivitiesByDay(activities []timeline.Activity, dates []time.Time) map[time.Time][]timeline.Activity {
	activitiesByDay := make(map[time.Time][]timeline.Activity)

	// Build a lookup from date string (YYYY-MM-DD) to the truncated date key.
	dateByKey := make(map[string]time.Time, len(dates))
	for _, date := range dates {
		dateStart := date.Truncate(24 * time.Hour)
		key := dateStart.Format("2006-01-02")
		activitiesByDay[dateStart] = make([]timeline.Activity, 0)
		dateByKey[key] = dateStart
	}

	// Group activities by day in a single pass (O(N)).
	for _, activity := range activities {
		// Use the activity's own date (in its timezone) formatted as YYYY-MM-DD.
		activityDateKey := activity.Timestamp.Format("2006-01-02")
		if dateStart, ok := dateByKey[activityDateKey]; ok {
			activitiesByDay[dateStart] = append(activitiesByDay[dateStart], activity)
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
