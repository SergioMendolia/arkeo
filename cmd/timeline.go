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

If no date is provided, defaults to yesterday. Date format: YYYY-MM-DD`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTimelineCommand,
}

var format string
var maxItems int
var week bool

func init() {
	timelineCmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, csv, taxi)")
	timelineCmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum number of activities to display (0 = unlimited)")
	timelineCmd.Flags().BoolVar(&week, "week", false, "Display activities for the entire work week (Monday-Friday) containing the selected date")
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
	var weekDays []time.Time
	isMachineReadable := format == "json" || format == "csv" || format == "taxi"
	verbose := !isMachineReadable

	if week {
		// Calculate Monday-Friday range for the week containing the selected date
		weekday := targetDate.Weekday()
		daysFromMonday := (int(weekday) - int(time.Monday) + 7) % 7
		monday := targetDate.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

		// Generate Monday-Friday dates
		weekDays = make([]time.Time, 5)
		for i := 0; i < 5; i++ {
			weekDays[i] = monday.AddDate(0, 0, i)
		}

		if !isMachineReadable {
			fmt.Printf("Fetching activities for week of %s (Monday-Friday)...\n", monday.Format("January 2, 2006"))
		}

		// Fetch activities for each day in the week
		for _, day := range weekDays {
			dayActivities := utils.FetchActivitiesParallel(ctx, utilsConnectors, day, verbose)
			allActivities = append(allActivities, dayActivities...)
		}

		if !isMachineReadable {
			fmt.Printf("Fetched %d activities from %d connector(s) across %d days.\n", len(allActivities), len(enabledConnectors), len(weekDays))
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

	// Create timeline(s) and add activities
	if week {
		// For week view, we'll pass activities grouped by day to the display function
		// The display function will handle grouping
	} else {
		// Single day: create timeline with target date
		tl := timeline.NewTimeline(targetDate.Truncate(24 * time.Hour))
		tl.AddActivitiesUnsorted(allActivities)
		tl.EnsureSorted()

		if !isMachineReadable {
			fmt.Println()
		}

		// Use enhanced display
		enhancedOpts := display.EnhancedTimelineOptions{
			TimelineOptions: display.TimelineOptions{
				MaxItems: maxItems, // Default is 0 (unlimited) set by flag
				Format:   format,   // Default is "table" set by flag
			},
		}

		if err := display.DisplayEnhancedTimeline(tl, enhancedOpts); err != nil {
			fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Week view: group activities by day and display
	if !isMachineReadable {
		fmt.Println()
	}

	enhancedOpts := display.EnhancedTimelineOptions{
		TimelineOptions: display.TimelineOptions{
			MaxItems: maxItems, // Default is 0 (unlimited) set by flag
			Format:   format,   // Default is "table" set by flag
		},
		WeekMode: true,
		WeekDays: weekDays,
	}

	if err := display.DisplayEnhancedWeekTimeline(allActivities, enhancedOpts); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
		os.Exit(1)
	}
}
