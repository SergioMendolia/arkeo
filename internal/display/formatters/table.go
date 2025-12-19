package formatters

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/display/colors"
	"github.com/arkeo/arkeo/internal/timeline"
)

// DisplayTable shows the timeline in a formatted table with colors
func DisplayTable(tl *timeline.Timeline, activities []timeline.Activity, format string) error {
	if len(activities) == 0 {
		fmt.Printf("No activities found for %s\n", colors.Colorize(tl.Date.Format("January 2, 2006"), colors.Bold))
		return nil
	}

	// Display header
	DisplayHeader(tl, activities)

	// Display activities
	return DisplayChronological(activities, format)
}

// DisplayHeader shows an enhanced header with summary info
func DisplayHeader(tl *timeline.Timeline, activities []timeline.Activity) {
	title := fmt.Sprintf("Timeline for %s", tl.Date.Format("Monday, January 2, 2006"))
	fmt.Printf("%s\n", colors.Colorize(title, colors.Bold+colors.Blue))

	if len(activities) > 0 {
		start := activities[0].Timestamp.Format("15:04")
		end := activities[len(activities)-1].Timestamp.Format("15:04")
		duration := activities[len(activities)-1].Timestamp.Sub(activities[0].Timestamp)

		fmt.Printf("%s activities from %s to %s (span: %s)\n\n",
			colors.Colorize(fmt.Sprintf("%d", len(activities)), colors.Bold),
			colors.Colorize(start, colors.Green),
			colors.Colorize(end, colors.Green),
			colors.Colorize(colors.FormatDuration(duration), colors.Cyan))
	}
}

// DisplayChronological shows activities in chronological order with enhancements
func DisplayChronological(activities []timeline.Activity, format string) error {
	fmt.Printf("Activities (chronological order):\n")
	fmt.Println(colors.Colorize(strings.Repeat("═", 60), colors.Gray))

	var lastTime time.Time
	for i, activity := range activities {
		// Show time gaps (always enabled)
		if i > 0 {
			gap := activity.Timestamp.Sub(lastTime)
			if gap > 1*time.Hour {
				DisplayTimeGap(gap)
			}
		}

		DisplayActivity(activity, format, "", i == len(activities)-1)
		lastTime = activity.Timestamp
	}

	return nil
}

// DisplayActivity shows a single activity with visual enhancements
func DisplayActivity(activity timeline.Activity, format string, prefix string, isLast bool) {
	label := colors.SourceLabels[activity.Source]
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
		colors.Colorize(timeStr, colors.Bold+colors.Green),
		colors.Colorize(sourceStr, colors.DarkGray),
		colors.Colorize(title, colors.GetActivityColor(activity)))

	// Show description only in JSON format
	if format == "json" && activity.Description != "" {
		fmt.Printf("%s   Description: %s\n", prefix,
			colors.Colorize(activity.Description, colors.Gray))
	}

	// Show duration only in JSON format
	if format == "json" && activity.Duration != nil {
		fmt.Printf("%s   Duration: %s\n", prefix,
			colors.Colorize(activity.FormatDuration(), colors.Cyan))
	}

	// Show URL only in JSON format
	if format == "json" && activity.URL != "" {
		fmt.Printf("%s   URL: %s\n", prefix,
			colors.Colorize(activity.URL, colors.Blue))
	}
}

// DisplayTimeGap shows a visual indicator for time gaps
func DisplayTimeGap(gap time.Duration) {
	gapStr := fmt.Sprintf("── %s gap ──", colors.FormatDuration(gap))
	fmt.Printf("     %s\n", colors.Colorize(gapStr, colors.Gray))
}
