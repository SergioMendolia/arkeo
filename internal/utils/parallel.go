package utils

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// Connector defines the interface for activity connectors that can be executed in parallel
type Connector interface {
	GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error)
}

// ConnectorResult represents the result of a connector execution
type ConnectorResult struct {
	Name       string
	Activities []timeline.Activity
	Error      error
	Duration   time.Duration
}

// ParallelExecutor handles parallel execution of multiple connectors
type ParallelExecutor struct {
	maxConcurrency int
	timeout        time.Duration
}

// NewParallelExecutor creates a new parallel executor with default settings
func NewParallelExecutor() *ParallelExecutor {
	return &ParallelExecutor{
		maxConcurrency: 10,              // Default max concurrent connectors
		timeout:        5 * time.Minute, // Default timeout per connector
	}
}

// NewParallelExecutorWithConfig creates a new parallel executor with custom settings
func NewParallelExecutorWithConfig(maxConcurrency int, timeout time.Duration) *ParallelExecutor {
	return &ParallelExecutor{
		maxConcurrency: maxConcurrency,
		timeout:        timeout,
	}
}

// FetchActivitiesParallel fetches activities from multiple connectors in parallel
func (pe *ParallelExecutor) FetchActivitiesParallel(ctx context.Context, connectorMap map[string]Connector, date time.Time) []ConnectorResult {
	// Channel to control concurrency
	semaphore := make(chan struct{}, pe.maxConcurrency)

	// Channel to collect results
	results := make(chan ConnectorResult, len(connectorMap))

	var wg sync.WaitGroup

	// Launch goroutines for each connector
	for name, connector := range connectorMap {
		wg.Add(1)
		go func(connectorName string, conn Connector) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create timeout context for this connector
			connectorCtx, cancel := context.WithTimeout(ctx, pe.timeout)
			defer cancel()

			start := time.Now()
			activities, err := conn.GetActivities(connectorCtx, date)
			duration := time.Since(start)

			results <- ConnectorResult{
				Name:       connectorName,
				Activities: activities,
				Error:      err,
				Duration:   duration,
			}
		}(name, connector)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect all results
	var allResults []ConnectorResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

// FetchAndCombineActivities fetches activities from multiple connectors and combines them
// This is a convenience function that returns just the activities and handles error reporting
func (pe *ParallelExecutor) FetchAndCombineActivities(ctx context.Context, connectorMap map[string]Connector, date time.Time, verbose bool) []timeline.Activity {
	results := pe.FetchActivitiesParallel(ctx, connectorMap, date)

	// Pre-allocate with estimated capacity
	totalActivities := make([]timeline.Activity, 0, len(results)*10)

	for _, result := range results {
		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error fetching from %s: %v\n", result.Name, result.Error)
			continue
		}

		if verbose {
			fmt.Printf("✓ %s: Found %d activities (took %v)\n",
				result.Name, len(result.Activities), result.Duration.Round(time.Millisecond))
		}

		totalActivities = append(totalActivities, result.Activities...)
	}

	return totalActivities
}

// FetchWithStats fetches activities and returns both activities and execution statistics
func (pe *ParallelExecutor) FetchWithStats(ctx context.Context, connectorMap map[string]Connector, date time.Time) ([]timeline.Activity, ExecutionStats) {
	start := time.Now()
	results := pe.FetchActivitiesParallel(ctx, connectorMap, date)
	totalDuration := time.Since(start)

	stats := ExecutionStats{
		TotalConnectors:   len(connectorMap),
		SuccessfulFetches: 0,
		FailedFetches:     0,
		TotalActivities:   0,
		TotalDuration:     totalDuration,
		ConnectorTimings:  make(map[string]time.Duration),
		ConnectorErrors:   make(map[string]error),
	}

	totalActivities := make([]timeline.Activity, 0, len(results)*10)

	for _, result := range results {
		stats.ConnectorTimings[result.Name] = result.Duration

		if result.Error != nil {
			stats.FailedFetches++
			stats.ConnectorErrors[result.Name] = result.Error
		} else {
			stats.SuccessfulFetches++
			stats.TotalActivities += len(result.Activities)
			totalActivities = append(totalActivities, result.Activities...)
		}
	}

	return totalActivities, stats
}

// ExecutionStats contains statistics about parallel connector execution
type ExecutionStats struct {
	TotalConnectors   int
	SuccessfulFetches int
	FailedFetches     int
	TotalActivities   int
	TotalDuration     time.Duration
	ConnectorTimings  map[string]time.Duration
	ConnectorErrors   map[string]error
}

// String returns a formatted string representation of the execution stats
func (s ExecutionStats) String() string {
	successRate := float64(s.SuccessfulFetches) / float64(s.TotalConnectors) * 100

	return fmt.Sprintf(
		"Execution Stats: %d/%d connectors successful (%.1f%%), "+
			"%d total activities, completed in %v",
		s.SuccessfulFetches,
		s.TotalConnectors,
		successRate,
		s.TotalActivities,
		s.TotalDuration.Round(time.Millisecond),
	)
}

// GetSlowestConnector returns the name and duration of the slowest connector
func (s ExecutionStats) GetSlowestConnector() (string, time.Duration) {
	var slowestName string
	var slowestDuration time.Duration

	for name, duration := range s.ConnectorTimings {
		if duration > slowestDuration {
			slowestName = name
			slowestDuration = duration
		}
	}

	return slowestName, slowestDuration
}

// GetFastestConnector returns the name and duration of the fastest connector
func (s ExecutionStats) GetFastestConnector() (string, time.Duration) {
	var fastestName string
	var fastestDuration time.Duration = time.Hour // Initialize with large value

	for name, duration := range s.ConnectorTimings {
		if duration < fastestDuration {
			fastestName = name
			fastestDuration = duration
		}
	}

	return fastestName, fastestDuration
}

// HasErrors returns true if any connector failed
func (s ExecutionStats) HasErrors() bool {
	return s.FailedFetches > 0
}

// DefaultParallelExecutor is a package-level instance for convenience
var DefaultParallelExecutor = NewParallelExecutor()

// FetchActivitiesParallel is a convenience function that uses the default parallel executor
func FetchActivitiesParallel(ctx context.Context, connectorMap map[string]Connector, date time.Time, verbose bool) []timeline.Activity {
	return DefaultParallelExecutor.FetchAndCombineActivities(ctx, connectorMap, date, verbose)
}

// FetchActivitiesWithStats is a convenience function that uses the default executor and returns stats
func FetchActivitiesWithStats(ctx context.Context, connectorMap map[string]Connector, date time.Time) ([]timeline.Activity, ExecutionStats) {
	return DefaultParallelExecutor.FetchWithStats(ctx, connectorMap, date)
}
