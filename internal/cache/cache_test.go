package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	c, err := New(filepath.Join(dir, "test_cache.db"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCache_StoreAndLoad(t *testing.T) {
	c := newTestCache(t)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	activities := []timeline.Activity{
		{ID: "1", Title: "Test Activity 1", Source: "github", Timestamp: date.Add(2 * time.Hour)},
		{ID: "2", Title: "Test Activity 2", Source: "calendar", Timestamp: date.Add(5 * time.Hour)},
	}

	if err := c.StoreDay(date, "github", activities); err != nil {
		t.Fatalf("StoreDay failed: %v", err)
	}

	loaded, err := c.LoadDay(date)
	if err != nil {
		t.Fatalf("LoadDay failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Expected 2 activities, got %d", len(loaded))
	}

	if loaded[0].Title != "Test Activity 1" {
		t.Errorf("Expected title 'Test Activity 1', got '%s'", loaded[0].Title)
	}
}

func TestCache_HasDay(t *testing.T) {
	c := newTestCache(t)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	if c.HasDay(date, nil) {
		t.Error("Expected HasDay=false before storing")
	}

	c.StoreDay(date, "github", []timeline.Activity{
		{ID: "1", Title: "Test", Source: "github", Timestamp: date},
	})

	if !c.HasDay(date, nil) {
		t.Error("Expected HasDay=true after storing")
	}

	if !c.HasDay(date, []string{"github"}) {
		t.Error("Expected HasDay=true for github connector")
	}

	if c.HasDay(date, []string{"github", "calendar"}) {
		t.Error("Expected HasDay=false when calendar is not cached")
	}
}

func TestCache_ResetDay(t *testing.T) {
	c := newTestCache(t)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	c.StoreDay(date, "github", []timeline.Activity{
		{ID: "1", Title: "Test", Source: "github", Timestamp: date},
	})

	if !c.HasDay(date, nil) {
		t.Error("Expected HasDay=true before reset")
	}

	if err := c.ResetDay(date); err != nil {
		t.Fatalf("ResetDay failed: %v", err)
	}

	if c.HasDay(date, nil) {
		t.Error("Expected HasDay=false after reset")
	}
}

func TestCache_ResetAll(t *testing.T) {
	c := newTestCache(t)
	date1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)

	c.StoreDay(date1, "github", []timeline.Activity{{ID: "1", Title: "A", Source: "github", Timestamp: date1}})
	c.StoreDay(date2, "calendar", []timeline.Activity{{ID: "2", Title: "B", Source: "calendar", Timestamp: date2}})

	if err := c.ResetAll(); err != nil {
		t.Fatalf("ResetAll failed: %v", err)
	}

	if c.HasDay(date1, nil) || c.HasDay(date2, nil) {
		t.Error("Expected all days cleared after ResetAll")
	}
}

func TestCache_ResetRange(t *testing.T) {
	c := newTestCache(t)
	date1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	date3 := time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC)

	c.StoreDay(date1, "github", []timeline.Activity{{ID: "1", Title: "A", Source: "github", Timestamp: date1}})
	c.StoreDay(date2, "github", []timeline.Activity{{ID: "2", Title: "B", Source: "github", Timestamp: date2}})
	c.StoreDay(date3, "github", []timeline.Activity{{ID: "3", Title: "C", Source: "github", Timestamp: date3}})

	// Reset Jan 15-16
	if err := c.ResetRange(date1, date2); err != nil {
		t.Fatalf("ResetRange failed: %v", err)
	}

	if c.HasDay(date1, nil) {
		t.Error("Expected date1 cleared")
	}
	if c.HasDay(date2, nil) {
		t.Error("Expected date2 cleared")
	}
	if !c.HasDay(date3, nil) {
		t.Error("Expected date3 to still be cached")
	}
}

func TestCache_OverwriteExisting(t *testing.T) {
	c := newTestCache(t)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	c.StoreDay(date, "github", []timeline.Activity{
		{ID: "1", Title: "Old", Source: "github", Timestamp: date},
	})

	c.StoreDay(date, "github", []timeline.Activity{
		{ID: "2", Title: "New", Source: "github", Timestamp: date},
	})

	loaded, _ := c.LoadDay(date)
	if len(loaded) != 1 {
		t.Fatalf("Expected 1 activity after overwrite, got %d", len(loaded))
	}
	if loaded[0].Title != "New" {
		t.Errorf("Expected title 'New', got '%s'", loaded[0].Title)
	}
}

func TestCache_MultipleConnectorsSameDay(t *testing.T) {
	c := newTestCache(t)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	c.StoreDay(date, "github", []timeline.Activity{
		{ID: "1", Title: "GH Activity", Source: "github", Timestamp: date},
	})
	c.StoreDay(date, "calendar", []timeline.Activity{
		{ID: "2", Title: "Cal Activity", Source: "calendar", Timestamp: date.Add(2 * time.Hour)},
	})

	loaded, _ := c.LoadDay(date)
	if len(loaded) != 2 {
		t.Fatalf("Expected 2 activities from 2 connectors, got %d", len(loaded))
	}
}

func TestCache_Stats(t *testing.T) {
	c := newTestCache(t)
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	c.StoreDay(date, "github", []timeline.Activity{{ID: "1", Title: "A", Source: "github", Timestamp: date}})
	c.StoreDay(date, "calendar", []timeline.Activity{{ID: "2", Title: "B", Source: "calendar", Timestamp: date}})

	stats, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalEntries != 2 {
		t.Errorf("Expected 2 total entries, got %d", stats.TotalEntries)
	}
	if stats.UniqueDates != 1 {
		t.Errorf("Expected 1 unique date, got %d", stats.UniqueDates)
	}
}