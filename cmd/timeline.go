package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/display"
	"github.com/arkeo/arkeo/internal/timeline"
	"github.com/arkeo/arkeo/internal/ui"
	"github.com/arkeo/arkeo/internal/utils"
)

// timelineCmd shows the timeline for a specific date
var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Show activity timeline for a date",
	Long: `Display the activity timeline for a specific date.
Activities are fetched from all enabled connectors and displayed in chronological order.`,
	Run: runTimelineCommand,
}

func init() {
	// Timeline flags
	timelineCmd.Flags().BoolVar(&showDetail, "details", false, "show detailed information for each activity")
	timelineCmd.Flags().IntVar(&maxItems, "max", 500, "maximum number of activities to show")
	timelineCmd.Flags().BoolVar(&groupByHour, "group", false, "group activities by hour")

	// Enhanced timeline flags
	timelineCmd.Flags().BoolVar(&useColors, "colors", true, "use colors in output")
	timelineCmd.Flags().BoolVar(&showTimeline, "visual", true, "show visual timeline view")
	timelineCmd.Flags().BoolVar(&showProgress, "progress", true, "show progress indicators")
	timelineCmd.Flags().BoolVar(&showGaps, "gaps", true, "highlight time gaps")
}

func runTimelineCommand(cmd *cobra.Command, args []string) {
	// Parse date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid date format. Use YYYY-MM-DD: %v\n", err)
		os.Exit(1)
	}
	targetDate := parsedDate

	// Initialize configuration and connectors
	configManager, registry := initializeSystem()

	// Create timeline
	tl := timeline.NewTimeline(targetDate.Truncate(24 * time.Hour))

	// Fetch activities from enabled connectors
	ctx := context.Background()
	enabledConnectors := getEnabledConnectors(configManager, registry)

	if len(enabledConnectors) == 0 {
		fmt.Println("No connectors are enabled. Use 'arkeo connectors list' to see available connectors.")
		fmt.Println("Enable a connector with: arkeo connectors enable <connector-name>")
		return
	}

	fmt.Printf("Fetching activities for %s...\n", targetDate.Format("January 2, 2006"))

	// Initialize progress tracker if enabled
	var progress *ui.ConnectorProgress
	if showProgress {
		progress = ui.NewConnectorProgress(useColors)
	}

	// Convert connectors to utils.Connector interface and start progress tracking
	utilsConnectors := make(map[string]utils.Connector)
	for name, conn := range enabledConnectors {
		utilsConnectors[name] = conn
		if progress != nil {
			progress.StartConnector(name)
		}
	}

	// Fetch activities from all connectors with progress tracking
	var activities []timeline.Activity
	if progress != nil {
		activities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)
		for name := range enabledConnectors {
			// Simulate progress completion
			connectorActivities := 0
			for _, activity := range activities {
				if activity.Source == name {
					connectorActivities++
				}
			}
			progress.FinishConnector(name, connectorActivities, nil)
		}
		progress.PrintSummary()
	} else {
		activities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)
	}

	tl.AddActivitiesUnsorted(activities)
	tl.EnsureSorted()

	fmt.Println()

	// Use enhanced display
	enhancedOpts := display.EnhancedTimelineOptions{
		TimelineOptions: display.TimelineOptions{
			ShowDetails:    showDetail,
			ShowTimestamps: true,
			GroupByHour:    groupByHour,
			MaxItems:       maxItems,
			Format:         format,
		},
		UseColors:    useColors,
		ShowTimeline: showTimeline,
		ShowProgress: showProgress,
		ShowGaps:     showGaps,
	}

	if err := display.DisplayEnhancedTimeline(tl, enhancedOpts); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
		os.Exit(1)
	}
}