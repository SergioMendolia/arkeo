package connectors

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/autotime/autotime/internal/timeline"
)

// GitLabConnector implements the Connector interface for GitLab
type GitLabConnector struct {
	*BaseConnector
	httpClient *http.Client
}

// AtomFeed represents the root element of an Atom feed
type AtomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []AtomEntry `xml:"entry"`
}

// AtomEntry represents an individual entry in an Atom feed
type AtomEntry struct {
	ID        string       `xml:"id"`
	Title     string       `xml:"title"`
	Published string       `xml:"published"`
	Updated   string       `xml:"updated"`
	Content   AtomContent  `xml:"content"`
	Link      AtomLink     `xml:"link"`
	Author    AtomAuthor   `xml:"author"`
	Category  AtomCategory `xml:"category"`
	Summary   string       `xml:"summary"`
}

// AtomContent represents the content element of an Atom entry
type AtomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// AtomLink represents a link element in an Atom entry
type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

// AtomAuthor represents the author element of an Atom entry
type AtomAuthor struct {
	Name string `xml:"name"`
}

// AtomCategory represents a category element in an Atom entry
type AtomCategory struct {
	Term string `xml:"term,attr"`
}

// NewGitLabConnector creates a new GitLab connector
func NewGitLabConnector() *GitLabConnector {
	return &GitLabConnector{
		BaseConnector: NewBaseConnector(
			"gitlab",
			"Fetches user activities from GitLab using Atom feeds",
		),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// isDebugMode checks if debug logging is enabled
func (g *GitLabConnector) isDebugMode() bool {
	// Check if AUTOTIME_DEBUG environment variable is set
	if os.Getenv("AUTOTIME_DEBUG") != "" {
		return true
	}
	// Check if LOG_LEVEL is set to debug
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	return logLevel == "debug"
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
			Key:         "feed_token",
			Type:        "secret",
			Required:    true,
			Description: "GitLab user feed token (found in Profile > Edit Profile > Access tokens)",
		},
	}
}

// ValidateConfig validates the GitLab configuration
func (g *GitLabConnector) ValidateConfig(config map[string]interface{}) error {
	username, ok := config["username"].(string)
	if !ok || username == "" {
		return fmt.Errorf("gitlab username is required")
	}

	feedToken, ok := config["feed_token"].(string)
	if !ok || feedToken == "" {
		return fmt.Errorf("gitlab feed token is required")
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

	username := g.GetConfigString("username")
	feedToken := g.GetConfigString("feed_token")

	if username == "" {
		return fmt.Errorf("no username configured")
	}
	if feedToken == "" {
		return fmt.Errorf("no feed token configured")
	}

	// Construct feed URL
	feedURL := fmt.Sprintf("%s/%s.atom?feed_token=%s", gitlabURL, username, feedToken)

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Testing connection to %s/%s.atom", gitlabURL, username)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/atom+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "AutoTime/1.0")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch GitLab feed: %w", err)
	}
	defer resp.Body.Close()

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Response status: %d %s", resp.StatusCode, resp.Status)
		log.Printf("GitLab Debug: Response headers: %v", resp.Header)
	}

	if resp.StatusCode != http.StatusOK {
		// Read response body for better error reporting
		body, _ := io.ReadAll(resp.Body)
		if g.isDebugMode() {
			log.Printf("GitLab Debug: Error response body: %s", string(body))
		}
		return fmt.Errorf("GitLab feed returned status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Response body length: %d bytes", len(body))
		if len(body) > 0 {
			// Show first 500 characters of response
			preview := string(body)
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			log.Printf("GitLab Debug: Response preview: %s", preview)
		}
	}

	// Try to parse XML
	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		if g.isDebugMode() {
			log.Printf("GitLab Debug: XML parsing failed: %v", err)
		}
		return fmt.Errorf("failed to parse GitLab feed: %w", err)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Successfully parsed feed with %d entries", len(feed.Entries))
	}

	return nil
}

// GetActivities retrieves GitLab activities for the specified date
func (g *GitLabConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	gitlabURL := g.GetConfigString("gitlab_url")
	if gitlabURL == "" {
		gitlabURL = "https://gitlab.com"
	}

	username := g.GetConfigString("username")
	feedToken := g.GetConfigString("feed_token")

	// Construct feed URL
	feedURL := fmt.Sprintf("%s/%s.atom?feed_token=%s", gitlabURL, username, feedToken)

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Fetching activities from %s/%s.atom for date %s", gitlabURL, username, date.Format("2006-01-02"))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/atom+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "AutoTime/1.0")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitLab feed: %w", err)
	}
	defer resp.Body.Close()

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Response status: %d %s", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if g.isDebugMode() {
			log.Printf("GitLab Debug: Error response body: %s", string(body))
		}
		return nil, fmt.Errorf("GitLab feed returned status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Response body length: %d bytes", len(body))
	}

	// Parse the Atom feed
	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		if g.isDebugMode() {
			log.Printf("GitLab Debug: XML parsing failed: %v", err)
			// Save raw response to file for inspection
			if file, ferr := os.CreateTemp("", "gitlab-response-*.xml"); ferr == nil {
				file.Write(body)
				file.Close()
				log.Printf("GitLab Debug: Raw response saved to %s", file.Name())
			}
		}
		return nil, fmt.Errorf("failed to parse GitLab feed: %w", err)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Successfully parsed feed with %d total entries", len(feed.Entries))
		for i, entry := range feed.Entries {
			log.Printf("GitLab Debug: Entry %d - Title: %s, Published: '%s', Updated: '%s'", i+1, entry.Title, entry.Published, entry.Updated)
		}
	}

	activities, err := g.convertEntriesToActivities(feed.Entries, date)
	if err != nil {
		return nil, err
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Converted %d entries to %d activities for date %s", len(feed.Entries), len(activities), date.Format("2006-01-02"))
	}

	return activities, nil
}

// convertEntriesToActivities converts Atom entries to timeline activities
func (g *GitLabConnector) convertEntriesToActivities(entries []AtomEntry, targetDate time.Time) ([]timeline.Activity, error) {
	var activities []timeline.Activity

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Converting %d entries to activities for target date %s", len(entries), targetDate.Format("2006-01-02"))
	}

	for i, entry := range entries {
		var publishedTime time.Time
		var err error

		// Try to parse the published date first
		if entry.Published != "" {
			publishedTime, err = time.Parse(time.RFC3339, entry.Published)
			if err != nil {
				// Try alternative formats if RFC3339 fails
				publishedTime, err = time.Parse("2006-01-02T15:04:05Z", entry.Published)
			}
		}

		// If published date is empty or failed to parse, try updated date
		if err != nil || entry.Published == "" {
			if entry.Updated != "" {
				publishedTime, err = time.Parse(time.RFC3339, entry.Updated)
				if err != nil {
					publishedTime, err = time.Parse("2006-01-02T15:04:05Z", entry.Updated)
				}
				if g.isDebugMode() && entry.Published == "" {
					log.Printf("GitLab Debug: Entry %d - Using updated date as fallback: %s", i+1, entry.Updated)
				}
			}
		}

		// If both dates failed to parse, skip this entry
		if err != nil {
			if g.isDebugMode() {
				log.Printf("GitLab Debug: Entry %d - Failed to parse both published '%s' and updated '%s' dates: %v",
					i+1, entry.Published, entry.Updated, err)
			}
			continue // Skip entries with unparseable dates
		}

		if g.isDebugMode() {
			dateSource := "published"
			if entry.Published == "" || (err != nil && entry.Updated != "") {
				dateSource = "updated"
			}
			log.Printf("GitLab Debug: Entry %d - Using %s date: %s, Target date: %s, Same day: %v (published: '%s', updated: '%s')",
				i+1, dateSource, publishedTime.Format("2006-01-02"), targetDate.Format("2006-01-02"),
				isSameDay(publishedTime, targetDate), entry.Published, entry.Updated)
		}

		// Check if the activity is on the target date
		if !isSameDay(publishedTime, targetDate) {
			if g.isDebugMode() {
				log.Printf("GitLab Debug: Entry %d - Skipping due to date mismatch", i+1)
			}
			continue
		}

		// Determine activity type based on content
		activityType := g.determineActivityType(entry)

		// Extract project/repository name from the link or content
		projectName := g.extractProjectName(entry)

		// Create tags
		tags := []string{"gitlab", activityType}
		if projectName != "" {
			tags = append(tags, projectName)
		}
		if entry.Category.Term != "" {
			tags = append(tags, entry.Category.Term)
		}

		// Create activity
		activity := timeline.Activity{
			ID:          fmt.Sprintf("gitlab-%s", g.generateActivityID(entry)),
			Type:        g.mapToTimelineActivityType(activityType),
			Title:       entry.Title,
			Description: g.extractDescription(entry),
			Timestamp:   publishedTime,
			Source:      "gitlab",
			URL:         entry.Link.Href,
			Tags:        tags,
			Metadata: map[string]string{
				"gitlab_id":     entry.ID,
				"activity_type": activityType,
				"project":       projectName,
				"author":        entry.Author.Name,
			},
		}

		if g.isDebugMode() {
			log.Printf("GitLab Debug: Entry %d - Created activity: %s", i+1, activity.Title)
		}

		activities = append(activities, activity)
	}

	if g.isDebugMode() {
		log.Printf("GitLab Debug: Final result: %d activities created", len(activities))
	}

	return activities, nil
}

// determineActivityType determines the type of GitLab activity from the entry
func (g *GitLabConnector) determineActivityType(entry AtomEntry) string {
	title := strings.ToLower(entry.Title)
	content := strings.ToLower(entry.Content.Value + entry.Summary)

	// Check for different activity types based on title and content
	switch {
	case strings.Contains(title, "pushed to") || strings.Contains(content, "commit"):
		return "commit"
	case strings.Contains(title, "opened merge request") || strings.Contains(title, "created merge request"):
		return "merge_request_opened"
	case strings.Contains(title, "merged merge request") || strings.Contains(title, "accepted merge request"):
		return "merge_request_merged"
	case strings.Contains(title, "closed merge request"):
		return "merge_request_closed"
	case strings.Contains(title, "opened issue") || strings.Contains(title, "created issue"):
		return "issue_opened"
	case strings.Contains(title, "closed issue"):
		return "issue_closed"
	case strings.Contains(title, "commented on"):
		return "comment"
	case strings.Contains(title, "created project") || strings.Contains(title, "created repository"):
		return "project_created"
	case strings.Contains(title, "created wiki page") || strings.Contains(title, "updated wiki page"):
		return "wiki"
	case strings.Contains(title, "created milestone") || strings.Contains(title, "closed milestone"):
		return "milestone"
	default:
		return "activity"
	}
}

// mapToTimelineActivityType maps GitLab activity types to timeline activity types
func (g *GitLabConnector) mapToTimelineActivityType(activityType string) timeline.ActivityType {
	switch activityType {
	case "commit":
		return timeline.ActivityTypeGitCommit
	case "merge_request_opened", "merge_request_merged", "merge_request_closed":
		return timeline.ActivityTypeJira // Using as generic project activity type
	case "issue_opened", "issue_closed":
		return timeline.ActivityTypeJira
	case "comment":
		return timeline.ActivityTypeCustom
	case "project_created":
		return timeline.ActivityTypeCustom
	case "wiki":
		return timeline.ActivityTypeCustom
	case "milestone":
		return timeline.ActivityTypeCustom
	default:
		return timeline.ActivityTypeCustom
	}
}

// extractProjectName extracts the project name from the entry
func (g *GitLabConnector) extractProjectName(entry AtomEntry) string {
	// Try to extract from the link URL
	if entry.Link.Href != "" {
		parts := strings.Split(entry.Link.Href, "/")
		for i, part := range parts {
			if part == "projects" && i+1 < len(parts) {
				// Project name might be URL encoded
				if projectName, err := url.QueryUnescape(parts[i+1]); err == nil {
					return projectName
				}
				return parts[i+1]
			}
		}
	}

	// Try to extract from title (common format: "username action in project")
	title := entry.Title
	if strings.Contains(title, " in ") {
		parts := strings.Split(title, " in ")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}

	// Try to extract from content
	content := entry.Content.Value
	if strings.Contains(content, "project ") {
		// This is a basic extraction - might need refinement based on actual feed content
		words := strings.Fields(content)
		for i, word := range words {
			if word == "project" && i+1 < len(words) {
				return words[i+1]
			}
		}
	}

	return ""
}

// extractDescription creates a description from the entry content
func (g *GitLabConnector) extractDescription(entry AtomEntry) string {
	if entry.Summary != "" {
		return entry.Summary
	}

	if entry.Content.Value != "" {
		// Clean up HTML/markdown content for display
		content := entry.Content.Value
		content = strings.ReplaceAll(content, "\n", " ")
		content = strings.TrimSpace(content)

		// Truncate if too long
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		return content
	}

	return "GitLab activity"
}

// generateActivityID generates a unique activity ID from the entry
func (g *GitLabConnector) generateActivityID(entry AtomEntry) string {
	// Use GitLab's internal ID if available, otherwise generate from title + timestamp
	if entry.ID != "" {
		// Extract the numeric part from GitLab's ID format
		parts := strings.Split(entry.ID, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback: generate from title and published time
	if entry.Published != "" {
		if publishedTime, err := time.Parse(time.RFC3339, entry.Published); err == nil {
			return fmt.Sprintf("%d-%s", publishedTime.Unix(), strings.ReplaceAll(entry.Title, " ", "-"))
		}
	}

	return fmt.Sprintf("gitlab-%d", time.Now().Unix())
}
