package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

// CalendarConnector implements the Connector interface for calendar services
type CalendarConnector struct {
	*BaseConnector
	httpClient *http.Client
}

// NewCalendarConnector creates a new calendar connector
func NewCalendarConnector() *CalendarConnector {
	return &CalendarConnector{
		BaseConnector: NewBaseConnector(
			"calendar",
			"Fetches calendar events and meetings",
		),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetRequiredConfig returns the required configuration for calendar
func (c *CalendarConnector) GetRequiredConfig() []ConfigField {
	return []ConfigField{
		{
			Key:         "provider",
			Type:        "string",
			Required:    true,
			Description: "Calendar provider (google, outlook, caldav)",
			Default:     "google",
		},
		{
			Key:         "client_id",
			Type:        "string",
			Required:    true,
			Description: "OAuth Client ID",
		},
		{
			Key:         "client_secret",
			Type:        "secret",
			Required:    true,
			Description: "OAuth Client Secret",
		},
		{
			Key:         "refresh_token",
			Type:        "secret",
			Required:    false,
			Description: "OAuth Refresh Token (will be obtained during auth)",
		},
		{
			Key:         "calendar_ids",
			Type:        "string",
			Required:    false,
			Description: "Comma-separated list of calendar IDs (default: primary)",
			Default:     "primary",
		},
		{
			Key:         "include_declined",
			Type:        "bool",
			Required:    false,
			Description: "Include declined events",
			Default:     "false",
		},
	}
}

// ValidateConfig validates the calendar configuration
func (c *CalendarConnector) ValidateConfig(config map[string]interface{}) error {
	provider, ok := config["provider"].(string)
	if !ok || provider == "" {
		return fmt.Errorf("calendar provider is required")
	}

	validProviders := []string{"google", "outlook", "caldav"}
	isValid := false
	for _, valid := range validProviders {
		if provider == valid {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid provider: %s. Must be one of: %s", provider, strings.Join(validProviders, ", "))
	}

	clientID, ok := config["client_id"].(string)
	if !ok || clientID == "" {
		return fmt.Errorf("client_id is required")
	}

	clientSecret, ok := config["client_secret"].(string)
	if !ok || clientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}

	return nil
}

// TestConnection tests the calendar connection
func (c *CalendarConnector) TestConnection(ctx context.Context) error {
	provider := c.GetConfigString("provider")

	switch provider {
	case "google":
		return c.testGoogleConnection(ctx)
	case "outlook":
		return c.testOutlookConnection(ctx)
	case "caldav":
		return c.testCalDAVConnection(ctx)
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
}

// GetActivities retrieves calendar activities for the specified date
func (c *CalendarConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	provider := c.GetConfigString("provider")

	switch provider {
	case "google":
		return c.getGoogleCalendarEvents(ctx, date)
	case "outlook":
		return c.getOutlookCalendarEvents(ctx, date)
	case "caldav":
		return c.getCalDAVEvents(ctx, date)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// testGoogleConnection tests Google Calendar connection
func (c *CalendarConnector) testGoogleConnection(ctx context.Context) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/calendar/v3/calendars/primary", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google calendar API returned status %d", resp.StatusCode)
	}

	return nil
}

// testOutlookConnection tests Outlook Calendar connection
func (c *CalendarConnector) testOutlookConnection(ctx context.Context) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me/calendars", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("outlook calendar API returned status %d", resp.StatusCode)
	}

	return nil
}

// testCalDAVConnection tests CalDAV connection
func (c *CalendarConnector) testCalDAVConnection(ctx context.Context) error {
	// CalDAV implementation would go here
	return fmt.Errorf("CalDAV support not yet implemented")
}

// getGoogleCalendarEvents retrieves events from Google Calendar
func (c *CalendarConnector) getGoogleCalendarEvents(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	calendarIDs := c.getCalendarIDs()
	var allActivities []timeline.Activity

	for _, calendarID := range calendarIDs {
		events, err := c.fetchGoogleEvents(ctx, token, calendarID, date)
		if err != nil {
			// Log error but continue with other calendars
			continue
		}
		allActivities = append(allActivities, events...)
	}

	return allActivities, nil
}

// fetchGoogleEvents fetches events from a specific Google calendar
func (c *CalendarConnector) fetchGoogleEvents(ctx context.Context, token, calendarID string, date time.Time) ([]timeline.Activity, error) {
	timeMin := date.Format(time.RFC3339)
	timeMax := date.Add(24 * time.Hour).Format(time.RFC3339)

	params := url.Values{}
	params.Set("timeMin", timeMin)
	params.Set("timeMax", timeMax)
	params.Set("singleEvents", "true")
	params.Set("orderBy", "startTime")
	params.Set("maxResults", "250")

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?%s",
		url.QueryEscape(calendarID), params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google calendar API returned status %d", resp.StatusCode)
	}

	var calendarResponse struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			Status      string `json:"status"`
			HTMLLink    string `json:"htmlLink"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
			Attendees []struct {
				Email          string `json:"email"`
				ResponseStatus string `json:"responseStatus"`
				Self           bool   `json:"self"`
			} `json:"attendees"`
			Creator struct {
				Email string `json:"email"`
			} `json:"creator"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&calendarResponse); err != nil {
		return nil, err
	}

	var activities []timeline.Activity
	includeDeclined := c.GetConfigBool("include_declined")

	for _, event := range calendarResponse.Items {
		// Skip declined events if configured to do so
		if !includeDeclined {
			for _, attendee := range event.Attendees {
				if attendee.Self && attendee.ResponseStatus == "declined" {
					continue
				}
			}
		}

		// Parse start and end times
		var startTime, endTime time.Time
		var err error

		if event.Start.DateTime != "" {
			startTime, err = time.Parse(time.RFC3339, event.Start.DateTime)
		} else if event.Start.Date != "" {
			startTime, err = time.Parse("2006-01-02", event.Start.Date)
		}
		if err != nil {
			continue
		}

		if event.End.DateTime != "" {
			endTime, err = time.Parse(time.RFC3339, event.End.DateTime)
		} else if event.End.Date != "" {
			endTime, err = time.Parse("2006-01-02", event.End.Date)
		}
		if err != nil {
			continue
		}

		duration := endTime.Sub(startTime)

		tags := []string{"calendar", "meeting"}
		if event.Location != "" {
			tags = append(tags, "in-person")
		} else {
			tags = append(tags, "virtual")
		}

		metadata := map[string]string{
			"calendar_id": calendarID,
			"event_id":    event.ID,
			"status":      event.Status,
		}

		if event.Location != "" {
			metadata["location"] = event.Location
		}
		if event.Creator.Email != "" {
			metadata["creator"] = event.Creator.Email
		}

		activity := timeline.Activity{
			ID:          fmt.Sprintf("calendar-google-%s", event.ID),
			Type:        timeline.ActivityTypeCalendar,
			Title:       event.Summary,
			Description: event.Description,
			Timestamp:   startTime,
			Duration:    &duration,
			Source:      "calendar",
			URL:         event.HTMLLink,
			Tags:        tags,
			Metadata:    metadata,
		}

		activities = append(activities, activity)
	}

	return activities, nil
}

// getOutlookCalendarEvents retrieves events from Outlook Calendar
func (c *CalendarConnector) getOutlookCalendarEvents(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	// Outlook implementation would go here - similar structure to Google
	return nil, fmt.Errorf("Outlook calendar support not yet implemented")
}

// getCalDAVEvents retrieves events from CalDAV server
func (c *CalendarConnector) getCalDAVEvents(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	// CalDAV implementation would go here
	return nil, fmt.Errorf("CalDAV support not yet implemented")
}

// getAccessToken retrieves or refreshes the access token
func (c *CalendarConnector) getAccessToken(ctx context.Context) (string, error) {
	// This would implement OAuth token refresh logic
	// For now, assume we have a valid refresh token stored
	refreshToken := c.GetConfigString("refresh_token")
	if refreshToken == "" {
		return "", fmt.Errorf("no refresh token available - please re-authenticate")
	}

	// TODO: Implement token refresh logic
	// This is a placeholder - in a real implementation you would:
	// 1. Use the refresh token to get a new access token
	// 2. Handle token expiration
	// 3. Store the new tokens securely

	return "", fmt.Errorf("token refresh not yet implemented")
}

// getCalendarIDs returns the list of calendar IDs to fetch from
func (c *CalendarConnector) getCalendarIDs() []string {
	calendarIDsStr := c.GetConfigString("calendar_ids")
	if calendarIDsStr == "" {
		return []string{"primary"}
	}

	ids := strings.Split(calendarIDsStr, ",")
	for i, id := range ids {
		ids[i] = strings.TrimSpace(id)
	}

	return ids
}
