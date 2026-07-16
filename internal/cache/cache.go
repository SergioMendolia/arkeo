package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"

	_ "modernc.org/sqlite"
)

// Cache stores fetched activities in a SQLite database so that past days
// don't need to be re-fetched from connectors on every run.
type Cache struct {
	db   *sql.DB
	path string
	mu   sync.Mutex
}

// New opens (or creates) the cache database at the given path.
func New(dbPath string) (*Cache, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init cache schema: %w", err)
	}

	return &Cache{db: db, path: dbPath}, nil
}

// initSchema creates the cache table if it doesn't exist.
func initSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS activity_cache (
		date        TEXT    NOT NULL,
		connector   TEXT    NOT NULL,
		activities  TEXT    NOT NULL,
		cached_at   INTEGER NOT NULL,
		PRIMARY KEY (date, connector)
	)`)
	return err
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

// HasDay returns true if the cache has entries for all the given connectors
// for the specified date. If connectors is empty, returns true if any entry
// exists for that date.
func (c *Cache) HasDay(date time.Time, connectors []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	dateStr := date.Format("2006-01-02")

	if len(connectors) == 0 {
		var count int
		err := c.db.QueryRow(
			"SELECT COUNT(*) FROM activity_cache WHERE date = ?", dateStr,
		).Scan(&count)
		return err == nil && count > 0
	}

	for _, conn := range connectors {
		var count int
		err := c.db.QueryRow(
			"SELECT COUNT(*) FROM activity_cache WHERE date = ? AND connector = ?",
			dateStr, conn,
		).Scan(&count)
		if err != nil || count == 0 {
			return false
		}
	}
	return true
}

// LoadDay retrieves all cached activities for the specified date.
func (c *Cache) LoadDay(date time.Time) ([]timeline.Activity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	dateStr := date.Format("2006-01-02")

	rows, err := c.db.Query(
		"SELECT activities FROM activity_cache WHERE date = ?",
		dateStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query cache: %w", err)
	}
	defer rows.Close()

	var allActivities []timeline.Activity
	for rows.Next() {
		var activitiesJSON string
		if err := rows.Scan(&activitiesJSON); err != nil {
			continue
		}

		var activities []timeline.Activity
		if err := json.Unmarshal([]byte(activitiesJSON), &activities); err != nil {
			continue
		}
		allActivities = append(allActivities, activities...)
	}

	return allActivities, rows.Err()
}

// StoreDay stores activities for a specific date and connector, replacing
// any existing entry for that (date, connector) pair.
func (c *Cache) StoreDay(date time.Time, connector string, activities []timeline.Activity) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dateStr := date.Format("2006-01-02")

	activitiesJSON, err := json.Marshal(activities)
	if err != nil {
		return fmt.Errorf("failed to marshal activities: %w", err)
	}

	_, err = c.db.Exec(
		`INSERT INTO activity_cache (date, connector, activities, cached_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(date, connector) DO UPDATE SET
		   activities = excluded.activities,
		   cached_at   = excluded.cached_at`,
		dateStr, connector, string(activitiesJSON), time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to store cache entry: %w", err)
	}

	return nil
}

// ResetDay removes all cache entries for the specified date.
func (c *Cache) ResetDay(date time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dateStr := date.Format("2006-01-02")

	_, err := c.db.Exec("DELETE FROM activity_cache WHERE date = ?", dateStr)
	if err != nil {
		return fmt.Errorf("failed to reset cache for %s: %w", dateStr, err)
	}
	return nil
}

// ResetAll removes all cache entries.
func (c *Cache) ResetAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec("DELETE FROM activity_cache")
	return err
}

// ResetRange removes cache entries for a range of dates (inclusive).
func (c *Cache) ResetRange(start, end time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	_, err := c.db.Exec(
		"DELETE FROM activity_cache WHERE date >= ? AND date <= ?",
		startStr, endStr,
	)
	return err
}

// Stats returns basic statistics about the cache.
func (c *Cache) Stats() (CacheStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var stats CacheStats

	err := c.db.QueryRow(
		"SELECT COUNT(*), COUNT(DISTINCT date), MIN(cached_at), MAX(cached_at) FROM activity_cache",
	).Scan(&stats.TotalEntries, &stats.UniqueDates, &stats.OldestCachedAt, &stats.NewestCachedAt)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// CacheStats contains statistics about the cache.
type CacheStats struct {
	TotalEntries    int   // total (date, connector) pairs
	UniqueDates     int   // number of distinct dates cached
	OldestCachedAt  int64 // unix timestamp of oldest cache entry
	NewestCachedAt  int64 // unix timestamp of newest cache entry
}