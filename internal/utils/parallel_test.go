package utils

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// MockSlowConnector simulates a slow connector for testing parallel execution
type MockSlowConnector struct {
	name       string
	delay      time.Duration
	activities []timeline.Activity
	shouldFail bool
	callCount  int
	mutex      sync.Mutex
}

func NewMockSlowConnector(name string, delay time.Duration) *MockSlowConnector {
	return &MockSlowConnector{
		name:       name,
		delay:      delay,
		activities: []timeline.Activity{},
	}
}

func (m *MockSlowConnector) SetActivities(activities []timeline.Activity) {
	m.activities = activities
}

func (m *MockSlowConnector) SetShouldFail(shouldFail bool) {
	m.shouldFail = shouldFail
}

func (m *MockSlowConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	m.mutex.Lock()
	m.callCount++
	m.mutex.Unlock()

	// Simulate work with delay
	select {
	case <-time.After(m.delay):
		if m.shouldFail {
			return nil, errors.New("mock connector error")
		}
		return m.activities, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *MockSlowConnector) GetCallCount() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.callCount
}

func TestParallelExecutor_FetchActivitiesParallel(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create test connectors with different delays
	connector1 := NewMockSlowConnector("fast", 10*time.Millisecond)
	connector1.SetActivities([]timeline.Activity{
		{ID: "1", Title: "Fast Activity", Timestamp: date},
	})

	connector2 := NewMockSlowConnector("medium", 50*time.Millisecond)
	connector2.SetActivities([]timeline.Activity{
		{ID: "2", Title: "Medium Activity", Timestamp: date},
	})

	connector3 := NewMockSlowConnector("slow", 100*time.Millisecond)
	connector3.SetActivities([]timeline.Activity{
		{ID: "3", Title: "Slow Activity", Timestamp: date},
	})

	connectorMap := map[string]Connector{
		"fast":   connector1,
		"medium": connector2,
		"slow":   connector3,
	}

	start := time.Now()
	results := executor.FetchActivitiesParallel(ctx, connectorMap, date)
	duration := time.Since(start)

	// Should complete faster than sequential execution (which would take ~160ms)
	if duration > 150*time.Millisecond {
		t.Errorf("Parallel execution took too long: %v", duration)
	}

	// Should have results from all connectors
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Verify all connectors were called
	if connector1.GetCallCount() != 1 || connector2.GetCallCount() != 1 || connector3.GetCallCount() != 1 {
		t.Error("Not all connectors were called exactly once")
	}

	// Verify results contain activities
	totalActivities := 0
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Unexpected error from connector %s: %v", result.Name, result.Error)
		}
		totalActivities += len(result.Activities)
	}

	if totalActivities != 3 {
		t.Errorf("Expected 3 total activities, got %d", totalActivities)
	}
}

func TestParallelExecutor_FetchWithErrors(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create connectors where some fail
	successConnector := NewMockSlowConnector("success", 10*time.Millisecond)
	successConnector.SetActivities([]timeline.Activity{
		{ID: "1", Title: "Success Activity", Timestamp: date},
	})

	failConnector := NewMockSlowConnector("fail", 20*time.Millisecond)
	failConnector.SetShouldFail(true)

	connectorMap := map[string]Connector{
		"success": successConnector,
		"fail":    failConnector,
	}

	results := executor.FetchActivitiesParallel(ctx, connectorMap, date)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	successCount := 0
	errorCount := 0

	for _, result := range results {
		if result.Error != nil {
			errorCount++
			if result.Name != "fail" {
				t.Errorf("Unexpected error from connector %s", result.Name)
			}
		} else {
			successCount++
			if result.Name != "success" {
				t.Errorf("Expected success from 'success' connector, got from %s", result.Name)
			}
		}
	}

	if successCount != 1 || errorCount != 1 {
		t.Errorf("Expected 1 success and 1 error, got %d success and %d errors", successCount, errorCount)
	}
}

func TestParallelExecutor_FetchAndCombineActivities(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	connector1 := NewMockSlowConnector("connector1", 10*time.Millisecond)
	connector1.SetActivities([]timeline.Activity{
		{ID: "1", Title: "Activity 1", Timestamp: date},
		{ID: "2", Title: "Activity 2", Timestamp: date},
	})

	connector2 := NewMockSlowConnector("connector2", 20*time.Millisecond)
	connector2.SetActivities([]timeline.Activity{
		{ID: "3", Title: "Activity 3", Timestamp: date},
	})

	connectorMap := map[string]Connector{
		"connector1": connector1,
		"connector2": connector2,
	}

	activities := executor.FetchAndCombineActivities(ctx, connectorMap, date, false)

	if len(activities) != 3 {
		t.Errorf("Expected 3 activities, got %d", len(activities))
	}

	// Verify activity IDs
	expectedIDs := map[string]bool{"1": false, "2": false, "3": false}
	for _, activity := range activities {
		if _, exists := expectedIDs[activity.ID]; exists {
			expectedIDs[activity.ID] = true
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("Activity with ID %s not found", id)
		}
	}
}

func TestParallelExecutor_FetchWithStats(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	connector1 := NewMockSlowConnector("success", 10*time.Millisecond)
	connector1.SetActivities([]timeline.Activity{
		{ID: "1", Title: "Activity 1", Timestamp: date},
	})

	connector2 := NewMockSlowConnector("fail", 20*time.Millisecond)
	connector2.SetShouldFail(true)

	connectorMap := map[string]Connector{
		"success": connector1,
		"fail":    connector2,
	}

	activities, stats := executor.FetchWithStats(ctx, connectorMap, date)

	// Verify activities
	if len(activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(activities))
	}

	// Verify stats
	if stats.TotalConnectors != 2 {
		t.Errorf("Expected 2 total connectors, got %d", stats.TotalConnectors)
	}

	if stats.SuccessfulFetches != 1 {
		t.Errorf("Expected 1 successful fetch, got %d", stats.SuccessfulFetches)
	}

	if stats.FailedFetches != 1 {
		t.Errorf("Expected 1 failed fetch, got %d", stats.FailedFetches)
	}

	if stats.TotalActivities != 1 {
		t.Errorf("Expected 1 total activity, got %d", stats.TotalActivities)
	}

	if !stats.HasErrors() {
		t.Error("Expected stats to indicate errors")
	}

	// Check timing information
	if len(stats.ConnectorTimings) != 2 {
		t.Errorf("Expected timing info for 2 connectors, got %d", len(stats.ConnectorTimings))
	}

	if len(stats.ConnectorErrors) != 1 {
		t.Errorf("Expected 1 connector error, got %d", len(stats.ConnectorErrors))
	}
}

func TestParallelExecutor_WithTimeout(t *testing.T) {
	// Create executor with short timeout
	executor := NewParallelExecutorWithConfig(10, 50*time.Millisecond)
	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create connector that takes longer than timeout
	slowConnector := NewMockSlowConnector("slow", 100*time.Millisecond)
	slowConnector.SetActivities([]timeline.Activity{
		{ID: "1", Title: "Slow Activity", Timestamp: date},
	})

	connectorMap := map[string]Connector{
		"slow": slowConnector,
	}

	results := executor.FetchActivitiesParallel(ctx, connectorMap, date)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Error == nil {
		t.Error("Expected timeout error")
	}

	if result.Error != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", result.Error)
	}
}

func TestExecutionStats_Methods(t *testing.T) {
	stats := ExecutionStats{
		TotalConnectors:   3,
		SuccessfulFetches: 2,
		FailedFetches:     1,
		TotalActivities:   5,
		TotalDuration:     123 * time.Millisecond,
		ConnectorTimings: map[string]time.Duration{
			"fast":   10 * time.Millisecond,
			"medium": 50 * time.Millisecond,
			"slow":   100 * time.Millisecond,
		},
		ConnectorErrors: map[string]error{
			"slow": errors.New("timeout error"),
		},
	}

	// Test String method
	str := stats.String()
	if str == "" {
		t.Error("String method returned empty string")
	}

	// Test HasErrors
	if !stats.HasErrors() {
		t.Error("HasErrors should return true when there are failed fetches")
	}

	// Test GetSlowestConnector
	slowestName, slowestDuration := stats.GetSlowestConnector()
	if slowestName != "slow" || slowestDuration != 100*time.Millisecond {
		t.Errorf("Expected slowest connector 'slow' with 100ms, got '%s' with %v", slowestName, slowestDuration)
	}

	// Test GetFastestConnector
	fastestName, fastestDuration := stats.GetFastestConnector()
	if fastestName != "fast" || fastestDuration != 10*time.Millisecond {
		t.Errorf("Expected fastest connector 'fast' with 10ms, got '%s' with %v", fastestName, fastestDuration)
	}
}

func TestDefaultParallelExecutor(t *testing.T) {
	ctx := context.Background()
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	connector := NewMockSlowConnector("test", 10*time.Millisecond)
	connector.SetActivities([]timeline.Activity{
		{ID: "1", Title: "Test Activity", Timestamp: date},
	})

	connectorMap := map[string]Connector{
		"test": connector,
	}

	// Test convenience function
	activities := FetchActivitiesParallel(ctx, connectorMap, date, false)
	if len(activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(activities))
	}

	// Test with stats
	activities2, stats := FetchActivitiesWithStats(ctx, connectorMap, date)
	if len(activities2) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(activities2))
	}

	if stats.TotalConnectors != 1 {
		t.Errorf("Expected 1 total connector, got %d", stats.TotalConnectors)
	}
}

func BenchmarkParallelExecutor_vs_Sequential(b *testing.B) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create connectors with realistic delays
	connectorMap := make(map[string]Connector)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("connector%d", i)
		connector := NewMockSlowConnector(name, 10*time.Millisecond)
		connector.SetActivities([]timeline.Activity{
			{ID: fmt.Sprintf("%d", i), Title: fmt.Sprintf("Activity %d", i), Timestamp: date},
		})
		connectorMap[name] = connector
	}

	b.Run("Parallel", func(b *testing.B) {
		executor := NewParallelExecutor()
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = executor.FetchAndCombineActivities(ctx, connectorMap, date, false)
		}
	})

	b.Run("Sequential", func(b *testing.B) {
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var allActivities []timeline.Activity
			for _, connector := range connectorMap {
				activities, err := connector.GetActivities(ctx, date)
				if err == nil {
					allActivities = append(allActivities, activities...)
				}
			}
		}
	})
}
