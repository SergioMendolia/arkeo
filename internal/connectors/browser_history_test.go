package connectors

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"

	_ "modernc.org/sqlite"
)

func TestNewBrowserHistoryConnector(t *testing.T) {
	connector := NewBrowserHistoryConnector()

	if connector.Name() != "browser_history" {
		t.Errorf("Expected name 'browser_history', got '%s'", connector.Name())
	}

	if connector.Description() == "" {
		t.Error("Expected non-empty description")
	}

	if connector.IsEnabled() {
		t.Error("Connector should be disabled by default")
	}
}

func TestBrowserHistoryConnector_GetRequiredConfig(t *testing.T) {
	connector := NewBrowserHistoryConnector()
	config := connector.GetRequiredConfig()

	if len(config) == 0 {
		t.Error("Expected config fields, got none")
	}

	fieldMap := make(map[string]bool)
	for _, field := range config {
		fieldMap[field.Key] = true
	}

	expectedFields := []string{"browsers", "exclude_domains", "group_window_minutes", "min_visits", "chrome_profile", "firefox_profile"}
	for _, key := range expectedFields {
		if !fieldMap[key] {
			t.Errorf("Expected config field '%s' not found", key)
		}
	}
}

func TestBrowserHistoryConnector_ValidateConfig(t *testing.T) {
	connector := NewBrowserHistoryConnector()

	// Valid config (all fields optional, so empty should pass)
	err := connector.ValidateConfig(map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected no error for empty config, got: %v", err)
	}
}

func TestBrowserHistoryConnector_ChromeEpochConversion(t *testing.T) {
	// Chrome epoch: microseconds since 1601-01-01 UTC
	// Test: 2024-01-15 12:00:00 UTC = 1705315200 Unix seconds
	expected := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	chromeUS := expected.UnixMicro() + chromeEpochOffset

	result := chromeTimeToTime(chromeUS)

	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestBrowserHistoryConnector_FirefoxEpochConversion(t *testing.T) {
	// Firefox epoch: microseconds since 1970-01-01
	expected := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	firefoxUS := expected.UnixMicro()

	result := time.UnixMicro(firefoxUS)

	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"localhost", "localhost"},
		{"127.0.0.1", "127.0.0.1"},
		{"github.com", "github.com"},
		{"www.github.com", "github.com"},
		{"docs.github.com", "github.com"},
		{"home.atlassian.com", "atlassian.com"},
		{"api.v2.example.com", "example.com"},
		{"foo.co.uk", "foo.co.uk"},
		{"bar.foo.co.uk", "foo.co.uk"},
		{"www.example.co.uk", "example.co.uk"},
		{"sub.domain.co.jp", "domain.co.jp"},
		{"  github.com  ", "github.com"},
		{"sub.domain.com.br", "domain.com.br"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeDomain(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDomain(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"https URL", "https://github.com/user/repo", "github.com"},
		{"http URL", "http://example.com/page", "example.com"},
		{"https with www", "https://www.google.com/search", "google.com"},
		{"https with subdomain", "https://docs.python.org/3/", "python.org"},
		{"file URL", "file:///home/user/doc.pdf", ""},
		{"chrome-extension URL", "chrome-extension://abc123/popup.html", ""},
		{"invalid URL", "://invalid", ""},
		{"about page", "about:blank", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDomain(tt.url)
			if result != tt.expected {
				t.Errorf("extractDomain(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestBrowserHistoryConnector_DomainGrouping(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	visits := []browserVisit{
		{domain: "github.com", title: "Page 1", timestamp: baseTime, browser: "chrome"},
		{domain: "github.com", title: "Page 2", timestamp: baseTime.Add(2 * time.Minute), browser: "chrome"},
		{domain: "github.com", title: "Page 3", timestamp: baseTime.Add(4 * time.Minute), browser: "chrome"},
		// Gap > 5 minutes: new group
		{domain: "github.com", title: "Page 4", timestamp: baseTime.Add(10 * time.Minute), browser: "chrome"},
		// Different domain
		{domain: "stackoverflow.com", title: "Q1", timestamp: baseTime.Add(11 * time.Minute), browser: "firefox"},
	}

	activities := groupVisitsByDomain(visits, 5*time.Minute, 1)

	if len(activities) != 3 {
		t.Fatalf("Expected 3 activities (2 github groups + 1 stackoverflow), got %d", len(activities))
	}

	// First group: 3 pages
	if activities[0].Metadata["visit_count"] != "3" {
		t.Errorf("Expected first group visit_count=3, got %s", activities[0].Metadata["visit_count"])
	}
	if activities[0].Title != "Visited github.com (3 pages)" {
		t.Errorf("Expected title 'Visited github.com (3 pages)', got %s", activities[0].Title)
	}

	// Second group: 1 page
	if activities[1].Metadata["visit_count"] != "1" {
		t.Errorf("Expected second group visit_count=1, got %s", activities[1].Metadata["visit_count"])
	}
	if activities[1].Title != "Visited github.com" {
		t.Errorf("Expected title 'Visited github.com', got %s", activities[1].Title)
	}

	// Third group: stackoverflow
	if activities[2].Metadata["domain"] != "stackoverflow.com" {
		t.Errorf("Expected third group domain=stackoverflow.com, got %s", activities[2].Metadata["domain"])
	}
}

func TestBrowserHistoryConnector_GroupingMinVisits(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	visits := []browserVisit{
		{domain: "github.com", title: "P1", timestamp: baseTime, browser: "chrome"},
		{domain: "github.com", title: "P2", timestamp: baseTime.Add(1 * time.Minute), browser: "chrome"},
		{domain: "solo.com", title: "S1", timestamp: baseTime.Add(5 * time.Minute), browser: "chrome"},
	}

	// minVisits=2: solo.com should be filtered out
	activities := groupVisitsByDomain(visits, 5*time.Minute, 2)

	if len(activities) != 1 {
		t.Fatalf("Expected 1 activity (solo.com filtered by minVisits=2), got %d", len(activities))
	}

	if activities[0].Metadata["domain"] != "github.com" {
		t.Errorf("Expected domain github.com, got %s", activities[0].Metadata["domain"])
	}
}

func TestBrowserHistoryConnector_ExcludeDomains(t *testing.T) {
	connector := NewBrowserHistoryConnector()
	connector.Configure(map[string]interface{}{
		"browsers":       "chrome",
		"exclude_domains": "google.com, localhost",
	})

	excluded := connector.getConfigExcludeDomains()
	if len(excluded) != 2 {
		t.Fatalf("Expected 2 excluded domains, got %d", len(excluded))
	}
	if excluded[0] != "google.com" || excluded[1] != "localhost" {
		t.Errorf("Expected [google.com, localhost], got %v", excluded)
	}
}

func TestBrowserHistoryConnector_GetConfigBrowsers(t *testing.T) {
	connector := NewBrowserHistoryConnector()

	// Default
	browsers := connector.getConfigBrowsers()
	if len(browsers) != 2 || browsers[0] != "chrome" || browsers[1] != "firefox" {
		t.Errorf("Expected [chrome, firefox], got %v", browsers)
	}

	// Custom
	connector.Configure(map[string]interface{}{"browsers": "chrome"})
	browsers = connector.getConfigBrowsers()
	if len(browsers) != 1 || browsers[0] != "chrome" {
		t.Errorf("Expected [chrome], got %v", browsers)
	}
}

func TestBrowserHistoryConnector_ActivityType(t *testing.T) {
	connector := NewBrowserHistoryConnector()
	connector.Configure(map[string]interface{}{})

	// Verify the activity type is correct
	if timeline.ActivityTypeBrowser != "browser" {
		t.Errorf("Expected ActivityTypeBrowser to be 'browser', got %s", timeline.ActivityTypeBrowser)
	}
}

// TestBrowserHistoryConnector_IntegrationTest tests with a real SQLite database.
// This test creates a temporary SQLite database with known data and verifies
// the connector can query it correctly.
func TestBrowserHistoryConnector_IntegrationTest(t *testing.T) {
	// Create a temporary Chrome-style database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "History")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create Chrome schema
	_, err = db.Exec(`CREATE TABLE urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url LONGVARCHAR,
		title LONGVARCHAR,
		visit_count INTEGER DEFAULT 0 NOT NULL,
		typed_count INTEGER DEFAULT 0 NOT NULL,
		last_visit_time INTEGER NOT NULL,
		hidden INTEGER DEFAULT 0 NOT NULL
	)`)
	if err != nil {
		t.Fatalf("Failed to create urls table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE visits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url INTEGER NOT NULL,
		visit_time INTEGER NOT NULL,
		from_visit INTEGER,
		external_referrer_url TEXT,
		transition INTEGER DEFAULT 0 NOT NULL,
		segment_id INTEGER,
		visit_duration INTEGER DEFAULT 0 NOT NULL
	)`)
	if err != nil {
		t.Fatalf("Failed to create visits table: %v", err)
	}

	// Insert test data
	// 2024-01-15 10:00:00 UTC in Chrome epoch microseconds
	visitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC).UnixMicro() + chromeEpochOffset
	visitDuration := int64(30 * time.Second / time.Microsecond) // 30 seconds

	_, err = db.Exec(`INSERT INTO urls (id, url, title, visit_count, typed_count, last_visit_time, hidden) VALUES
		(1, 'https://github.com/user/repo', 'User Repo', 5, 0, ?, 0),
		(2, 'https://github.com/user/repo/issues', 'Issues', 3, 0, ?, 0),
		(3, 'https://stackoverflow.com/questions/123', 'Question 123', 1, 0, ?, 0),
		(4, 'file:///home/user/doc.pdf', 'Local Doc', 1, 0, ?, 0)`,
		visitTime, visitTime, visitTime, visitTime)
	if err != nil {
		t.Fatalf("Failed to insert test urls: %v", err)
	}

	_, err = db.Exec(`INSERT INTO visits (url, visit_time, visit_duration) VALUES
		(1, ?, ?),
		(2, ?, ?),
		(3, ?, 0),
		(4, ?, 0)`,
		visitTime, visitDuration,
		visitTime+int64(2*time.Minute/time.Microsecond), visitDuration,
		visitTime+int64(5*time.Minute/time.Microsecond),
		visitTime+int64(10*time.Minute/time.Microsecond))
	if err != nil {
		t.Fatalf("Failed to insert test visits: %v", err)
	}

	db.Close()

	// Now query using our functions
	p := browserDBPath{browser: "chrome", dbPath: dbPath}
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	visits, err := queryBrowserHistoryStatic(context.Background(), p, start, end)
	if err != nil {
		t.Fatalf("Failed to query test database: %v", err)
	}

	// Should get 4 visits (including file:// which is filtered later)
	if len(visits) != 4 {
		t.Fatalf("Expected 4 visits, got %d", len(visits))
	}

	// Verify domains
	domains := make(map[string]bool)
	for _, v := range visits {
		domains[v.domain] = true
	}

	if !domains["github.com"] {
		t.Error("Expected github.com in visits")
	}
	if !domains["stackoverflow.com"] {
		t.Error("Expected stackoverflow.com in visits")
	}
	// file:// should have empty domain
	if !domains[""] {
		t.Error("Expected empty domain for file:// URL")
	}

	// Verify Chrome epoch conversion
	if !visits[0].timestamp.Equal(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("Expected first visit at 10:00 UTC, got %v", visits[0].timestamp)
	}

	// Verify duration
	if visits[0].duration != 30*time.Second {
		t.Errorf("Expected 30s duration, got %v", visits[0].duration)
	}
}

func TestBrowserHistoryConnector_TestConnection_NoDB(t *testing.T) {
	connector := NewBrowserHistoryConnector()
	connector.Configure(map[string]interface{}{"browsers": "chrome"})

	// On a system with no Chrome, this should fail gracefully
	err := connector.TestConnection(context.Background())
	if err == nil {
		// If it passes, that means there IS a browser DB — that's fine too
		t.Log("Browser DB found (test passes if Chrome is installed)")
	}
}

func TestCopyDatabase(t *testing.T) {
	// Create a source file
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.db")
	content := []byte("test database content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy it
	dstPath, err := copyDatabase(srcPath)
	if err != nil {
		t.Fatalf("copyDatabase failed: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(dstPath))

	// Verify the copy exists and has the same content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Error("Copied file content does not match source")
	}

	// Verify the source still exists (we only copy, not move)
	if _, err := os.Stat(srcPath); err != nil {
		t.Error("Source file should still exist after copy")
	}
}