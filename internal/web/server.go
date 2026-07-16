package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/cache"
	"github.com/arkeo/arkeo/internal/config"
	"github.com/arkeo/arkeo/internal/connectors"
	"github.com/arkeo/arkeo/internal/timeline"
	"github.com/arkeo/arkeo/internal/utils"
)

//go:embed templates/*.html
var templateFS embed.FS

// Server is the web UI server.
type Server struct {
	configManager *config.Manager
	registry      *connectors.ConnectorRegistry
	cache         *cache.Cache
	templates     map[string]*template.Template
	httpServer    *http.Server
}

// New creates a new web server.
func New(configManager *config.Manager, registry *connectors.ConnectorRegistry, activityCache *cache.Cache) *Server {
	// Parse layout + each page template separately so the "content"
	// block doesn't collide across pages.
	layout := "templates/layout.html"
	pages := map[string]string{
		"timeline":   "templates/timeline.html",
		"connectors": "templates/connectors.html",
		"browser":     "templates/browser.html",
	}
	templates := make(map[string]*template.Template, len(pages))
	for name, page := range pages {
		templates[name] = template.Must(template.ParseFS(templateFS, layout, page))
	}

	return &Server{
		configManager: configManager,
		registry:      registry,
		cache:          activityCache,
		templates:     templates,
	}
}

// ListenAndServe starts the web server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleTimeline)
	mux.HandleFunc("/connectors", s.handleConnectors)
	mux.HandleFunc("/browser", s.handleBrowser)
	mux.HandleFunc("/api/timeline", s.handleAPITimeline)
	mux.HandleFunc("/api/cache/reset", s.handleAPICacheReset)
	mux.HandleFunc("/api/connectors/enable", s.handleAPIConnectorToggle(true))
	mux.HandleFunc("/api/connectors/disable", s.handleAPIConnectorToggle(false))
	mux.HandleFunc("/api/connectors/test", s.handleAPIConnectorTest)
	mux.HandleFunc("/api/connectors/config", s.handleAPIConnectorConfig)
	mux.HandleFunc("/api/browser/domains", s.handleAPIBrowserDomains)
	mux.HandleFunc("/api/browser/exclusions", s.handleAPIBrowserExclusions)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	url := fmt.Sprintf("http://%s", addr)
	log.Printf("Arkeo web UI: %s", url)

	// Try to open the browser automatically
	go openBrowser(url)

	return s.httpServer.ListenAndServe()
}

// openBrowser tries to open the default browser to the given URL.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	if cmd != nil {
		cmd.Start()
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// --- Page Handlers ---

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Read date and format from query params so the page is URL-accessible
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "table"
	}
	data := pageData{ActivePage: "timeline", Date: dateStr, Format: format}
	s.renderPage(w, "timeline", data)
}

func (s *Server) handleConnectors(w http.ResponseWriter, r *http.Request) {
	var connectorList []connectorInfo
	for name, conn := range s.registry.List() {
		connectorList = append(connectorList, connectorInfo{
			Name:        name,
			Description: conn.Description(),
			Enabled:     s.configManager.IsConnectorEnabled(name),
		})
	}
	sort.Slice(connectorList, func(i, j int) bool { return connectorList[i].Name < connectorList[j].Name })

	data := pageData{ActivePage: "connectors", Connectors: connectorList}
	s.renderPage(w, "connectors", data)
}

func (s *Server) handleBrowser(w http.ResponseWriter, r *http.Request) {
	data := pageData{ActivePage: "browser", Days: "90"}
	s.renderPage(w, "browser", data)
}

func (s *Server) renderPage(w http.ResponseWriter, contentTemplate string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, ok := s.templates[contentTemplate]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// --- API Handlers ---

func (s *Server) handleAPITimeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "table"
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeJSONError(w, "Invalid date format")
		return
	}

	day := parsedDate.Truncate(24 * time.Hour)

	enabledConnectors := getEnabledConnectors(s.configManager, s.registry)
	if len(enabledConnectors) == 0 {
		writeJSONError(w, "No connectors enabled")
		return
	}

	utilsConnectors := make(map[string]utils.Connector)
	connectorNames := make([]string, 0, len(enabledConnectors))
	for name, conn := range enabledConnectors {
		utilsConnectors[name] = conn
		connectorNames = append(connectorNames, name)
	}

	ctx := context.Background()
	var dayActivities []timeline.Activity
	isCached := false

	if s.cache != nil && s.cache.HasDay(day, connectorNames) {
		cached, err := s.cache.LoadDay(day)
		if err == nil {
			dayActivities = cached
			isCached = true
		}
	}

	if !isCached {
		executor := utils.NewParallelExecutor()
		results := executor.FetchActivitiesParallel(ctx, utilsConnectors, day)
		for _, result := range results {
			if result.Error != nil {
				continue
			}
			dayActivities = append(dayActivities, result.Activities...)
			if s.cache != nil {
				s.cache.StoreDay(day, result.Name, result.Activities)
			}
		}
	}

	// Sort
	sort.Slice(dayActivities, func(i, j int) bool {
		return dayActivities[i].Timestamp.Before(dayActivities[j].Timestamp)
	})

	// Build activity views
	var activitiesView []activityView
	var prevTime time.Time
	for _, a := range dayActivities {
		av := activityView{
			Time:        a.Timestamp.Format("15:04"),
			SourceLabel: getSourceLabel(a.Source),
			Title:       a.Title,
			Description: a.Description,
		}
		if a.Duration != nil {
			av.Duration = a.FormatDuration()
		}
		if !prevTime.IsZero() {
			gap := a.Timestamp.Sub(prevTime)
			if gap > time.Hour {
				av.Gap = formatDuration(gap)
			}
		}
		activitiesView = append(activitiesView, av)
		prevTime = a.Timestamp
	}

	span := ""
	if len(dayActivities) > 0 {
		span = formatDuration(dayActivities[len(dayActivities)-1].Timestamp.Sub(dayActivities[0].Timestamp))
	}

	dayResult := struct {
		Date        string          `json:"date"`
		DateDisplay string          `json:"date_display"`
		Count       int             `json:"count"`
		Span        string          `json:"span"`
		Cached      bool            `json:"cached"`
		Activities  []activityView  `json:"activities"`
	}{
		Date:        day.Format("2006-01-02"),
		DateDisplay: day.Format("Monday, January 2, 2006"),
		Count:       len(dayActivities),
		Span:        span,
		Cached:      isCached,
		Activities:  activitiesView,
	}

	if format == "json" {
		// Return raw JSON (metadata-free projection)
		type jsonActivity struct {
			ID          string                `json:"id"`
			Type        timeline.ActivityType `json:"type"`
			Title       string                `json:"title"`
			Description string                `json:"description"`
			Timestamp   time.Time             `json:"timestamp"`
			Duration    *time.Duration        `json:"duration,omitempty"`
			Source      string                `json:"source"`
			URL         string                `json:"url,omitempty"`
		}
		var acts []jsonActivity
		for _, a := range dayActivities {
			acts = append(acts, jsonActivity{
				ID: a.ID, Type: a.Type, Title: a.Title, Description: a.Description,
				Timestamp: a.Timestamp, Duration: a.Duration, Source: a.Source, URL: a.URL,
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"date":       dayResult.Date,
			"date_display": dayResult.DateDisplay,
			"activities": acts,
			"cached":     isCached,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"days":  []interface{}{dayResult},
		"total": len(dayActivities),
		"cached_days": func() int { if isCached { return 1 }; return 0 }(),
	})
}

func (s *Server) handleAPICacheReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cache == nil {
		writeJSONError(w, "Cache not available")
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		writeJSONError(w, "Date required")
		return
	}

	rangeStr := r.URL.Query().Get("range")
	rangeNum := 1
	if rangeStr != "" {
		fmt.Sscanf(rangeStr, "%d", &rangeNum)
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeJSONError(w, "Invalid date")
		return
	}

	if rangeNum > 1 {
		start := parsedDate.AddDate(0, 0, -(rangeNum - 1))
		s.cache.ResetRange(start, parsedDate)
	} else {
		s.cache.ResetDay(parsedDate)
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Cache cleared"})
}

func (s *Server) handleAPIConnectorToggle(enable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		name := r.URL.Query().Get("name")
		if name == "" {
			writeJSONError(w, "Connector name required")
			return
		}
		if _, exists := s.registry.Get(name); !exists {
			writeJSONError(w, "Connector not found")
			return
		}
		if enable {
			s.configManager.EnableConnector(name)
		} else {
			s.configManager.DisableConnector(name)
		}
		if err := s.configManager.Save(); err != nil {
			writeJSONError(w, "Failed to save config: "+err.Error())
			return
		}
		action := "enabled"
		if !enable {
			action = "disabled"
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Connector " + name + " " + action})
	}
}

func (s *Server) handleAPIConnectorTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSONError(w, "Connector name required")
		return
	}
	conn, exists := s.registry.Get(name)
	if !exists {
		writeJSONError(w, "Connector not found")
		return
	}

	// Configure the connector
	connConfig, hasConfig := s.configManager.GetConnectorConfig(name)
	if !hasConfig || !connConfig.Enabled {
		writeJSONError(w, "Connector is not enabled or configured")
		return
	}

	configWithLogLevel := make(map[string]interface{})
	for k, v := range connConfig.Config {
		configWithLogLevel[k] = v
	}
	configWithLogLevel["log_level"] = s.configManager.GetConfig().App.LogLevel

	if err := conn.Configure(configWithLogLevel); err != nil {
		writeJSONError(w, "Config error: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := conn.TestConnection(ctx); err != nil {
		writeJSONError(w, err.Error())
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Connection successful"})
}

func (s *Server) handleAPIConnectorConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSONError(w, "Connector name required")
		return
	}
	conn, exists := s.registry.Get(name)
	if !exists {
		writeJSONError(w, "Connector not found")
		return
	}

	if r.Method == "GET" {
		// Return current config values and the list of required config fields
		connConfig, hasConfig := s.configManager.GetConnectorConfig(name)
		fields := make(map[string]string)
		if hasConfig && connConfig.Config != nil {
			for k, v := range connConfig.Config {
				if s, ok := v.(string); ok {
					fields[k] = s
				} else {
					fields[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		// Determine which fields are secrets
		secretFields := []string{}
		for _, field := range conn.GetRequiredConfig() {
			if field.Type == "secret" {
				secretFields = append(secretFields, field.Key)
			}
			// Ensure all required config fields appear even if empty
			if _, exists := fields[field.Key]; !exists {
				fields[field.Key] = ""
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"fields":        fields,
			"secret_fields": secretFields,
		})
		return
	}

	// POST — save config values
	var body struct {
		Config map[string]string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, "Invalid request body")
		return
	}

	// Merge with existing config (don't overwrite fields not sent)
	connConfig, hasConfig := s.configManager.GetConnectorConfig(name)
	merged := make(map[string]interface{})
	if hasConfig && connConfig.Config != nil {
		for k, v := range connConfig.Config {
			merged[k] = v
		}
	}
	for k, v := range body.Config {
		merged[k] = v
	}

	s.configManager.SetConnectorConfig(name, config.ConnectorConfig{
		Enabled: s.configManager.IsConnectorEnabled(name),
		Config:  merged,
	})

	if err := s.configManager.Save(); err != nil {
		writeJSONError(w, "Failed to save config: "+err.Error())
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Settings saved"})
}

func (s *Server) handleAPIBrowserDomains(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	daysStr := r.URL.Query().Get("days")
	days := 90
	if daysStr != "" {
		fmt.Sscanf(daysStr, "%d", &days)
	}

	browsers := []string{"chrome", "firefox"}
	stats, err := connectors.ScanBrowserDomains(browsers, days)
	if err != nil {
		writeJSONError(w, err.Error())
		return
	}

	// Load current exclusions
	excludeSet := make(map[string]bool)
	connConfig, exists := s.configManager.GetConnectorConfig("browser_history")
	if exists && connConfig.Config != nil {
		if raw, ok := connConfig.Config["exclude_domains"].(string); ok {
			for _, d := range strings.Split(raw, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					excludeSet[connectors.NormalizeDomain(d)] = true
				}
			}
		}
	}

	type domainView struct {
		Domain      string `json:"domain"`
		VisitCount  int    `json:"visit_count"`
		PageCount   int    `json:"page_count"`
		Excluded    bool   `json:"excluded"`
	}

	var domains []domainView
	for _, s := range stats {
		domains = append(domains, domainView{
			Domain:     s.Domain,
			VisitCount: s.VisitCount,
			PageCount:  s.PageCount,
			Excluded:   excludeSet[s.Domain],
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"domains": domains})
}

func (s *Server) handleAPIBrowserExclusions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Domains []string `json:"domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, "Invalid request body")
		return
	}

	sort.Strings(body.Domains)
	excludeStr := strings.Join(body.Domains, ", ")
	s.configManager.SetConnectorConfigValue("browser_history", "exclude_domains", excludeStr)

	if err := s.configManager.Save(); err != nil {
		writeJSONError(w, "Failed to save config: "+err.Error())
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("Saved %d excluded domains", len(body.Domains))})
}

// --- Helpers ---

func writeJSONError(w http.ResponseWriter, msg string) {
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func getSourceLabel(source string) string {
	labels := map[string]string{
		"github":          "GH",
		"gitlab":          "GL",
		"calendar":        "CAL",
		"youtrack":        "YT",
		"macos_system":    "MAC",
		"browser_history": "WEB",
		"webhooks":        "HOOK",
	}
	if label, ok := labels[source]; ok {
		return label
	}
	if len(source) >= 3 {
		return strings.ToUpper(source[:3])
	}
	return strings.ToUpper(source)
}

func getEnabledConnectors(configManager *config.Manager, registry *connectors.ConnectorRegistry) map[string]connectors.Connector {
	enabled := make(map[string]connectors.Connector)
	appConfig := configManager.GetConfig()

	for name, connector := range registry.List() {
		if configManager.IsConnectorEnabled(name) {
			configWithAppSettings := make(map[string]interface{})
			connectorConfig, exists := configManager.GetConnectorConfig(name)
			if exists {
				for k, v := range connectorConfig.Config {
					configWithAppSettings[k] = v
				}
			}
			configWithAppSettings[connectors.CommonConfigKeys.LogLevel] = appConfig.App.LogLevel
			configWithAppSettings[connectors.CommonConfigKeys.DateFormat] = appConfig.App.DateFormat
			configWithAppSettings[connectors.CommonConfigKeys.Timeout] = 30

			if os.Getenv("ARKEO_DEBUG") != "" {
				configWithAppSettings[connectors.CommonConfigKeys.DebugMode] = true
			}

			if err := connector.Configure(configWithAppSettings); err != nil {
				continue
			}
			connector.SetEnabled(true)
			enabled[name] = connector
		}
	}
	return enabled
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

// --- Types ---

type pageData struct {
	ActivePage string
	Date       string
	Format     string
	Days       string
	Connectors []connectorInfo
}

type connectorInfo struct {
	Name        string
	Description string
	Enabled     bool
}

type activityView struct {
	Time        string `json:"time"`
	SourceLabel string `json:"source_label"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Duration    string `json:"duration"`
	Gap         string `json:"gap"`
}

// FormatDuration is a helper to format durations for display.
func init() {
	// Ensure template FS is embedded
	_ = templateFS
}