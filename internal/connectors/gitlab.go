package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// GitLabConnector implements the Connector interface for GitLab
type GitLabConnector struct {
	*BaseConnector
}

// GitLabEvent represents an event from the GitLab events API
type GitLabEvent struct {
	ID          int                 `json:"id"`
	Title       *string             `json:"title"`
	ProjectID   int                 `json:"project_id"`
	ActionName  string              `json:"action_name"`
	TargetID    *int                `json:"target_id"`
	TargetIID   *int                `json:"target_iid"`
	TargetType  *string             `json:"target_type"`
	AuthorID    int                 `json:"author_id"`
	TargetTitle *string             `json:"target_title"`
	CreatedAt   string              `json:"created_at"`
	Author      GitLabEventAuthor   `json:"author"`
	PushData    *GitLabPushData     `json:"push_data,omitempty"`
	Project     *GitLabEventProject `json:"project,omitempty"`
	Note        *GitLabEventNote    `json:"note,omitempty"`
}

// GitLabEventAuthor represents the author of an event
type GitLabEventAuthor struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// GitLabPushData represents push data in an event
type GitLabPushData struct {
	CommitCount int    `json:"commit_count"`
	Action      string `json:"action"`
	RefType     string `json:"ref_type"`
	CommitFrom  string `json:"commit_from"`
	CommitTo    string `json:"commit_to"`
	Ref         string `json:"ref"`
	CommitTitle string `json:"commit_title"`
}

// GitLabEventProject represents project info in an event
type GitLabEventProject struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
}

// GitLabEventNote represents note/comment data in an event
type GitLabEventNote struct {
	ID         int    `json:"id"`
	Body       string `json:"body"`
	AuthorID   int    `json:"author_id"`
	CreatedAt  string `json:"created_at"`
	System     bool   `json:"system"`
	NoteableID int    `json:"noteable_id"`
}

// NewGitLabConnector creates a new GitLab connector
func NewGitLabConnector() *GitLabConnector {
	return &GitLabConnector{
		BaseConnector: NewBaseConnector(
			"gitlab",
			"Fetches user activities from GitLab Events API (all branches, merge requests, issues)",
		),
	}
}

// GetHTTPClient is now used directly from BaseConnector

// stripHTMLTags removes HTML tags from a string
func (g *GitLabConnector) stripHTMLTags(input string) string {
	// Regular expression to match HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	// Replace HTML tags with a space to prevent words from being concatenated
	cleaned := htmlTagRegex.ReplaceAllString(input, " ")
	// Clean up extra whitespace
	cleaned = strings.TrimSpace(cleaned)
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	cleaned = spaceRegex.ReplaceAllString(cleaned, " ")
	return cleaned
}

// isDebugMode checks if debug logging is enabled
func (g *GitLabConnector) isDebugMode() bool {
	return g.BaseConnector.IsDebugMode()
}

// GetRequiredConfig returns the required configuration for GitLab
func (g *GitLabConnector) GetRequiredConfig() []ConfigField {
	return []ConfigField{
		{
			Key:         "gitlab_url",
			Type:        "string",
			Required:    false,
			Description: "GitLab instance URL (e.g., https://gitlab.com)",
			Default:     "https://gitlab.com",
		},
		{
			Key:         "username",
			Type:        "string",
			Required:    true,
			Description: "GitLab username",
		},
		{
			Key:         "access_token",
			Type:        "secret",
			Required:    true,
			Description: "GitLab personal access token (create at Profile > Access Tokens with 'read_api' scope)",
		},
	}
}

// Configure sets the connector configuration with GitLab-specific validation
func (g *GitLabConnector) Configure(config map[string]interface{}) error {
	if err := g.ValidateConfig(config); err != nil {
		return err
	}
	g.BaseConnector.config = config
	return nil
}

// ValidateConfig validates the GitLab configuration
func (g *GitLabConnector) ValidateConfig(config map[string]interface{}) error {
	username, ok := config["username"].(string)
	if !ok || username == "" {
		return fmt.Errorf("gitlab username is required")
	}

	accessToken, ok := config["access_token"].(string)
	if !ok || accessToken == "" {
		return fmt.Errorf("gitlab access token is required")
	}

	// Validate GitLab URL if provided
	if gitlabURL, ok := config["gitlab_url"].(string); ok && gitlabURL != "" {
		if _, err := url.Parse(gitlabURL); err != nil {
			return fmt.Errorf("invalid gitlab_url: %w", err)
		}
	}

	return nil
}

// TestConnection tests the GitLab connection
func (g *GitLabConnector) TestConnection(ctx context.Context) error {
	gitlabURL := g.GetConfigString("gitlab_url")
	if gitlabURL == "" {
		gitlabURL = "https://gitlab.com"
	}

	accessToken := g.GetConfigString("access_token")
	if accessToken == "" {
		return fmt.Errorf("no access token configured")
	}

	// Test API connection by fetching user events
	apiURL := fmt.Sprintf("%s/api/v4/events?per_page=1", strings.TrimSuffix(gitlabURL, "/"))

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Testing connection to %s", apiURL)
	}

	// Create request
	req, err := g.CreateRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := g.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch GitLab events: %w", err)
	}
	defer resp.Body.Close()

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Response status: %d %s", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed - please check your access token")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if g.isDebugMode() {
			log.Printf("GitLab Debug: Error response body: %s", string(body))
		}
		return fmt.Errorf("GitLab API returned status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read and validate response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Try to parse as JSON to validate API response
	var events []GitLabEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return fmt.Errorf("failed to parse GitLab API response: %w", err)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Successfully connected, found %d recent events", len(events))
	}

	return nil
}

// GetActivities retrieves GitLab activities for the specified date
// GetActivities gets activities from GitLab for the given date
func (g *GitLabConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	gitlabURL := g.GetConfigString("gitlab_url")
	if gitlabURL == "" {
		gitlabURL = "https://gitlab.com"
	}

	username := g.GetConfigString("username")
	accessToken := g.GetConfigString("access_token")

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Fetching events for user %s on date %s", username, date.Format("2006-01-02"))
	}

	// Get events from GitLab API
	events, err := g.getEvents(ctx, gitlabURL, accessToken, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Found %d events for date %s", len(events), date.Format("2006-01-02"))
	}

	var allActivities []timeline.Activity

	// Convert events to activities
	for _, event := range events {
		activity := g.convertEventToActivity(event)
		if activity != nil {
			allActivities = append(allActivities, *activity)
		}
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Total activities found: %d", len(allActivities))
	}

	return allActivities, nil
}

// getEvents fetches user events from GitLab for the specified date with pagination
func (g *GitLabConnector) getEvents(ctx context.Context, gitlabURL, accessToken string, date time.Time) ([]GitLabEvent, error) {
	var allDayEvents []GitLabEvent
	page := 1
	perPage := 100
	maxPages := 10 // Prevent infinite loops
	targetDate := date.Truncate(24 * time.Hour)
	nextDay := targetDate.Add(24 * time.Hour)

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Fetching events for date %s", date.Format("2006-01-02"))
	}

	for page <= maxPages {
		apiURL := fmt.Sprintf("%s/api/v4/events?per_page=%d&page=%d",
			strings.TrimSuffix(gitlabURL, "/"), perPage, page)

		if g.isDebugMode() {
			log.Printf("GitLab Debug: Fetching page %d from %s", page, apiURL)
		}

		req, err := g.CreateRequest(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request for page %d: %w", page, err)
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")

		resp, err := g.GetHTTPClient().Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch events page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("GitLab API returned status %d for page %d: %s", resp.StatusCode, page, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response for page %d: %w", page, err)
		}

		var pageEvents []GitLabEvent
		if err := json.Unmarshal(body, &pageEvents); err != nil {
			return nil, fmt.Errorf("failed to parse events response for page %d: %w", page, err)
		}

		if len(pageEvents) == 0 {
			if g.isDebugMode() {
				log.Printf("GitLab Debug: No more events found on page %d", page)
			}
			break
		}

		// Filter and check events by date
		tooOldEvents := false

		for _, event := range pageEvents {
			eventTime, err := time.Parse(time.RFC3339, event.CreatedAt)
			if err != nil {
				if g.isDebugMode() {
					log.Printf("GitLab Debug: Failed to parse event time %s: %v", event.CreatedAt, err)
				}
				continue
			}

			// If event is from target date, include it
			if eventTime.After(targetDate) && eventTime.Before(nextDay) {
				allDayEvents = append(allDayEvents, event)
			} else if eventTime.Before(targetDate) {
				// Events are typically in reverse chronological order
				// If we find events older than our target date, we can stop
				tooOldEvents = true
				break
			}
		}

		if g.isDebugMode() {
			log.Printf("GitLab Debug: Page %d: found %d events, %d from target date",
				page, len(pageEvents), len(allDayEvents))
		}

		// Stop pagination if we've gone past our target date
		if tooOldEvents {
			if g.isDebugMode() {
				log.Printf("GitLab Debug: Found events older than target date, stopping pagination")
			}
			break
		}

		// If this page didn't have the full number of events, we've reached the end
		if len(pageEvents) < perPage {
			if g.isDebugMode() {
				log.Printf("GitLab Debug: Reached last page (page %d had %d events)", page, len(pageEvents))
			}
			break
		}

		page++
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Total events found for target date: %d", len(allDayEvents))
	}

	return allDayEvents, nil
}

// convertEventToActivity converts a GitLab event to a timeline activity
func (g *GitLabConnector) convertEventToActivity(event GitLabEvent) *timeline.Activity {
	// Parse event date
	eventTime, err := time.Parse(time.RFC3339, event.CreatedAt)
	if err != nil {
		if g.isDebugMode() {
			log.Printf("GitLab Debug: Failed to parse event time %s: %v", event.CreatedAt, err)
		}
		eventTime = time.Now() // Fallback
	}

	// Handle different event types
	switch event.ActionName {
	case "pushed to":
		return g.convertPushEventToActivity(event, eventTime)
	case "opened", "merged", "closed":
		if event.TargetType != nil && *event.TargetType == "MergeRequest" {
			return g.convertMergeRequestEventToActivity(event, eventTime)
		} else if event.TargetType != nil && *event.TargetType == "Issue" {
			return g.convertIssueEventToActivity(event, eventTime)
		}
	case "commented on":
		return g.convertCommentEventToActivity(event, eventTime)
	case "created":
		if event.TargetType != nil && *event.TargetType == "Project" {
			return g.convertProjectEventToActivity(event, eventTime)
		} else if event.TargetType != nil && (*event.TargetType == "Issue" || *event.TargetType == "MergeRequest") {
			return g.convertIssueEventToActivity(event, eventTime)
		}
	}

	// For unhandled event types, create a generic activity
	if g.isDebugMode() {
		log.Printf("GitLab Debug: Unhandled event type: %s with target type: %v", event.ActionName, event.TargetType)
	}

	return g.convertGenericEventToActivity(event, eventTime)
}

// convertPushEventToActivity converts a GitLab push event to a timeline activity
func (g *GitLabConnector) convertPushEventToActivity(event GitLabEvent, eventTime time.Time) *timeline.Activity {
	if event.PushData == nil {
		return nil
	}

	// Use commit title from push data or generate a default
	title := strings.TrimSpace(event.PushData.CommitTitle)
	if title == "" {
		if event.PushData.CommitCount == 1 {
			title = fmt.Sprintf("Pushed 1 commit to %s", event.PushData.Ref)
		} else {
			title = fmt.Sprintf("Pushed %d commits to %s", event.PushData.CommitCount, event.PushData.Ref)
		}
	}

	// Create description
	var description string
	if event.Project != nil {
		description = fmt.Sprintf("Pushed to %s branch in %s", event.PushData.Ref, event.Project.PathWithNamespace)
	} else {
		description = fmt.Sprintf("Pushed to %s branch", event.PushData.Ref)
	}

	if event.PushData.CommitCount > 1 {
		description += fmt.Sprintf(" (%d commits)", event.PushData.CommitCount)
	}

	// Generate URL - try to link to the commits
	var activityURL string
	if event.Project != nil && event.PushData.CommitTo != "" {
		if event.PushData.CommitCount == 1 {
			// Single commit - link to commit
			activityURL = fmt.Sprintf("%s/-/commit/%s", event.Project.WebURL, event.PushData.CommitTo)
		} else {
			// Multiple commits - link to compare view
			activityURL = fmt.Sprintf("%s/-/compare/%s...%s", event.Project.WebURL, event.PushData.CommitFrom, event.PushData.CommitTo)
		}
	} else if event.Project != nil {
		// Fallback to project URL
		activityURL = event.Project.WebURL
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("gitlab-push-%d", event.ID),
		Type:        timeline.ActivityTypeGitCommit,
		Title:       g.stripHTMLTags(title),
		Description: description,
		Timestamp:   eventTime,
		Source:      "gitlab",
		URL:         activityURL,
		Metadata: map[string]string{
			"event_id":     fmt.Sprintf("%d", event.ID),
			"action_name":  event.ActionName,
			"commit_count": fmt.Sprintf("%d", event.PushData.CommitCount),
			"ref":          event.PushData.Ref,
			"ref_type":     event.PushData.RefType,
			"commit_from":  event.PushData.CommitFrom,
			"commit_to":    event.PushData.CommitTo,
			"author":       event.Author.Username,
		},
	}
}

// convertMergeRequestEventToActivity converts a GitLab merge request event to a timeline activity
func (g *GitLabConnector) convertMergeRequestEventToActivity(event GitLabEvent, eventTime time.Time) *timeline.Activity {
	title := event.ActionName + " merge request"
	if event.TargetTitle != nil {
		title = fmt.Sprintf("%s merge request: %s", strings.Title(event.ActionName), g.stripHTMLTags(*event.TargetTitle))
	}

	var description string
	if event.Project != nil {
		description = fmt.Sprintf("%s merge request in %s", strings.Title(event.ActionName), event.Project.PathWithNamespace)
	} else {
		description = fmt.Sprintf("%s merge request", strings.Title(event.ActionName))
	}

	if event.TargetIID != nil {
		description += fmt.Sprintf(" (!%d)", *event.TargetIID)
	}

	// Generate URL
	var activityURL string
	if event.Project != nil && event.TargetIID != nil {
		activityURL = fmt.Sprintf("%s/-/merge_requests/%d", event.Project.WebURL, *event.TargetIID)
	} else if event.Project != nil {
		activityURL = event.Project.WebURL
	}

	metadata := map[string]string{
		"event_id":    fmt.Sprintf("%d", event.ID),
		"action_name": event.ActionName,
		"author":      event.Author.Username,
	}
	if event.TargetIID != nil {
		metadata["merge_request_iid"] = fmt.Sprintf("%d", *event.TargetIID)
	}
	if event.TargetID != nil {
		metadata["merge_request_id"] = fmt.Sprintf("%d", *event.TargetID)
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("gitlab-mr-%d", event.ID),
		Type:        timeline.ActivityTypeJira,
		Title:       title,
		Description: description,
		Timestamp:   eventTime,
		Source:      "gitlab",
		URL:         activityURL,
		Metadata:    metadata,
	}
}

// convertIssueEventToActivity converts a GitLab issue event to a timeline activity
func (g *GitLabConnector) convertIssueEventToActivity(event GitLabEvent, eventTime time.Time) *timeline.Activity {
	action := event.ActionName
	if action == "created" {
		action = "opened"
	}

	title := action + " issue"
	if event.TargetTitle != nil {
		title = fmt.Sprintf("%s issue: %s", strings.Title(action), g.stripHTMLTags(*event.TargetTitle))
	}

	var description string
	if event.Project != nil {
		description = fmt.Sprintf("%s issue in %s", strings.Title(action), event.Project.PathWithNamespace)
	} else {
		description = fmt.Sprintf("%s issue", strings.Title(action))
	}

	if event.TargetIID != nil {
		description += fmt.Sprintf(" (#%d)", *event.TargetIID)
	}

	// Generate URL
	var activityURL string
	if event.Project != nil && event.TargetIID != nil {
		activityURL = fmt.Sprintf("%s/-/issues/%d", event.Project.WebURL, *event.TargetIID)
	} else if event.Project != nil {
		activityURL = event.Project.WebURL
	}

	metadata := map[string]string{
		"event_id":    fmt.Sprintf("%d", event.ID),
		"action_name": event.ActionName,
		"author":      event.Author.Username,
	}
	if event.TargetIID != nil {
		metadata["issue_iid"] = fmt.Sprintf("%d", *event.TargetIID)
	}
	if event.TargetID != nil {
		metadata["issue_id"] = fmt.Sprintf("%d", *event.TargetID)
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("gitlab-issue-%d", event.ID),
		Type:        timeline.ActivityTypeJira,
		Title:       title,
		Description: description,
		Timestamp:   eventTime,
		Source:      "gitlab",
		URL:         activityURL,
		Metadata:    metadata,
	}
}

// convertCommentEventToActivity converts a GitLab comment event to a timeline activity
func (g *GitLabConnector) convertCommentEventToActivity(event GitLabEvent, eventTime time.Time) *timeline.Activity {
	var title string
	var targetType string

	if event.TargetType != nil {
		switch *event.TargetType {
		case "MergeRequest":
			targetType = "merge request"
		case "Issue":
			targetType = "issue"
		case "Commit":
			targetType = "commit"
		default:
			targetType = strings.ToLower(*event.TargetType)
		}
	} else {
		targetType = "item"
	}

	if event.TargetTitle != nil {
		title = fmt.Sprintf("Commented on %s: %s", targetType, g.stripHTMLTags(*event.TargetTitle))
	} else {
		title = fmt.Sprintf("Commented on %s", targetType)
	}

	var description string
	if event.Project != nil {
		description = fmt.Sprintf("Commented on %s in %s", targetType, event.Project.PathWithNamespace)
	} else {
		description = fmt.Sprintf("Commented on %s", targetType)
	}

	// Generate URL - this is more complex as we need to determine the right URL format
	var activityURL string
	if event.Project != nil && event.TargetIID != nil {
		switch targetType {
		case "merge request":
			activityURL = fmt.Sprintf("%s/-/merge_requests/%d", event.Project.WebURL, *event.TargetIID)
		case "issue":
			activityURL = fmt.Sprintf("%s/-/issues/%d", event.Project.WebURL, *event.TargetIID)
		default:
			activityURL = event.Project.WebURL
		}
	} else if event.Project != nil {
		activityURL = event.Project.WebURL
	}

	metadata := map[string]string{
		"event_id":    fmt.Sprintf("%d", event.ID),
		"action_name": event.ActionName,
		"author":      event.Author.Username,
		"target_type": targetType,
	}
	if event.TargetIID != nil {
		metadata["target_iid"] = fmt.Sprintf("%d", *event.TargetIID)
	}
	if event.TargetID != nil {
		metadata["target_id"] = fmt.Sprintf("%d", *event.TargetID)
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("gitlab-comment-%d", event.ID),
		Type:        timeline.ActivityTypeCustom,
		Title:       title,
		Description: description,
		Timestamp:   eventTime,
		Source:      "gitlab",
		URL:         activityURL,
		Metadata:    metadata,
	}
}

// convertProjectEventToActivity converts a GitLab project event to a timeline activity
func (g *GitLabConnector) convertProjectEventToActivity(event GitLabEvent, eventTime time.Time) *timeline.Activity {
	title := "Created project"
	if event.TargetTitle != nil {
		title = fmt.Sprintf("Created project: %s", g.stripHTMLTags(*event.TargetTitle))
	} else if event.Project != nil {
		title = fmt.Sprintf("Created project: %s", event.Project.Name)
	}

	var description string
	if event.Project != nil {
		description = fmt.Sprintf("Created new project %s", event.Project.PathWithNamespace)
	} else {
		description = "Created new project"
	}

	// Generate URL
	var activityURL string
	if event.Project != nil {
		activityURL = event.Project.WebURL
	}

	metadata := map[string]string{
		"event_id":    fmt.Sprintf("%d", event.ID),
		"action_name": event.ActionName,
		"author":      event.Author.Username,
	}
	if event.ProjectID != 0 {
		metadata["project_id"] = fmt.Sprintf("%d", event.ProjectID)
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("gitlab-project-%d", event.ID),
		Type:        timeline.ActivityTypeCustom,
		Title:       title,
		Description: description,
		Timestamp:   eventTime,
		Source:      "gitlab",
		URL:         activityURL,
		Metadata:    metadata,
	}
}

// convertGenericEventToActivity converts any unhandled GitLab event to a generic timeline activity
func (g *GitLabConnector) convertGenericEventToActivity(event GitLabEvent, eventTime time.Time) *timeline.Activity {
	title := event.ActionName
	if event.TargetTitle != nil {
		title = fmt.Sprintf("%s: %s", strings.Title(event.ActionName), g.stripHTMLTags(*event.TargetTitle))
	} else {
		title = strings.Title(event.ActionName)
	}

	var description string
	if event.Project != nil {
		description = fmt.Sprintf("%s in %s", strings.Title(event.ActionName), event.Project.PathWithNamespace)
	} else {
		description = strings.Title(event.ActionName)
	}

	if event.TargetType != nil {
		description += fmt.Sprintf(" (%s)", strings.ToLower(*event.TargetType))
	}

	// Generate URL
	var activityURL string
	if event.Project != nil {
		activityURL = event.Project.WebURL
	}

	metadata := map[string]string{
		"event_id":    fmt.Sprintf("%d", event.ID),
		"action_name": event.ActionName,
		"author":      event.Author.Username,
	}
	if event.TargetType != nil {
		metadata["target_type"] = *event.TargetType
	}
	if event.TargetIID != nil {
		metadata["target_iid"] = fmt.Sprintf("%d", *event.TargetIID)
	}
	if event.TargetID != nil {
		metadata["target_id"] = fmt.Sprintf("%d", *event.TargetID)
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("gitlab-generic-%d", event.ID),
		Type:        timeline.ActivityTypeCustom,
		Title:       title,
		Description: description,
		Timestamp:   eventTime,
		Source:      "gitlab",
		URL:         activityURL,
		Metadata:    metadata,
	}
}
