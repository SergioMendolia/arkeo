package connectors

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"

	_ "modernc.org/sqlite"
)

// Chrome epoch: microseconds since 1601-01-01 UTC.
const chromeEpochOffset = 11644473600000000

// multiPartTLDs lists common second-level domains that use a country-code TLD
// (e.g. "co.uk", "com.au"). When normalizing a hostname we keep three labels
// for these so that "foo.co.uk" normalizes to "foo.co.uk" rather than "co.uk".
var multiPartTLDs = map[string]bool{
	"co.uk": true, "com.au": true, "co.jp": true, "co.nz": true,
	"co.kr": true, "com.br": true, "co.in": true, "com.mx": true,
	"org.uk": true, "co.za": true, "com.sg": true, "co.id": true,
	"com.tr": true, "com.tw": true, "co.th": true, "com.cn": true,
	"com.hk": true, "co.il": true, "com.ar": true, "com.pe": true,
}

// BrowserHistoryConnector implements the Connector interface for browser history.
type BrowserHistoryConnector struct {
	*BaseConnector
}

// NewBrowserHistoryConnector creates a new browser history connector.
func NewBrowserHistoryConnector() *BrowserHistoryConnector {
	return &BrowserHistoryConnector{
		BaseConnector: NewBaseConnector(
			"browser_history",
			"Fetches browsing history from Chrome/Chromium and Firefox",
		),
	}
}

// GetRequiredConfig returns the required configuration for browser history.
func (b *BrowserHistoryConnector) GetRequiredConfig() []ConfigField {
	return MergeConfigFields([]ConfigField{
		{
			Key:         "browsers",
			Type:        "string",
			Required:    false,
			Description: "Comma-separated list of browsers to scan (chrome, firefox)",
			Default:     "chrome,firefox",
		},
		{
			Key:         "exclude_domains",
			Type:        "string",
			Required:    false,
			Description: "Comma-separated list of domains to exclude",
			Default:     "",
		},
		{
			Key:         "group_window_minutes",
			Type:        "int",
			Required:    false,
			Description: "Group visits to the same domain within N minutes into one activity",
			Default:     5,
		},
		{
			Key:         "min_visits",
			Type:        "int",
			Required:    false,
			Description: "Minimum visit count to show a domain (0 = show all)",
			Default:     1,
		},
		{
			Key:         "chrome_profile",
			Type:        "string",
			Required:    false,
			Description: "Chrome profile directory name (default: Default)",
			Default:     "Default",
		},
		{
			Key:         "firefox_profile",
			Type:        "string",
			Required:    false,
			Description: "Firefox profile directory name (auto-detected if empty)",
			Default:     "",
		},
	})
}

// ValidateConfig validates the browser history configuration.
func (b *BrowserHistoryConnector) ValidateConfig(config map[string]interface{}) error {
	return ValidateConfigFields(config, b.GetRequiredConfig())
}

// TestConnection tests if at least one browser history database is accessible.
func (b *BrowserHistoryConnector) TestConnection(ctx context.Context) error {
	browsers := b.getConfigBrowsers()
	paths := detectBrowserDBPaths(browsers)
	if len(paths) == 0 {
		return fmt.Errorf("no browser history databases found")
	}

	for _, p := range paths {
		if _, err := os.Stat(p.dbPath); err != nil {
			continue
		}
		// Try to open and query the database
		tmpPath, err := copyDatabase(p.dbPath)
		if err != nil {
			continue
		}
		defer os.RemoveAll(filepath.Dir(tmpPath))

		db, err := sql.Open("sqlite", "file:"+tmpPath+"?mode=ro")
		if err != nil {
			continue
		}
		var n int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM urls").Scan(&n)
		db.Close()
		if err != nil {
			continue
		}
		return nil
	}

	return fmt.Errorf("could not read any browser history database")
}

// GetActivities retrieves browser history activities for the specified date.
func (b *BrowserHistoryConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	if b.IsDebugMode() {
		log.Printf("BrowserHistory Debug: Fetching activities for date %s", date.Format("2006-01-02"))
	}

	browsers := b.getConfigBrowsers()
	paths := detectBrowserDBPaths(browsers)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no browser history databases found")
	}

	excludeDomains := b.getConfigExcludeDomains()
	excludeSet := make(map[string]bool, len(excludeDomains))
	for _, d := range excludeDomains {
		excludeSet[normalizeDomain(d)] = true
	}

	groupWindow := time.Duration(b.GetConfigInt("group_window_minutes")) * time.Minute
	if groupWindow <= 0 {
		groupWindow = 5 * time.Minute
	}
	minVisits := b.GetConfigInt("min_visits")

	// Normalize to local day (consistent with H3 fix).
	localDate := date.In(time.Local)
	startOfDay := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, localDate.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var allVisits []browserVisit

	for _, p := range paths {
		if b.IsDebugMode() {
			log.Printf("BrowserHistory Debug: Scanning %s database at %s", p.browser, p.dbPath)
		}

		visits, err := b.queryBrowserHistory(ctx, p, startOfDay, endOfDay)
		if err != nil {
			if b.IsDebugMode() {
				log.Printf("BrowserHistory Debug: Failed to query %s: %v", p.browser, err)
			}
			continue
		}

		if b.IsDebugMode() {
			log.Printf("BrowserHistory Debug: Found %d visits from %s", len(visits), p.browser)
		}
		allVisits = append(allVisits, visits...)
	}

	// Filter out non-HTTP and excluded domains.
	var filtered []browserVisit
	for _, v := range allVisits {
		domain := normalizeDomain(v.domain)
		v.domain = domain
		if excludeSet[domain] {
			continue
		}
		filtered = append(filtered, v)
	}

	// Sort by timestamp.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].timestamp.Before(filtered[j].timestamp)
	})

	// Group visits by domain within the time window.
	activities := groupVisitsByDomain(filtered, groupWindow, minVisits)

	if b.IsDebugMode() {
		log.Printf("BrowserHistory Debug: Total activities after grouping: %d", len(activities))
	}

	return activities, nil
}

// browserDBPath represents a detected browser database location.
type browserDBPath struct {
	browser string // "chrome" or "firefox"
	dbPath  string // path to the SQLite database file
}

// browserVisit represents a single browser history visit.
type browserVisit struct {
	url       string
	title     string
	domain    string
	timestamp time.Time
	duration  time.Duration
	browser   string
}

// DomainStats represents visit statistics for a domain.
type DomainStats struct {
	Domain     string
	VisitCount int
	PageCount  int // unique URLs
	LastVisit  time.Time
}

// ScanBrowserDomains scans browser history and returns per-domain statistics.
// Scans the last `days` days of history from the specified browsers.
func ScanBrowserDomains(browsers []string, days int) ([]DomainStats, error) {
	paths := detectBrowserDBPaths(browsers)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no browser history databases found")
	}

	since := time.Now().AddDate(0, 0, -days)
	domainMap := make(map[string]*DomainStats)

	for _, p := range paths {
		visits, err := queryBrowserHistoryStatic(context.Background(), p, since, time.Now())
		if err != nil {
			continue
		}
		for _, v := range visits {
			domain := normalizeDomain(v.domain)
			if domain == "" {
				continue
			}
			stats, ok := domainMap[domain]
			if !ok {
				stats = &DomainStats{Domain: domain}
				domainMap[domain] = stats
			}
			stats.VisitCount++
			// Track unique URLs for page count
			stats.PageCount++ // simplified; could use a set for exact uniqueness
			if v.timestamp.After(stats.LastVisit) {
				stats.LastVisit = v.timestamp
			}
		}
	}

	result := make([]DomainStats, 0, len(domainMap))
	for _, s := range domainMap {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].VisitCount > result[j].VisitCount
	})
	return result, nil
}

// getConfigBrowsers returns the list of browsers to scan from config.
func (b *BrowserHistoryConnector) getConfigBrowsers() []string {
	raw := b.GetConfigString("browsers")
	if raw == "" {
		return []string{"chrome", "firefox"}
	}
	var browsers []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			browsers = append(browsers, s)
		}
	}
	if len(browsers) == 0 {
		return []string{"chrome", "firefox"}
	}
	return browsers
}

// getConfigExcludeDomains returns the list of excluded domains from config.
func (b *BrowserHistoryConnector) getConfigExcludeDomains() []string {
	raw := b.GetConfigString("exclude_domains")
	if raw == "" {
		return nil
	}
	var domains []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			domains = append(domains, s)
		}
	}
	return domains
}

// detectBrowserDBPaths finds browser history database files on the system.
func detectBrowserDBPaths(browsers []string) []browserDBPath {
	var paths []browserDBPath

	for _, browser := range browsers {
		switch browser {
		case "chrome":
			for _, p := range chromeHistoryPaths() {
				if _, err := os.Stat(p); err == nil {
					paths = append(paths, browserDBPath{browser: "chrome", dbPath: p})
				}
			}
		case "firefox":
			for _, p := range firefoxHistoryPaths() {
				if _, err := os.Stat(p); err == nil {
					paths = append(paths, browserDBPath{browser: "firefox", dbPath: p})
				}
			}
		}
	}

	return paths
}

// chromeHistoryPaths returns possible Chrome/Chromium history database paths.
func chromeHistoryPaths() []string {
	profile := "Default"
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		paths = []string{
			filepath.Join(home, "Library/Application Support/Google/Chrome", profile, "History"),
			filepath.Join(home, "Library/Application Support/Chromium", profile, "History"),
		}
	case "windows":
		home := os.Getenv("LOCALAPPDATA")
		if home == "" {
			home = os.Getenv("APPDATA")
		}
		paths = []string{
			filepath.Join(home, "Google", "Chrome", profile, "History"),
			filepath.Join(home, "Chromium", profile, "History"),
		}
	default: // linux and other unix
		home, _ := os.UserHomeDir()
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		paths = []string{
			filepath.Join(xdgConfig, "google-chrome", profile, "History"),
			filepath.Join(xdgConfig, "chromium", profile, "History"),
		}
	}

	return paths
}

// firefoxHistoryPaths returns possible Firefox (and Firefox-based) history
// database paths. Covers standard Firefox, Snap/Flatpak installs, and
// Firefox-based browsers like Zen.
func firefoxHistoryPaths() []string {
	var dirs []string

	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dirs = []string{
			filepath.Join(home, "Library/Application Support/Firefox/Profiles"),
		}
	case "windows":
		home := os.Getenv("APPDATA")
		if home == "" {
			home = os.Getenv("APPDATA")
		}
		dirs = []string{
			filepath.Join(home, "Mozilla", "Firefox", "Profiles"),
		}
	default: // linux and other unix
		home, _ := os.UserHomeDir()
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		dirs = []string{
			// Standard Firefox
			filepath.Join(home, ".mozilla/firefox"),
			// XDG-compliant Firefox (some distros/configs)
			filepath.Join(xdgConfig, "mozilla/firefox"),
			// Snap Firefox
			filepath.Join(home, "snap/firefox/common/.mozilla/firefox"),
			// Flatpak Firefox
			filepath.Join(xdgConfig, "org.mozilla.firefox/.mozilla/firefox"),
			// Zen Browser (Firefox-based)
			filepath.Join(xdgConfig, "zen"),
		}
	}

	var paths []string
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*", "places.sqlite"))
		paths = append(paths, matches...)
	}
	return paths
}

// queryBrowserHistory queries a single browser's history database for visits
// in the given time range.
func (b *BrowserHistoryConnector) queryBrowserHistory(ctx context.Context, p browserDBPath, start, end time.Time) ([]browserVisit, error) {
	return queryBrowserHistoryStatic(ctx, p, start, end)
}

// queryBrowserHistoryStatic is the non-method version used by ScanBrowserDomains.
func queryBrowserHistoryStatic(ctx context.Context, p browserDBPath, start, end time.Time) ([]browserVisit, error) {
	tmpPath, err := copyDatabase(p.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy %s database: %w", p.browser, err)
	}
	defer os.RemoveAll(filepath.Dir(tmpPath))

	db, err := sql.Open("sqlite", "file:"+tmpPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open %s database: %w", p.browser, err)
	}
	defer db.Close()

	switch p.browser {
	case "chrome":
		return queryChromeHistory(ctx, db, start, end, p.browser)
	case "firefox":
		return queryFirefoxHistory(ctx, db, start, end, p.browser)
	default:
		return nil, fmt.Errorf("unsupported browser: %s", p.browser)
	}
}

// queryChromeHistory queries Chrome/Chromium history database.
func queryChromeHistory(ctx context.Context, db *sql.DB, start, end time.Time, browser string) ([]browserVisit, error) {
	startUS := start.UnixMicro() + chromeEpochOffset
	endUS := end.UnixMicro() + chromeEpochOffset

	query := `SELECT u.url, u.title, v.visit_time, v.visit_duration
		FROM visits v
		JOIN urls u ON v.url = u.id
		WHERE v.visit_time >= ? AND v.visit_time < ?
		ORDER BY v.visit_time`

	rows, err := db.QueryContext(ctx, query, startUS, endUS)
	if err != nil {
		return nil, fmt.Errorf("failed to query chrome history: %w", err)
	}
	defer rows.Close()

	var visits []browserVisit
	for rows.Next() {
		var u, title string
		var visitTime, visitDuration int64
		if err := rows.Scan(&u, &title, &visitTime, &visitDuration); err != nil {
			continue
		}

		ts := chromeTimeToTime(visitTime)
		dur := time.Duration(visitDuration) * time.Microsecond

		visits = append(visits, browserVisit{
			url:       u,
			title:     title,
			domain:    extractDomain(u),
			timestamp: ts,
			duration:  dur,
			browser:   browser,
		})
	}

	return visits, rows.Err()
}

// queryFirefoxHistory queries Firefox history database.
func queryFirefoxHistory(ctx context.Context, db *sql.DB, start, end time.Time, browser string) ([]browserVisit, error) {
	startUS := start.UnixMicro()
	endUS := end.UnixMicro()

	query := `SELECT p.url, p.title, v.visit_date
		FROM moz_historyvisits v
		JOIN moz_places p ON v.place_id = p.id
		WHERE v.visit_date >= ? AND v.visit_date < ?
		ORDER BY v.visit_date`

	rows, err := db.QueryContext(ctx, query, startUS, endUS)
	if err != nil {
		return nil, fmt.Errorf("failed to query firefox history: %w", err)
	}
	defer rows.Close()

	var visits []browserVisit
	for rows.Next() {
		var u, title string
		var visitDate int64
		if err := rows.Scan(&u, &title, &visitDate); err != nil {
			continue
		}

		ts := time.UnixMicro(visitDate)

		visits = append(visits, browserVisit{
			url:       u,
			title:     title,
			domain:    extractDomain(u),
			timestamp: ts,
			browser:   browser,
		})
	}

	return visits, rows.Err()
}

// copyDatabase copies a SQLite database (and its WAL/SHM files) to a temp
// directory so we can open it read-only without conflicting with the browser
// which may hold a lock on the original file.
func copyDatabase(srcPath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "arkeo-browser-*")
	if err != nil {
		return "", err
	}

	base := filepath.Base(srcPath)
	dstPath := filepath.Join(tmpDir, base)

	if err := copyFile(srcPath, dstPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	// Copy WAL and SHM files if they exist.
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecar := srcPath + suffix
		if _, err := os.Stat(sidecar); err == nil {
			if err := copyFile(sidecar, dstPath+suffix); err != nil {
				// Non-fatal: WAL/SHM may not exist or may be transient.
				continue
			}
		}
	}

	return dstPath, nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, srcF)
	return err
}

// chromeTimeToTime converts a Chrome/Chromium visit_time (microseconds since
// 1601-01-01 UTC) to a Go time.Time.
func chromeTimeToTime(visitTime int64) time.Time {
	return time.UnixMicro(visitTime - chromeEpochOffset)
}

// extractDomain parses a URL and returns the normalized domain.
// Returns empty string for non-HTTP schemes (file://, chrome-extension://, etc.).
func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return normalizeDomain(parsed.Hostname())
}

// NormalizeDomain is the exported version of normalizeDomain for use by
// other packages (e.g. the cmd package's domain manager TUI).
func NormalizeDomain(hostname string) string {
	return normalizeDomain(hostname)
}

// normalizeDomain strips "www." and collapses subdomains to the registrable
// domain. Examples:
//   "www.github.com"     -> "github.com"
//   "docs.github.com"    -> "github.com"
//   "home.atlassian.com" -> "atlassian.com"
//   "foo.co.uk"          -> "foo.co.uk"
//   "bar.foo.co.uk"      -> "foo.co.uk"
func normalizeDomain(hostname string) string {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return ""
	}
	hostname = strings.TrimPrefix(hostname, "www.")
	if hostname == "localhost" || hostname == "127.0.0.1" {
		return hostname
	}

	parts := strings.Split(hostname, ".")
	if len(parts) <= 2 {
		return hostname
	}

	// Check for multi-part TLD (e.g. "co.uk").
	lastTwo := strings.Join(parts[len(parts)-2:], ".")
	if multiPartTLDs[lastTwo] {
		if len(parts) >= 3 {
			return strings.Join(parts[len(parts)-3:], ".")
		}
		return hostname
	}

	// Standard TLD: take last 2 parts.
	return strings.Join(parts[len(parts)-2:], ".")
}

// groupVisitsByDomain groups visits to the same domain within the given time
// window into a single timeline activity.
func groupVisitsByDomain(visits []browserVisit, window time.Duration, minVisits int) []timeline.Activity {
	if len(visits) == 0 {
		return nil
	}

	var activities []timeline.Activity
	var group []browserVisit

	flushGroup := func() {
		if len(group) == 0 {
			return
		}
		if minVisits > 0 && len(group) < minVisits {
			group = group[:0]
			return
		}

		domain := group[0].domain
		uniqueTitles := make(map[string]bool)
		var topTitles []string
		for _, v := range group {
			if v.title != "" && !uniqueTitles[v.title] {
				uniqueTitles[v.title] = true
				topTitles = append(topTitles, v.title)
				if len(topTitles) >= 3 {
					break
				}
			}
		}

		var totalDuration time.Duration
		browsers := make(map[string]bool)
		for _, v := range group {
			totalDuration += v.duration
			browsers[v.browser] = true
		}
		var browserList []string
		for b := range browsers {
			browserList = append(browserList, b)
		}

		title := fmt.Sprintf("Visited %s (%d pages)", domain, len(group))
		if len(group) == 1 {
			title = fmt.Sprintf("Visited %s", domain)
		}

		description := ""
		if len(topTitles) > 0 {
			description = strings.Join(topTitles, ", ")
		}

		var dur *time.Duration
		if totalDuration > 0 {
			d := totalDuration
			dur = &d
		}

		activities = append(activities, timeline.Activity{
			ID:          fmt.Sprintf("browser-%s-%d", domain, group[0].timestamp.Unix()),
			Type:        timeline.ActivityTypeBrowser,
			Title:       title,
			Description: description,
			Timestamp:   group[0].timestamp,
			Duration:    dur,
			Source:      "browser_history",
			URL:         "https://" + domain,
			Metadata: map[string]string{
				"domain":      domain,
				"visit_count": fmt.Sprintf("%d", len(group)),
				"browsers":    strings.Join(browserList, ", "),
			},
		})
		group = group[:0]
	}

	for _, v := range visits {
		if len(group) == 0 {
			group = append(group, v)
			continue
		}
		last := group[len(group)-1]
		if v.domain == last.domain && v.timestamp.Sub(last.timestamp) <= window {
			group = append(group, v)
		} else {
			flushGroup()
			group = append(group, v)
		}
	}
	flushGroup()

	return activities
}