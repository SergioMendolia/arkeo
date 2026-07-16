package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/cache"
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
For example, --range 180 fetches ~6 months of history.

Past days are cached in a local SQLite database. Use --reset-cache to force
re-fetching from connectors.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runTimelineCommand,
}

var (
	format      string
	maxItems    int
	week        bool
	rangeDays   int
	resetCache  bool
	noCache     bool
)

func init() {
	timelineCmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	timelineCmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum number of activities to display (0 = unlimited)")
	timelineCmd.Flags().BoolVar(&week, "week", false, "Display activities for the entire work week (Monday-Friday) containing the selected date")
	timelineCmd.Flags().IntVar(&rangeDays, "range", 0, "Fetch activities for the last N days ending at the selected date (e.g. --range 180 for ~6 months)")
	timelineCmd.Flags().BoolVar(&resetCache, "reset-cache", false, "Clear cached activities for the selected date range before fetching")
	timelineCmd.Flags().BoolVar(&noCache, "no-cache", false, "Skip cache (always fetch from connectors, don't store results)")
}

func runTimelineCommand(cmd *cobra.Command, args []string) {
	// Parse date argument or default to yesterday
	var dateStr string
	if len(args) > 0 {
		dateStr = args[0]
	} else {
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
	_ = configManager.GetConfig()

	// Initialize cache (unless --no-cache)
	var activityCache *cache.Cache
	if !noCache {
		configDir, err := configManager.GetConfigDir()
		if err == nil {
			cachePath := filepath.Join(configDir, "cache.db")
			activityCache, err = cache.New(cachePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not open cache: %v\n", err)
			}
			defer activityCache.Close()
		}
	}

	// Handle --reset-cache
	if resetCache && activityCache != nil {
		if rangeDays > 0 {
			startDate := targetDate.AddDate(0, 0, -(rangeDays - 1))
			if err := activityCache.ResetRange(startDate, targetDate); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not reset cache: %v\n", err)
			}
		} else if week {
			weekday := targetDate.Weekday()
			daysFromMonday := (int(weekday) - int(time.Monday) + 7) % 7
			monday := targetDate.AddDate(0, 0, -daysFromMonday)
			friday := monday.AddDate(0, 0, 4)
			if err := activityCache.ResetRange(monday, friday); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not reset cache: %v\n", err)
			}
		} else {
			if err := activityCache.ResetDay(targetDate); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not reset cache: %v\n", err)
			}
		}
		if !isMachineReadableFormat(format) {
			fmt.Fprintf(os.Stderr, "Cache cleared for the selected date range.\n")
		}
	}

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
	connectorNames := make([]string, 0, len(enabledConnectors))
	for name, conn := range enabledConnectors {
		utilsConnectors[name] = conn
		connectorNames = append(connectorNames, name)
	}

	isMachineReadable := isMachineReadableFormat(format)
	verbose := !isMachineReadable

	// Build the list of days to fetch
	var daysToFetch []time.Time
	if rangeDays > 0 {
		daysToFetch = make([]time.Time, rangeDays)
		for i := 0; i < rangeDays; i++ {
			daysToFetch[i] = targetDate.AddDate(0, 0, -(rangeDays - 1 - i)).Truncate(24 * time.Hour)
		}
	} else if week {
		weekday := targetDate.Weekday()
		daysFromMonday := (int(weekday) - int(time.Monday) + 7) % 7
		monday := targetDate.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)
		daysToFetch = make([]time.Time, 5)
		for i := 0; i < 5; i++ {
			daysToFetch[i] = monday.AddDate(0, 0, i)
		}
	} else {
		daysToFetch = []time.Time{targetDate.Truncate(24 * time.Hour)}
	}

	if !isMachineReadable {
		if len(daysToFetch) > 1 {
			fmt.Printf("Fetching activities for %d days (%s to %s)...\n",
				len(daysToFetch),
				daysToFetch[0].Format("2006-01-02"),
				daysToFetch[len(daysToFetch)-1].Format("2006-01-02"))
		} else {
			fmt.Printf("Fetching activities for %s...\n", targetDate.Format("January 2, 2006"))
		}
	}

	// Fetch activities for each day, using cache when available
	var allActivities []timeline.Activity
	cachedDays := 0
	fetchedDays := 0

	for _, day := range daysToFetch {
		// Check cache first (unless --no-cache)
		if activityCache != nil && activityCache.HasDay(day, connectorNames) {
			cachedActivities, err := activityCache.LoadDay(day)
			if err == nil {
				allActivities = append(allActivities, cachedActivities...)
				cachedDays++
				if verbose {
					fmt.Printf("  %s: %d activities (cached)\n",
						day.Format("2006-01-02"), len(cachedActivities))
				}
				continue
			}
		}

		// Cache miss — fetch from connectors
		executor := utils.NewParallelExecutor()
		results := executor.FetchActivitiesParallel(ctx, utilsConnectors, day)

		var dayActivities []timeline.Activity
		for _, result := range results {
			if result.Error != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "  Warning: %s: %v\n", result.Name, result.Error)
				}
				continue
			}

			if verbose {
				fmt.Printf("  %s %s: %d activities (took %v)\n",
					day.Format("2006-01-02"),
					result.Name, len(result.Activities), result.Duration.Round(time.Millisecond))
			}

			dayActivities = append(dayActivities, result.Activities...)

			// Store in cache (unless --no-cache) — store even when 0 activities
			// so HasDay knows this connector was fetched for this day.
			if activityCache != nil {
				if err := activityCache.StoreDay(day, result.Name, result.Activities); err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "  Warning: could not cache %s/%s: %v\n", day.Format("2006-01-02"), result.Name, err)
					}
				}
			}
		}

		allActivities = append(allActivities, dayActivities...)
		fetchedDays++
	}

	if !isMachineReadable {
		cacheInfo := ""
		if cachedDays > 0 {
			cacheInfo = fmt.Sprintf(" (%d from cache, %d fetched)", cachedDays, fetchedDays)
		}
		fmt.Printf("Fetched %d activities from %d connector(s) across %d days%s.\n",
			len(allActivities), len(enabledConnectors), len(daysToFetch), cacheInfo)
	}

	// Prepare display options
	opts := display.TimelineOptions{
		MaxItems: maxItems,
		Format:   format,
	}

	if len(daysToFetch) > 1 {
		opts.Dates = daysToFetch
	} else {
		opts.Dates = []time.Time{targetDate.Truncate(24 * time.Hour)}
	}

	if !isMachineReadable {
		fmt.Println()
	}

	if err := display.DisplayTimeline(allActivities, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying timeline: %v\n", err)
		os.Exit(1)
	}
}

func isMachineReadableFormat(format string) bool {
	return format == "json"
}