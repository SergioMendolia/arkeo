package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/display"
	"github.com/arkeo/arkeo/internal/timeline"
	"github.com/arkeo/arkeo/internal/utils"
)

// timelineCmd shows the timeline for a specific date
var timelineCmd = &cobra.Command{
	Use:   "timeline [date]",
	Short: "Show activity timeline for a date",
	Long: `Display the activity timeline for a specific date.
Activities are fetched from all enabled connectors and displayed in chronological order.

If no date is provided, defaults to yesterday. Date format: YYYY-MM-DD

Use --range N to fetch activities for the last N days ending at the selected date.
For example, --range 180 fetches ~6 months of history.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTimelineCommand,
}

var format string
var maxItems int
var week bool
var rangeDays int

func init() {
	timelineCmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	timelineCmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum number of activities to display (0 = unlimited)")
	timelineCmd.Flags().BoolVar(&week, "week", false, "Display activities for the entire work week (Monday-Friday) containing the selected date")
	timelineCmd.Flags().IntVar(&rangeDays, "range", 0, "Fetch activities for the last N days ending at the selected date (e.g. --range 180 for ~6 months)")
}

func runTimelineCommand(cmd *cobra.Command, args []string) {
	// Parse date argument or default to yesterday
	var dateStr string
	if len(args) > 0 {
		dateStr = args[0]
	} else {
		// Default to yesterday
		yesterday := time.Now().AddDate(0, 0, -1)
		dateStr = yesterday.Format("2006-01-02")
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid date format. Use YYYY-MM-DD: %v\n", err)
		os.Exit(1)
	}
	targetDate := parsedDate

	// Initialize configuration and connectors
	configManager, registry := initializeSystem()
	_ = configManager.GetConfig() // Config may be used for other settings in the future

	// Fetch activities from enabled connectors
	ctx := context.Background()
	enabledConnectors := getEnabledConnectors(configManager, registry)

	if len(enabledConnectors) == 0 {
		fmt.Println("No connectors are enabled. Use 'arkeo connectors list' to see available connectors.")
		fmt.Println("Enable a connector with: arkeo connectors enable <connector-name>")
		return
	}

	// Convert connectors to utils.Connector interface
	utilsConnectors := make(map[string]utils.Connector)
	for name, conn := range enabledConnectors {
		utilsConnectors[name] = conn
	}

	var allActivities []timeline.Activity
	var rangeDaysList []time.Time
	isMachineReadable := format == "json"
	verbose := !isMachineReadable

	if rangeDays > 0 {
		// Range mode: fetch activities for the last N days ending at the target date
		rangeDaysList = make([]time.Time, rangeDays)
		for i := 0; i < rangeDays; i++ {
			rangeDaysList[i] = targetDate.AddDate(0, 0, -(rangeDays - 1 - i)).Truncate(24 * time.Hour)
		}

		if !isMachineReadable {
			fmt.Printf("Fetching activities for %d days (%s to %s)...\n",
				rangeDays,
				rangeDaysList[0].Format("2006-01-02"),
				rangeDaysList[len(rangeDaysList)-1].Format("2006-01-02"))
		}

		for _, day := range rangeDaysList {
			dayActivities := utils.FetchActivitiesParallel(ctx, utilsConnectors, day, verbose)
			allActivities = append(allActivities, dayActivities...)
		}

		if !isMachineReadable {
			fmt.Printf("Fetched %d activities from %d connector(s) across %d days.\n", len(allActivities), len(enabledConnectors), len(rangeDaysList))
		}
	} else if week {
		// Calculate Monday-Friday range for the week containing the selected date
		weekday := targetDate.Weekday()
		daysFromMonday := (int(weekday) - int(time.Monday) + 7) % 7
		monday := targetDate.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

		// Generate Monday-Friday dates
		rangeDaysList = make([]time.Time, 5)
		for i := 0; i < 5; i++ {
			rangeDaysList[i] = monday.AddDate(0, 0, i)
		}

		if !isMachineReadable {
			fmt.Printf("Fetching activities for week of %s (Monday-Friday)...\n", monday.Format("January 2, 2006"))
		}

		// Fetch activities for each day in the week
		for _, day := range rangeDaysList {
			dayActivities := utils.FetchActivitiesParallel(ctx, utilsConnectors, day, verbose)
			allActivities = append(allActivities, dayActivities...)
		}

		if !isMachineReadable {
			fmt.Printf("Fetched %d activities from %d connector(s) across %d days.\n", len(allActivities), len(enabledConnectors), len(rangeDaysList))
		}
	} else {
		// Single day mode
		if !isMachineReadable {
			fmt.Printf("Fetching activities for %s...\n", targetDate.Format("January 2, 2006"))
		}
		allActivities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, verbose)
		if !isMachineReadable {
			fmt.Printf("Fetched %d activities from %d connector(s).\n", len(allActivities), len(enabledConnectors))
		}
	}

	// Prepare display options
	opts := display.TimelineOptions{
		MaxItems: maxItems, // Default is 0 (unlimited) set by flag
		Format:   format,   // Default is "table" set by flag
	}

	if rangeDays > 0 || week {
		// Range or week view: set dates to the generated list
		opts.Dates = rangeDaysList
	} else {
		// Single day: set dates to single date
		opts.Dates = []time.Time{targetDate.Truncate(24 * time.Hour)}
	}

	if !isMachineReadable {
		fmt.Println()
	}

	// Use unified display (handles both single day and week)
	if err := display.DisplayTimeline(allActivities, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
		os.Exit(1)
	}
}
