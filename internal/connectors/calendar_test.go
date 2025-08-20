package connectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

func TestNewCalendarConnector(t *testing.T) {
	connector := NewCalendarConnector()

	if connector == nil {
		t.Fatal("NewCalendarConnector returned nil")
	}

	if connector.Name() != "calendar" {
		t.Errorf("Expected name 'calendar', got %q", connector.Name())
	}

	expectedDescription := "Fetches Google Calendar events using secret iCal URLs"
	if connector.Description() != expectedDescription {
		t.Errorf("Expected description %q, got %q", expectedDescription, connector.Description())
	}

	if connector.IsEnabled() {
		t.Error("New connector should be disabled by default")
	}

	if connector.httpClient == nil {
		t.Error("HTTP client should be initialized")
	}

	if connector.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", connector.httpClient.Timeout)
	}
}

func TestCalendarConnector_GetRequiredConfig(t *testing.T) {
	connector := NewCalendarConnector()
	config := connector.GetRequiredConfig()

	expectedFields := []struct {
		key      string
		typ      string
		required bool
		def      string
	}{
		{"ical_urls", "string", true, ""},
		{"include_declined", "bool", false, "false"},
	}

	if len(config) != len(expectedFields) {
		t.Errorf("Expected %d config fields, got %d", len(expectedFields), len(config))
	}

	for i, expected := range expectedFields {
		if i >= len(config) {
			t.Errorf("Missing config field at index %d", i)
			continue
		}

		field := config[i]
		if field.Key != expected.key {
			t.Errorf("Field %d: expected key %q, got %q", i, expected.key, field.Key)
		}

		if field.Type != expected.typ {
			t.Errorf("Field %d: expected type %q, got %q", i, expected.typ, field.Type)
		}

		if field.Required != expected.required {
			t.Errorf("Field %d: expected required %v, got %v", i, expected.required, field.Required)
		}

		if field.Default != expected.def {
			t.Errorf("Field %d: expected default %q, got %q", i, expected.def, field.Default)
		}
	}
}

func TestCalendarConnector_ValidateConfig(t *testing.T) {
	connector := NewCalendarConnector()

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"ical_urls": "https://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics",
			},
			expectError: false,
		},
		{
			name: "valid config with multiple URLs",
			config: map[string]interface{}{
				"ical_urls": "https://calendar.google.com/calendar/ical/test1%40gmail.com/private-abc123/basic.ics,https://calendar.google.com/calendar/ical/test2%40gmail.com/private-def456/basic.ics",
			},
			expectError: false,
		},
		{
			name: "valid config with include_declined",
			config: map[string]interface{}{
				"ical_urls":        "https://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics",
				"include_declined": true,
			},
			expectError: false,
		},
		{
			name: "missing ical_urls",
			config: map[string]interface{}{
				"include_declined": false,
			},
			expectError: true,
			errorMsg:    "ical_urls is required",
		},
		{
			name: "empty ical_urls",
			config: map[string]interface{}{
				"ical_urls": "",
			},
			expectError: true,
			errorMsg:    "ical_urls cannot be empty",
		},
		{
			name: "invalid URL format",
			config: map[string]interface{}{
				"ical_urls": "not-a-valid-url",
			},
			expectError: true,
			errorMsg:    "invalid iCal URL format: not-a-valid-url",
		},
		{
			name: "non-HTTPS URL",
			config: map[string]interface{}{
				"ical_urls": "http://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics",
			},
			expectError: true,
			errorMsg:    "iCal URL must use HTTPS: http://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := connector.ValidateConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestCalendarConnector_TestConnection(t *testing.T) {
	tests := []struct {
		name           string
		config         map[string]interface{}
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful connection",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/calendar")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR\n"))
			},
			expectError: false,
		},
		{
			name: "server error",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError:   true,
			errorContains: "failed to fetch iCal data",
		},
		{
			name: "not found",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError:   true,
			errorContains: "iCal URL not found",
		},
		{
			name:   "no config",
			config: map[string]interface{}{},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError:   true,
			errorContains: "ical_urls not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewTLSServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			connector := NewCalendarConnector()

			// Configure the connector
			if len(tt.config) > 0 && tt.name != "no config" {
				tt.config["ical_urls"] = server.URL
				err := connector.Configure(tt.config)
				if err != nil {
					t.Fatalf("Failed to configure connector: %v", err)
				}
			}

			ctx := context.Background()
			err := connector.TestConnection(ctx)

			if tt.expectError {
				if err == nil {
					t.Error("Expected connection test to fail")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected connection test to succeed, got: %v", err)
				}
			}
		})
	}
}

func TestCalendarConnector_GetActivities(t *testing.T) {
	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name               string
		config             map[string]interface{}
		icalContent        string
		expectedActivities int
		expectError        bool
		errorContains      string
	}{
		{
			name: "successful fetch with events",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			icalContent: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20240115T090000Z
DTEND:20240115T100000Z
SUMMARY:Morning Standup
DESCRIPTION:Daily team meeting
LOCATION:Conference Room A
STATUS:CONFIRMED
END:VEVENT
BEGIN:VEVENT
UID:test-event-2@example.com
DTSTART:20240115T140000Z
DTEND:20240115T150000Z
SUMMARY:Code Review
DESCRIPTION:Review pull requests
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`,
			expectedActivities: 2,
			expectError:        false,
		},
		{
			name: "no events for date",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			icalContent: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20240114T090000Z
DTEND:20240114T100000Z
SUMMARY:Yesterday's Meeting
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`,
			expectedActivities: 0,
			expectError:        false,
		},
		{
			name: "declined event excluded by default",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			icalContent: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20240115T090000Z
DTEND:20240115T100000Z
SUMMARY:Declined Meeting
STATUS:DECLINED
END:VEVENT
END:VCALENDAR`,
			expectedActivities: 0,
			expectError:        false,
		},
		{
			name: "declined event included when configured",
			config: map[string]interface{}{
				"ical_urls":        "", // Will be set to test server URL
				"include_declined": true,
			},
			icalContent: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20240115T090000Z
DTEND:20240115T100000Z
SUMMARY:Declined Meeting
STATUS:DECLINED
END:VEVENT
END:VCALENDAR`,
			expectedActivities: 1,
			expectError:        false,
		},
		{
			name: "all-day event",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			icalContent: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART;VALUE=DATE:20240115
SUMMARY:All Day Event
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`,
			expectedActivities: 1,
			expectError:        false,
		},
		{
			name: "invalid ical content",
			config: map[string]interface{}{
				"ical_urls": "", // Will be set to test server URL
			},
			icalContent:        "not valid ical content",
			expectedActivities: 0,
			expectError:        false, // Should not error, just return no events
		},
		{
			name:   "no configuration",
			config: map[string]interface{}{},
			icalContent: `BEGIN:VCALENDAR
VERSION:2.0
END:VCALENDAR`,
			expectedActivities: 0,
			expectError:        true,
			errorContains:      "ical_urls not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/calendar")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.icalContent))
			}))
			defer server.Close()

			connector := NewCalendarConnector()

			// Configure the connector
			if len(tt.config) > 0 && tt.name != "no configuration" {
				tt.config["ical_urls"] = server.URL
				err := connector.Configure(tt.config)
				if err != nil {
					t.Fatalf("Failed to configure connector: %v", err)
				}
			}

			ctx := context.Background()
			activities, err := connector.GetActivities(ctx, testDate)

			if tt.expectError {
				if err == nil {
					t.Error("Expected GetActivities to fail")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected GetActivities to succeed, got: %v", err)
				}
			}

			if len(activities) != tt.expectedActivities {
				t.Errorf("Expected %d activities, got %d", tt.expectedActivities, len(activities))
			}

			// Verify activity structure if we got activities
			if len(activities) > 0 {
				activity := activities[0]
				if activity.Type != timeline.ActivityTypeCalendar {
					t.Errorf("Expected activity type %s, got %s", timeline.ActivityTypeCalendar, activity.Type)
				}

				if activity.Source != "calendar" {
					t.Errorf("Expected source 'calendar', got %q", activity.Source)
				}

				if activity.Title == "" {
					t.Error("Activity title should not be empty")
				}

				if activity.Timestamp.IsZero() {
					t.Error("Activity timestamp should not be zero")
				}

				// Check if the timestamp is on the correct date
				if !isSameDayTest(activity.Timestamp, testDate) {
					t.Errorf("Activity timestamp %v is not on the expected date %v", activity.Timestamp, testDate)
				}
			}
		})
	}
}

func TestCalendarConnector_MultipleURLs(t *testing.T) {
	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create two test servers with different events
	server1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test1
BEGIN:VEVENT
UID:server1-event@example.com
DTSTART:20240115T090000Z
DTEND:20240115T100000Z
SUMMARY:Server 1 Event
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`))
	}))
	defer server1.Close()

	server2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test2
BEGIN:VEVENT
UID:server2-event@example.com
DTSTART:20240115T140000Z
DTEND:20240115T150000Z
SUMMARY:Server 2 Event
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`))
	}))
	defer server2.Close()

	connector := NewCalendarConnector()
	config := map[string]interface{}{
		"ical_urls": server1.URL + "," + server2.URL,
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	ctx := context.Background()
	activities, err := connector.GetActivities(ctx, testDate)

	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}

	if len(activities) != 2 {
		t.Errorf("Expected 2 activities from both servers, got %d", len(activities))
	}

	// Verify we got events from both servers
	titles := make(map[string]bool)
	for _, activity := range activities {
		titles[activity.Title] = true
	}

	if !titles["Server 1 Event"] {
		t.Error("Missing event from server 1")
	}

	if !titles["Server 2 Event"] {
		t.Error("Missing event from server 2")
	}
}

func TestCalendarConnector_EventDuration(t *testing.T) {
	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	icalContent := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:Test
BEGIN:VEVENT
UID:timed-event@example.com
DTSTART:20240115T090000Z
DTEND:20240115T103000Z
SUMMARY:90 Minute Meeting
STATUS:CONFIRMED
END:VEVENT
BEGIN:VEVENT
UID:allday-event@example.com
DTSTART;VALUE=DATE:20240115
SUMMARY:All Day Event
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalContent))
	}))
	defer server.Close()

	connector := NewCalendarConnector()
	config := map[string]interface{}{
		"ical_urls": server.URL,
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure connector: %v", err)
	}

	ctx := context.Background()
	activities, err := connector.GetActivities(ctx, testDate)

	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}

	if len(activities) != 2 {
		t.Errorf("Expected 2 activities, got %d", len(activities))
	}

	for _, activity := range activities {
		if activity.Title == "90 Minute Meeting" {
			if activity.Duration == nil {
				t.Error("Timed event should have duration")
			} else if *activity.Duration != 90*time.Minute {
				t.Errorf("Expected duration 90m, got %v", *activity.Duration)
			}
		} else if activity.Title == "All Day Event" {
			if activity.Duration != nil {
				t.Error("All day event should not have duration")
			}
		}
	}
}

func TestCalendarConnector_ConfigureFlow(t *testing.T) {
	connector := NewCalendarConnector()

	// Test initial state
	if connector.IsEnabled() {
		t.Error("Connector should start disabled")
	}

	// Configure
	config := map[string]interface{}{
		"ical_urls":        "https://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics",
		"include_declined": true,
	}

	err := connector.Configure(config)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	// Verify configuration
	expectedURL := "https://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics"
	if connector.GetConfigString("ical_urls") != expectedURL {
		t.Error("ical_urls not configured correctly")
	}

	if !connector.GetConfigBool("include_declined") {
		t.Error("include_declined not configured correctly")
	}

	// Enable connector
	connector.SetEnabled(true)
	if !connector.IsEnabled() {
		t.Error("Connector should be enabled")
	}
}

func TestCalendarConnector_HTTPTimeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR\n"))
	}))
	defer server.Close()

	connector := NewCalendarConnector()

	// Set a very short timeout for testing
	connector.httpClient.Timeout = 10 * time.Millisecond

	config := map[string]interface{}{
		"ical_urls": server.URL,
	}
	connector.Configure(config)

	ctx := context.Background()
	_, err := connector.GetActivities(ctx, time.Now())

	if err == nil {
		t.Error("Expected timeout error")
	}
}

// Helper functions

func isSameDayTest(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// Benchmark tests
func BenchmarkCalendarConnector_ValidateConfig(b *testing.B) {
	connector := NewCalendarConnector()
	config := map[string]interface{}{
		"ical_urls": "https://calendar.google.com/calendar/ical/test%40gmail.com/private-abc123/basic.ics",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connector.ValidateConfig(config)
	}
}

func BenchmarkCalendarConnector_GetRequiredConfig(b *testing.B) {
	connector := NewCalendarConnector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connector.GetRequiredConfig()
	}
}
