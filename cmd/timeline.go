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

func init() {
	timelineCmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, csv)")
	timelineCmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum number of activities to display (0 = unlimited)")
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

	// Convert connectors to utils.Connector interface
	utilsConnectors := make(map[string]utils.Connector)
	for name, conn := range enabledConnectors {
		utilsConnectors[name] = conn
	}

	// Fetch activities from all connectors (always in parallel)
	activities := utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)

	// Print completion message
	fmt.Printf("Fetched %d activities from %d connector(s).\n", len(activities), len(enabledConnectors))

	tl.AddActivitiesUnsorted(activities)
	tl.EnsureSorted()

	fmt.Println()

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
}
