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
	// No timeline-specific flags - all settings are now configured via config file
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
	config := configManager.GetConfig()
	preferences := config.Preferences

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
	if preferences.ShowProgress {
		progress = ui.NewConnectorProgress(preferences.UseColors)
	}

	// Convert connectors to utils.Connector interface and start progress tracking
	utilsConnectors := make(map[string]utils.Connector)
	for name, conn := range enabledConnectors {
		utilsConnectors[name] = conn
		if progress != nil {
			progress.StartConnector(name)
		}
	}

	// Align progress bars after all connectors are added
	if progress != nil {
		progress.AlignProgressBars()
	}

	// Fetch activities from all connectors with progress tracking
	var activities []timeline.Activity
	if preferences.ParallelFetch {
		if progress != nil {
			// Use the new progress callback system
			activities = utils.FetchActivitiesParallelWithProgress(ctx, utilsConnectors, targetDate,
				func(connectorName, status string, current, total int, err error) {
					if status == "connecting" {
						// Already started in progress.StartConnector above
					} else if status == "completed" {
						if err == nil {
							// Just update the progress bar to completion, don't finish yet
							progress.UpdateConnector(connectorName, "", 1, 1)
						} else {
							progress.FinishConnector(connectorName, 0, err)
						}
					} else if status == "failed" {
						progress.FinishConnector(connectorName, 0, err)
					}
				})

			// Update final counts for all connectors
			for name := range enabledConnectors {
				connectorActivities := 0
				for _, activity := range activities {
					if activity.Source == name {
						connectorActivities++
					}
				}
				// Finish with the correct count if not already finished
				if !progress.IsConnectorFinished(name) {
					progress.FinishConnector(name, connectorActivities, nil)
				}
			}
			progress.PrintSummary()
		} else {
			activities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)
		}
	} else {
		// Sequential fetch (fallback if parallel is disabled)
		activities = utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, false)
		if progress != nil {
			for name := range enabledConnectors {
				connectorActivities := 0
				for _, activity := range activities {
					if activity.Source == name {
						connectorActivities++
					}
				}
				progress.FinishConnector(name, connectorActivities, nil)
			}
			progress.PrintSummary()
		}
	}

	tl.AddActivitiesUnsorted(activities)
	tl.EnsureSorted()

	fmt.Println()

	// Use enhanced display
	enhancedOpts := display.EnhancedTimelineOptions{
		TimelineOptions: display.TimelineOptions{
			ShowDetails: preferences.ShowDetails,
			GroupByHour: preferences.GroupByHour,
			MaxItems:    preferences.MaxItems,
			Format:      preferences.DefaultFormat,
		},
		UseColors:    preferences.UseColors,
		ShowProgress: preferences.ShowProgress,
		ShowGaps:     preferences.ShowGaps,
	}

	if err := display.DisplayEnhancedTimeline(tl, enhancedOpts); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
		os.Exit(1)
	}
}
