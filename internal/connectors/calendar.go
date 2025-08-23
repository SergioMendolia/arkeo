package connectors

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// CalendarConnector implements the Connector interface for Google Calendar using iCal feeds
type CalendarConnector struct {
	*BaseConnector
}

// NewCalendarConnector creates a new calendar connector
func NewCalendarConnector() *CalendarConnector {
	return &CalendarConnector{
		BaseConnector: NewBaseConnector(
			"calendar",
			"Fetches Google Calendar events using secret iCal URLs",
		),
	}
}

// GetHTTPClient is now used directly from BaseConnector

// GetRequiredConfig returns the required configuration for calendar
func (c *CalendarConnector) GetRequiredConfig() []ConfigField {
	requiredFields := []ConfigField{
		{
			Key:         "ical_urls",
			Type:        "string",
			Required:    true,
			Description: "Comma-separated list of Google Calendar secret iCal URLs",
		},
		{
			Key:         "include_declined",
			Type:        "bool",
			Required:    false,
			Description: "Include declined events",
			Default:     false,
		},
	}

	// Merge with common fields
	return MergeConfigFields(requiredFields)
}

// ValidateConfig validates the calendar configuration
func (c *CalendarConnector) ValidateConfig(config map[string]interface{}) error {
	// First use the common validation helper that checks all required fields
	if err := ValidateConfigFields(config, c.GetRequiredConfig()); err != nil {
		return err
	}

	// Additional calendar-specific validation
	icalURLs := config["ical_urls"].(string)

	// Validate that URLs look like Google Calendar iCal URLs
	urls := c.parseICalURLs(icalURLs)
	for _, url := range urls {
		if !c.isValidGoogleCalendarURL(url) {
			return fmt.Errorf("invalid Google Calendar iCal URL: %s", url)
		}
	}

	return nil
}

// isDebugMode checks if debug logging is enabled
func (c *CalendarConnector) isDebugMode() bool {
	return c.BaseConnector.IsDebugMode()
}

// TestConnection tests the calendar connection
func (c *CalendarConnector) TestConnection(ctx context.Context) error {
	urls := c.parseICalURLs(c.GetConfigString("ical_urls"))

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Testing connection to %d calendar(s)", len(urls))
	}

	for i, url := range urls {
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Testing calendar %d: %s", i+1, c.maskURL(url))
		}
		if err := c.testICalURL(ctx, url); err != nil {
			if c.isDebugMode() {
				log.Printf("Calendar Debug: Failed to connect to calendar %d: %v", i+1, err)
			}
			return fmt.Errorf("failed to connect to calendar %d (%s): %v", i+1, c.maskURL(url), err)
		}
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Successfully connected to calendar %d", i+1)
		}
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: All %d calendar(s) connected successfully", len(urls))
	}

	return nil
}

// GetActivities retrieves calendar activities for the specified date
func (c *CalendarConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	urls := c.parseICalURLs(c.GetConfigString("ical_urls"))
	var allActivities []timeline.Activity
	seenEventUIDs := make(map[string]bool) // Track seen event UIDs to prevent duplicates

	// Use timeout from configuration
	timeout := c.GetConfigInt(CommonConfigKeys.Timeout)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Fetching events for date %s from %d calendar(s)", date.Format("2006-01-02"), len(urls))
	}

	for i, url := range urls {
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Processing calendar %d: %s", i+1, c.maskURL(url))
		}
		activities, err := c.fetchCalendarEvents(ctx, url, date)
		if err != nil {
			if c.isDebugMode() {
				log.Printf("Calendar Debug: Failed to fetch events from calendar %d: %v", i+1, err)
			}
			// Log error but continue with other calendars
			continue
		}
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Found %d events from calendar %d", len(activities), i+1)
		}

		// Add activities, filtering out duplicates based on event UID
		duplicatesSkipped := 0
		for _, activity := range activities {
			// Extract event UID from the activity metadata
			if eventUID, exists := activity.Metadata["event_id"]; exists && eventUID != "" {
				if seenEventUIDs[eventUID] {
					duplicatesSkipped++
					if c.isDebugMode() {
						log.Printf("Calendar Debug: Skipping duplicate event '%s' with UID: %s", activity.Title, eventUID)
					}
					continue
				}
				seenEventUIDs[eventUID] = true
				allActivities = append(allActivities, activity)
			} else {
				// Handle activities without event_id or with empty UID (shouldn't happen with valid iCal data)
				if c.isDebugMode() {
					if !exists {
						log.Printf("Calendar Debug: Warning - activity '%s' has no event_id, adding without deduplication", activity.Title)
					} else {
						log.Printf("Calendar Debug: Warning - activity '%s' has empty event_id, adding without deduplication", activity.Title)
					}
				}
				allActivities = append(allActivities, activity)
			}
		}

		if c.isDebugMode() && duplicatesSkipped > 0 {
			log.Printf("Calendar Debug: Skipped %d duplicate events from calendar %d", duplicatesSkipped, i+1)
		}
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Total activities found: %d (after deduplication)", len(allActivities))
		log.Printf("Calendar Debug: Unique events tracked: %d", len(seenEventUIDs))
	}

	return allActivities, nil
}

// parseICalURLs splits the comma-separated URLs and trims whitespace
func (c *CalendarConnector) parseICalURLs(urls string) []string {
	if urls == "" {
		return []string{}
	}

	parts := strings.Split(urls, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// isValidGoogleCalendarURL checks if the URL looks like a valid Google Calendar iCal URL
func (c *CalendarConnector) isValidGoogleCalendarURL(url string) bool {
	// Google Calendar iCal URLs follow this pattern:
	// https://calendar.google.com/calendar/ical/[calendar-id]/[secret]/basic.ics
	pattern := `^https://calendar\.google\.com/calendar/ical/[^/]+/[^/]+/basic\.ics$`
	matched, _ := regexp.MatchString(pattern, url)
	return matched
}

// maskURL masks the secret part of the URL for logging
func (c *CalendarConnector) maskURL(url string) string {
	// Replace the secret part with asterisks
	re := regexp.MustCompile(`(https://calendar\.google\.com/calendar/ical/[^/]+/)([^/]+)(/basic\.ics)`)
	return re.ReplaceAllString(url, "${1}***${3}")
}

// testICalURL tests connectivity to a single iCal URL
func (c *CalendarConnector) testICalURL(ctx context.Context, url string) error {
	if c.isDebugMode() {
		log.Printf("Calendar Debug: Making HTTP request to %s", c.maskURL(url))
	}

	req, err := c.CreateRequest(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.GetHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Response status: %d %s", resp.StatusCode, resp.Status)
		log.Printf("Calendar Debug: Content-Type: %s", resp.Header.Get("Content-Type"))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: failed to fetch calendar data", resp.StatusCode)
	}

	// Check if response looks like iCal data
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/calendar") && !strings.Contains(contentType, "text/plain") {
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Unexpected content type: %s", contentType)
		}
		return fmt.Errorf("unexpected content type: %s", contentType)
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Successfully validated iCal response")
	}

	return nil
}

// fetchCalendarEvents fetches and parses events from an iCal URL for a specific date
func (c *CalendarConnector) fetchCalendarEvents(ctx context.Context, url string, date time.Time) ([]timeline.Activity, error) {
	if c.isDebugMode() {
		log.Printf("Calendar Debug: Fetching calendar data from %s", c.maskURL(url))
	}

	req, err := c.CreateRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if c.isDebugMode() {
		log.Printf("Calendar Debug: HTTP response: %d %s", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: failed to fetch calendar data", resp.StatusCode)
	}

	events, err := c.parseICalData(resp.Body)
	if err != nil {
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Failed to parse iCal data: %v", err)
		}
		return nil, err
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Parsed %d total events from iCal data", len(events))
	}

	// Filter events for the specific date
	var activities []timeline.Activity
	targetDate := date.Format("2006-01-02")
	includeDeclined := c.GetConfigBool("include_declined")
	filteredCount := 0
	declinedSkipped := 0

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Filtering events for target date: %s", targetDate)
		log.Printf("Calendar Debug: Include declined events: %t", includeDeclined)
	}

	for _, event := range events {
		eventDate := event.StartTime.Format("2006-01-02")
		if eventDate != targetDate {
			continue
		}
		filteredCount++

		// Skip declined events if configured to do so
		if !includeDeclined && event.Status == "DECLINED" {
			declinedSkipped++
			if c.isDebugMode() {
				log.Printf("Calendar Debug: Skipping declined event: %s", event.Summary)
			}
			continue
		}

		duration := event.EndTime.Sub(event.StartTime)

		metadata := map[string]string{
			"event_id": event.UID,
			"status":   event.Status,
		}

		if event.Location != "" {
			metadata["location"] = event.Location
		}
		if event.Organizer != "" {
			metadata["organizer"] = event.Organizer
		}

		activity := timeline.Activity{
			ID:          fmt.Sprintf("calendar-google-%s", event.UID),
			Type:        timeline.ActivityTypeCalendar,
			Title:       event.Summary,
			Description: event.Description,
			Timestamp:   event.StartTime,
			Duration:    &duration,
			Source:      "calendar",
			URL:         event.URL,
			Metadata:    metadata,
		}

		activities = append(activities, activity)

		if c.isDebugMode() {
			log.Printf("Calendar Debug: Added event: %s at %s (duration: %v)",
				event.Summary, event.StartTime.Format("15:04"), duration)
		}
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Events for target date: %d", filteredCount)
		log.Printf("Calendar Debug: Declined events skipped: %d", declinedSkipped)
		log.Printf("Calendar Debug: Final activities created: %d", len(activities))
	}

	return activities, nil
}

// ICalEvent represents a parsed iCal event
type ICalEvent struct {
	UID         string
	Summary     string
	Description string
	Location    string
	StartTime   time.Time
	EndTime     time.Time
	Status      string
	Organizer   string
	URL         string
}

// parseICalData parses iCal format data and extracts events
func (c *CalendarConnector) parseICalData(body interface{}) ([]ICalEvent, error) {
	var events []ICalEvent
	var currentEvent *ICalEvent
	lineCount := 0
	eventCount := 0

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Starting to parse iCal data")
	}

	scanner := bufio.NewScanner(body.(interface{ Read([]byte) (int, error) }))

	for scanner.Scan() {
		lineCount++
		line := strings.TrimSpace(scanner.Text())

		if line == "BEGIN:VEVENT" {
			currentEvent = &ICalEvent{}
			eventCount++
			if c.isDebugMode() {
				log.Printf("Calendar Debug: Found event %d at line %d", eventCount, lineCount)
			}
		} else if line == "END:VEVENT" && currentEvent != nil {
			if currentEvent.Summary != "" {
				events = append(events, *currentEvent)
				if c.isDebugMode() {
					log.Printf("Calendar Debug: Completed parsing event: %s (Start: %s)",
						currentEvent.Summary, currentEvent.StartTime.Format("2006-01-02 15:04"))
				}
			} else {
				if c.isDebugMode() {
					log.Printf("Calendar Debug: Skipping event with empty summary")
				}
			}
			currentEvent = nil
		} else if currentEvent != nil {
			c.parseICalLine(line, currentEvent)
		}
	}

	if err := scanner.Err(); err != nil {
		if c.isDebugMode() {
			log.Printf("Calendar Debug: Scanner error: %v", err)
		}
		return nil, err
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Parsed %d lines, found %d events, extracted %d valid events",
			lineCount, eventCount, len(events))
	}

	return events, nil
}

// parseICalLine parses a single line from iCal data
func (c *CalendarConnector) parseICalLine(line string, event *ICalEvent) {
	// Handle line folding (lines starting with space or tab are continuations)
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return // Skip folded lines for simplicity
	}

	// Split property and value
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}

	property := parts[0]
	value := parts[1]

	// Handle properties with parameters (e.g., "DTSTART;TZID=America/New_York")
	propParts := strings.SplitN(property, ";", 2)
	propName := propParts[0]

	switch propName {
	case "UID":
		event.UID = value
	case "SUMMARY":
		event.Summary = c.unescapeICalValue(value)
	case "DESCRIPTION":
		event.Description = c.unescapeICalValue(value)
	case "LOCATION":
		event.Location = c.unescapeICalValue(value)
	case "DTSTART":
		if t, err := c.parseICalDateTime(value); err == nil {
			event.StartTime = t
		} else if c.isDebugMode() {
			log.Printf("Calendar Debug: Failed to parse DTSTART '%s': %v", value, err)
		}
	case "DTEND":
		if t, err := c.parseICalDateTime(value); err == nil {
			event.EndTime = t
		} else if c.isDebugMode() {
			log.Printf("Calendar Debug: Failed to parse DTEND '%s': %v", value, err)
		}
	case "STATUS":
		event.Status = value
	case "ORGANIZER":
		// Extract email from ORGANIZER field (format: "ORGANIZER:mailto:email@example.com")
		if strings.HasPrefix(value, "mailto:") {
			event.Organizer = strings.TrimPrefix(value, "mailto:")
		} else {
			event.Organizer = value
		}
	case "URL":
		event.URL = value
	}
}

// parseICalDateTime parses iCal date/time formats
func (c *CalendarConnector) parseICalDateTime(value string) (time.Time, error) {
	// Handle different iCal date/time formats
	formats := []string{
		"20060102T150405Z", // UTC format
		"20060102T150405",  // Local format
		"20060102",         // Date only
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			if c.isDebugMode() {
				log.Printf("Calendar Debug: Successfully parsed date/time '%s' using format '%s' -> %s",
					value, format, t.Format("2006-01-02 15:04:05"))
			}
			return t, nil
		}
	}

	if c.isDebugMode() {
		log.Printf("Calendar Debug: Failed to parse date/time with all formats: %s", value)
	}

	return time.Time{}, fmt.Errorf("unable to parse date/time: %s", value)
}

// unescapeICalValue unescapes iCal text values
func (c *CalendarConnector) unescapeICalValue(value string) string {
	// Unescape common iCal escape sequences
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\\,", ",")
	value = strings.ReplaceAll(value, "\\;", ";")
	value = strings.ReplaceAll(value, "\\\\", "\\")
	return value
}
