package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// YouTrackConnector implements the Connector interface for YouTrack
type YouTrackConnector struct {
	*BaseConnector
}

// NewYouTrackConnector creates a new YouTrack connector
func NewYouTrackConnector() *YouTrackConnector {
	return &YouTrackConnector{
		BaseConnector: NewBaseConnector(
			"youtrack",
			"Fetches activities and issue updates from YouTrack",
		),
	}
}

// GetHTTPClient is now used directly from BaseConnector

// GetRequiredConfig returns the required configuration for YouTrack
func (y *YouTrackConnector) GetRequiredConfig() []ConfigField {
	return []ConfigField{
		{
			Key:         "base_url",
			Type:        "string",
			Required:    true,
			Description: "YouTrack base URL (e.g., https://mycompany.youtrack.cloud/)",
		},
		{
			Key:         "token",
			Type:        "secret",
			Required:    true,
			Description: "YouTrack permanent token",
		},
		{
			Key:         "username",
			Type:        "string",
			Required:    false,
			Description: "Username to filter activities for (defaults to token owner)",
		},
		{
			Key:         "include_work_items",
			Type:        "bool",
			Required:    false,
			Description: "Include work items (time tracking entries)",
			Default:     "true",
		},
		{
			Key:         "include_comments",
			Type:        "bool",
			Required:    false,
			Description: "Include comment activities",
			Default:     "true",
		},
		{
			Key:         "include_issues",
			Type:        "bool",
			Required:    false,
			Description: "Include issue field changes",
			Default:     "true",
		},
		{
			Key:         "api_fields",
			Type:        "string",
			Required:    false,
			Description: "Custom API fields to request (for compatibility with different YouTrack versions)",
			Default:     "id,timestamp,author(login,name),category(id,name),target(id,idReadable,summary,project(name,shortName),issue(id,idReadable,summary,project(name,shortName))),field(name),added,removed",
		},
		{
			Key:         "log_level",
			Type:        "string",
			Required:    false,
			Description: "Enable debug logging (set to 'debug')",
			Default:     "info",
		},
	}
}

// isDebugMode checks if debug logging is enabled
func (y *YouTrackConnector) isDebugMode() bool {
	// Use the base connector implementation
	return y.BaseConnector.IsDebugMode()
}

// IsDebugMode provides public access to debug mode status
func (y *YouTrackConnector) IsDebugMode() bool {
	return y.isDebugMode()
}

// ValidateConfig validates the YouTrack configuration
func (y *YouTrackConnector) ValidateConfig(config map[string]interface{}) error {
	baseURL, ok := config["base_url"].(string)
	if !ok || baseURL == "" {
		return fmt.Errorf("youtrack base_url is required")
	}

	// Ensure base URL has proper format
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return fmt.Errorf("youtrack base_url must start with http:// or https://")
	}

	// Ensure base URL ends with /
	if !strings.HasSuffix(baseURL, "/") {
		config["base_url"] = baseURL + "/"
	}

	token, ok := config["token"].(string)
	if !ok || token == "" {
		return fmt.Errorf("youtrack token is required")
	}

	// Validate URL is accessible format
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid YouTrack base_url format: %v", err)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("YouTrack base_url must include a valid host")
	}

	// Check for common YouTrack URL patterns
	if strings.Contains(baseURL, "youtrack") || strings.Contains(baseURL, "jetbrains") {
		// Looks like a YouTrack URL, good
	} else {
		// Could be a custom domain, that's fine too
		if parsedURL.Scheme == "" {
			return fmt.Errorf("YouTrack base_url must include protocol (http:// or https://)")
		}
	}

	return nil
}

// TestConnection tests the YouTrack connection
func (y *YouTrackConnector) TestConnection(ctx context.Context) error {
	baseURL := y.GetConfigString("base_url")
	token := y.GetConfigString("token")

	if baseURL == "" || token == "" {
		return fmt.Errorf("base_url and token must be configured")
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Testing connection to %s", baseURL)
	}

	// Test connection by checking API access and activities endpoint
	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Testing user authentication...")
	}

	// First test: Check user authentication
	userURL := baseURL + "api/admin/users/me?fields=id,login,name"
	if err := y.testAPIEndpoint(ctx, userURL, token, "user authentication"); err != nil {
		return err
	}

	// Second test: Check activities API access
	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Testing activities API access...")
	}

	activitiesURL := baseURL + "api/activities?per_page=1&fields=id,timestamp"
	if err := y.testAPIEndpoint(ctx, activitiesURL, token, "activities API"); err != nil {
		return err
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Successfully connected to YouTrack - all API endpoints accessible")
	}

	return nil
}

// testAPIEndpoint tests a specific API endpoint
func (y *YouTrackConnector) testAPIEndpoint(ctx context.Context, apiURL, token, endpointName string) error {
	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Testing %s endpoint: %s", endpointName, apiURL)
	}

	req, err := y.CreateBearerRequest(ctx, "GET", apiURL, token)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", endpointName, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := y.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to YouTrack %s: %w", endpointName, err)
	}
	defer resp.Body.Close()

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: %s response status: %d %s", endpointName, resp.StatusCode, resp.Status)
	}

	// Read response body for error details
	body, bodyErr := io.ReadAll(resp.Body)
	bodyStr := ""
	if bodyErr == nil {
		bodyStr = string(body)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		if y.isDebugMode() && bodyStr != "" {
			log.Printf("YouTrack Debug: Unauthorized response body: %s", bodyStr)
		}
		return fmt.Errorf("invalid YouTrack token or insufficient permissions for %s", endpointName)
	}

	if resp.StatusCode == http.StatusForbidden {
		if y.isDebugMode() && bodyStr != "" {
			log.Printf("YouTrack Debug: Forbidden response body: %s", bodyStr)
		}
		return fmt.Errorf("YouTrack token lacks required permissions for %s", endpointName)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("YouTrack %s endpoint not found - check your YouTrack version and URL", endpointName)
	}

	if resp.StatusCode == http.StatusBadRequest {
		if y.isDebugMode() && bodyStr != "" {
			log.Printf("YouTrack Debug: Bad request response body: %s", bodyStr)
		}
		return fmt.Errorf("YouTrack %s endpoint returned bad request (400): %s", endpointName, bodyStr)
	}

	if resp.StatusCode != http.StatusOK {
		if y.isDebugMode() && bodyStr != "" {
			log.Printf("YouTrack Debug: Error response body: %s", bodyStr)
		}
		return fmt.Errorf("YouTrack %s returned status %d: %s", endpointName, resp.StatusCode, bodyStr)
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: %s endpoint test successful", endpointName)
	}

	return nil
}

// GetActivities retrieves YouTrack activities for the specified date
func (y *YouTrackConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	var allActivities []timeline.Activity

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Fetching activities for date %s", date.Format("2006-01-02"))
	}

	// Get user info if username not specified
	username := y.GetConfigString("username")
	if username == "" {
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: No username specified, fetching current user")
		}
		user, err := y.getCurrentUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current user: %w", err)
		}
		username = user.Login
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Using username: %s", username)
		}
	}

	// Get activities for the specified date
	activities, err := y.getActivities(ctx, date, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get activities: %w", err)
	}

	allActivities = append(allActivities, activities...)

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Total activities found: %d", len(allActivities))
	}

	return allActivities, nil
}

// youTrackUser represents a YouTrack user
type youTrackUser struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// youTrackActivity represents a YouTrack activity item
type youTrackActivity struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Author    struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"author"`
	Category struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"category"`
	Target interface{} `json:"target"`
	Field  *struct {
		Name string `json:"name"`
	} `json:"field"`
	Added   interface{} `json:"added"`
	Removed interface{} `json:"removed"`
}

// youTrackIssue represents a YouTrack issue (simplified)
type youTrackIssue struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Project struct {
		Name      string `json:"name"`
		ShortName string `json:"shortName"`
	} `json:"project"`
}

// getCurrentUser fetches the current user information
func (y *YouTrackConnector) getCurrentUser(ctx context.Context) (*youTrackUser, error) {
	baseURL := y.GetConfigString("base_url")
	token := y.GetConfigString("token")

	apiURL := baseURL + "api/admin/users/me?fields=id,login,name,email"

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Fetching current user from %s", apiURL)
	}
	req, err := y.CreateBearerRequest(ctx, "GET", apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for current user: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := y.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Failed to get user info, status: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to get user info, status: %d", resp.StatusCode)
	}

	var user youTrackUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// getActivities fetches activities for a specific date and user
func (y *YouTrackConnector) getActivities(ctx context.Context, date time.Time, username string) ([]timeline.Activity, error) {
	baseURL := y.GetConfigString("base_url")
	token := y.GetConfigString("token")

	// Validate parameters
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	// Calculate start and end timestamps for the day
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	startTimestamp := startOfDay.Unix() * 1000 // YouTrack uses milliseconds
	endTimestamp := endOfDay.Unix() * 1000

	// Validate timestamps (YouTrack doesn't accept negative timestamps)
	if startTimestamp < 0 || endTimestamp < 0 {
		return nil, fmt.Errorf("invalid date range: timestamps cannot be negative")
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Fetching activities for user %s from %d to %d", username, startTimestamp, endTimestamp)
	}

	// Build query parameters
	params := url.Values{}

	// Use custom API fields if specified, otherwise use default
	apiFields := y.GetConfigString("api_fields")
	if apiFields == "" {
		apiFields = "id,timestamp,author(login,name),category(id,name),target(id,idReadable,summary,project(name,shortName),issue(id,idReadable,summary,project(name,shortName))),field(name),added(id,name,login,fullName,presentation,displayName,$type),removed(id,name,login,fullName,presentation,displayName,$type)"
	}
	params.Set("fields", apiFields)
	params.Set("author", username)
	params.Set("start", strconv.FormatInt(startTimestamp, 10))
	params.Set("end", strconv.FormatInt(endTimestamp, 10))

	// Add category filters based on configuration
	var categories []string
	if y.GetConfigBool("include_issues") {
		categories = append(categories, "CustomFieldCategory", "LinkCategory")
	}
	if y.GetConfigBool("include_comments") {
		categories = append(categories, "CommentsCategory")
	}
	if y.GetConfigBool("include_work_items") {
		categories = append(categories, "WorkItemCategory")
	}

	// YouTrack API requires at least one category to be specified
	if len(categories) == 0 {
		// Default to basic issue activities if nothing is enabled
		categories = append(categories, "CustomFieldCategory")
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: No categories specified, defaulting to CustomFieldCategory")
		}
	}

	params.Set("categories", strings.Join(categories, ","))
	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Including categories: %s", strings.Join(categories, ", "))
	}

	// Validate URL construction
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/activities?" + params.Encode()

	// Check URL length (some systems have URL length limits)
	if len(apiURL) > 2048 {
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Warning - URL length is %d characters, may exceed some limits", len(apiURL))
		}
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Making API request to %s", apiURL)
		log.Printf("YouTrack Debug: Request parameters:")
		log.Printf("  - fields: %s", params.Get("fields"))
		log.Printf("  - author: %s", params.Get("author"))
		log.Printf("  - start: %s", params.Get("start"))
		log.Printf("  - end: %s", params.Get("end"))
		if params.Get("categories") != "" {
			log.Printf("  - categories: %s", params.Get("categories"))
		}
		log.Printf("YouTrack Debug: URL length: %d characters", len(apiURL))

		// Show if custom API fields are being used
		customFields := y.GetConfigString("api_fields")
		if customFields != "" {
			log.Printf("YouTrack Debug: Using custom API fields configuration")
		}
	}

	req, err := y.CreateBearerRequest(ctx, "GET", apiURL, token)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := y.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for better error details
		body, bodyErr := io.ReadAll(resp.Body)
		bodyStr := ""
		if bodyErr == nil {
			bodyStr = string(body)
		}

		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Activities API returned status %d", resp.StatusCode)
			log.Printf("YouTrack Debug: Request URL: %s", apiURL)
			if bodyStr != "" {
				log.Printf("YouTrack Debug: Response body: %s", bodyStr)
			}
			log.Printf("YouTrack Debug: Request headers: Authorization=Bearer [hidden], Accept=application/json")
		}

		if resp.StatusCode == http.StatusBadRequest {
			// Provide specific guidance for common 400 error causes
			errorMsg := fmt.Sprintf("youtrack API bad request (400): %s", bodyStr)
			if strings.Contains(bodyStr, "Invalid field") {
				errorMsg += ". Check if the 'fields' parameter contains valid YouTrack field names"
			} else if strings.Contains(bodyStr, "author") {
				errorMsg += ". Check if the username exists in YouTrack"
			} else if strings.Contains(bodyStr, "categories") || strings.Contains(bodyStr, "No requested categories") {
				errorMsg += ". Check if the activity categories are valid for your YouTrack version, or try enabling at least one category type"
			} else if strings.Contains(bodyStr, "timestamp") || strings.Contains(bodyStr, "start") || strings.Contains(bodyStr, "end") {
				errorMsg += ". Check if the date range is valid"
			} else if bodyStr == "" {
				errorMsg = "youtrack API bad request (400). Common causes: invalid username, unsupported field names, incorrect date format, or missing activity categories"
			}
			return nil, fmt.Errorf("%s", errorMsg)
		}
		return nil, fmt.Errorf("youtrack activities API returned status %d: %s", resp.StatusCode, bodyStr)
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Successfully received API response")
	}

	var activities []youTrackActivity
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Failed to decode JSON response: %v", err)
		}
		return nil, err
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Parsed %d activities from API response", len(activities))
	}

	convertedActivities := y.convertActivities(activities)

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Converted %d activities for timeline", len(convertedActivities))
	}

	return convertedActivities, nil
}

// convertActivities converts YouTrack activities to timeline activities
func (y *YouTrackConnector) convertActivities(ytActivities []youTrackActivity) []timeline.Activity {
	var activities []timeline.Activity

	for i, ytActivity := range ytActivities {
		if y.isDebugMode() && i < 5 { // Log first 5 activities for debugging
			log.Printf("YouTrack Debug: Converting activity %d: ID=%s, Category=%s", i+1, ytActivity.ID, ytActivity.Category.ID)
		}
		activity := y.convertActivity(ytActivity)
		if activity != nil {
			activities = append(activities, *activity)
		} else if y.isDebugMode() {
			log.Printf("YouTrack Debug: Skipped activity %s (category: %s) - conversion returned nil", ytActivity.ID, ytActivity.Category.ID)
		}
	}

	return activities
}

// extractFieldValue extracts a user-friendly string representation from YouTrack field values
// Handles various data types returned by the YouTrack API (strings, objects, arrays)
func (y *YouTrackConnector) extractFieldValue(value interface{}) string {
	if value == nil {
		return ""
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Extracting field value from: %+v (type: %T)", value, value)
	}

	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Processing object with keys: %v", getMapKeys(v))
		}

		// Try common YouTrack field patterns in priority order

		// For state/enum fields - try name first
		if name, exists := v["name"]; exists {
			if nameStr, ok := name.(string); ok {
				if y.isDebugMode() {
					log.Printf("YouTrack Debug: Extracted name: %s", nameStr)
				}
				return nameStr
			}
		}

		// For user fields - try fullName, then name, then login
		if fullName, exists := v["fullName"]; exists {
			if fullNameStr, ok := fullName.(string); ok {
				if y.isDebugMode() {
					log.Printf("YouTrack Debug: Extracted fullName: %s", fullNameStr)
				}
				return fullNameStr
			}
		}

		if login, exists := v["login"]; exists {
			if loginStr, ok := login.(string); ok {
				if y.isDebugMode() {
					log.Printf("YouTrack Debug: Extracted login: %s", loginStr)
				}
				return loginStr
			}
		}

		// For other fields - try presentation
		if presentation, exists := v["presentation"]; exists {
			if presStr, ok := presentation.(string); ok {
				if y.isDebugMode() {
					log.Printf("YouTrack Debug: Extracted presentation: %s", presStr)
				}
				return presStr
			}
		}

		// Try other common field names
		fieldNames := []string{"displayName", "text", "value", "summary", "title"}
		for _, fieldName := range fieldNames {
			if fieldValue, exists := v[fieldName]; exists {
				if fieldStr, ok := fieldValue.(string); ok && fieldStr != "" {
					if y.isDebugMode() {
						log.Printf("YouTrack Debug: Extracted %s: %s", fieldName, fieldStr)
					}
					return fieldStr
				}
			}
		}

		// Skip $type field and other metadata fields when looking for fallback
		for key, val := range v {
			if key == "$type" || key == "id" || key == "$id" {
				continue
			}
			if str, ok := val.(string); ok && str != "" {
				if y.isDebugMode() {
					log.Printf("YouTrack Debug: Using fallback field %s: %s", key, str)
				}
				return str
			}
		}

		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Could not extract value from object: %+v", v)
		}
		return ""
	case []interface{}:
		var values []string
		for _, item := range v {
			if extracted := y.extractFieldValue(item); extracted != "" {
				values = append(values, extracted)
			}
		}
		if len(values) > 0 {
			result := strings.Join(values, ", ")
			if y.isDebugMode() {
				log.Printf("YouTrack Debug: Extracted array values: %s", result)
			}
			return result
		}
		return ""
	default:
		// For any other type, try to convert to string
		result := fmt.Sprintf("%v", v)
		if y.isDebugMode() {
			log.Printf("YouTrack Debug: Converting to string: %s", result)
		}
		return result
	}
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// convertActivity converts a single YouTrack activity to timeline activity
func (y *YouTrackConnector) convertActivity(ytActivity youTrackActivity) *timeline.Activity {
	// Convert timestamp from milliseconds to time.Time
	timestamp := time.Unix(ytActivity.Timestamp/1000, (ytActivity.Timestamp%1000)*1000000)

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Converting activity %s (category: %s, timestamp: %s)",
			ytActivity.ID, ytActivity.Category.ID, timestamp.Format("2006-01-02 15:04:05"))
	}

	// Extract issue information from target
	var issueID, issueKey, issueSummary, projectName string
	if target, ok := ytActivity.Target.(map[string]interface{}); ok {
		// For comment activities, the target is a comment and issue info is in target.issue
		if issue, exists := target["issue"].(map[string]interface{}); exists {
			if y.isDebugMode() {
				log.Printf("YouTrack Debug: Activity %s - extracting issue info from target.issue (comment activity)", ytActivity.ID)
			}
			if id, exists := issue["id"].(string); exists {
				issueID = id
			}
			if idReadable, exists := issue["idReadable"].(string); exists {
				issueKey = idReadable
			}
			if summary, exists := issue["summary"].(string); exists {
				issueSummary = summary
			}
			if project, exists := issue["project"].(map[string]interface{}); exists {
				if name, exists := project["name"].(string); exists {
					projectName = name
				}
			}
		} else {
			// For non-comment activities, issue info is directly in target
			if y.isDebugMode() {
				log.Printf("YouTrack Debug: Activity %s - extracting issue info from target directly (non-comment activity)", ytActivity.ID)
			}
			if id, exists := target["id"].(string); exists {
				issueID = id
			}
			if idReadable, exists := target["idReadable"].(string); exists {
				issueKey = idReadable
			}
			if summary, exists := target["summary"].(string); exists {
				issueSummary = summary
			}
			if project, exists := target["project"].(map[string]interface{}); exists {
				if name, exists := project["name"].(string); exists {
					projectName = name
				}
			}
		}
	}

	// Use issue key if available, fallback to internal ID
	displayIssueID := issueKey
	if displayIssueID == "" {
		displayIssueID = issueID
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Activity %s - extracted issue info: ID='%s', Key='%s', Display='%s'",
			ytActivity.ID, issueID, issueKey, displayIssueID)
	}

	// Generate activity title and description based on category
	var title, description string

	categoryID := ytActivity.Category.ID
	categoryName := ytActivity.Category.Name

	switch categoryID {
	case "CustomFieldCategory":
		fieldName := "field"
		if ytActivity.Field != nil {
			fieldName = ytActivity.Field.Name
		}

		// Extract new field value from 'added' field
		newValue := y.extractFieldValue(ytActivity.Added)
		oldValue := y.extractFieldValue(ytActivity.Removed)

		// Build title with new value if available
		if newValue != "" {
			title = fmt.Sprintf("Updated %s to %s", fieldName, newValue)
			if displayIssueID != "" {
				title = fmt.Sprintf("Updated %s to %s in %s", fieldName, newValue, displayIssueID)
			}
		} else {
			title = fmt.Sprintf("Updated %s", fieldName)
			if displayIssueID != "" {
				title = fmt.Sprintf("Updated %s in %s", fieldName, displayIssueID)
			}
		}

		// Build description with old and new values if available
		if newValue != "" && oldValue != "" {
			description = fmt.Sprintf("Changed %s from %s to %s", fieldName, oldValue, newValue)
		} else if newValue != "" {
			description = fmt.Sprintf("Set %s to %s", fieldName, newValue)
		} else if oldValue != "" {
			description = fmt.Sprintf("Cleared %s (was %s)", fieldName, oldValue)
		} else {
			description = fmt.Sprintf("Modified %s", fieldName)
		}

	case "CommentsCategory":
		title = "Added comment"
		if displayIssueID != "" {
			title = fmt.Sprintf("Commented on %s", displayIssueID)
		}
		description = "Added a comment"

	case "WorkItemCategory":
		title = "Logged work"
		if displayIssueID != "" {
			title = fmt.Sprintf("Logged work on %s", displayIssueID)
		}
		description = "Added work item"

	case "LinkCategory":
		title = "Updated links"
		if displayIssueID != "" {
			title = fmt.Sprintf("Updated links for %s", displayIssueID)
		}
		description = "Modified issue links"

	default:
		title = categoryName
		if displayIssueID != "" {
			title = fmt.Sprintf("%s - %s", categoryName, displayIssueID)
		}
		description = fmt.Sprintf("YouTrack activity: %s", categoryName)
	}

	// Build metadata
	metadata := map[string]string{
		"category": categoryID,
		"author":   ytActivity.Author.Login,
	}

	if issueID != "" {
		metadata["issue_id"] = issueID
	}
	if issueKey != "" {
		metadata["issue_key"] = issueKey
	}
	if issueSummary != "" {
		metadata["issue_summary"] = issueSummary
	}
	if projectName != "" {
		metadata["project"] = projectName
	}

	// Add field change metadata for CustomFieldCategory
	if categoryID == "CustomFieldCategory" {
		if ytActivity.Field != nil {
			metadata["field_name"] = ytActivity.Field.Name
		}
		if newValue := y.extractFieldValue(ytActivity.Added); newValue != "" {
			metadata["field_new_value"] = newValue
		}
		if oldValue := y.extractFieldValue(ytActivity.Removed); oldValue != "" {
			metadata["field_old_value"] = oldValue
		}
	}

	// Build URL to the issue using the readable issue key if available
	var activityURL string
	if displayIssueID != "" {
		baseURL := y.GetConfigString("base_url")
		activityURL = baseURL + "issue/" + displayIssueID
	}

	if y.isDebugMode() {
		log.Printf("YouTrack Debug: Created activity: %s -> %s", ytActivity.ID, title)
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("youtrack-%s", ytActivity.ID),
		Type:        timeline.ActivityTypeYouTrack,
		Title:       title,
		Description: description,
		Timestamp:   timestamp,
		Source:      "youtrack",
		URL:         activityURL,
		Metadata:    metadata,
	}
}
