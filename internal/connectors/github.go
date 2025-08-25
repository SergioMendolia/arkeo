package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// GitHubConnector implements the Connector interface for GitHub
type GitHubConnector struct {
	*BaseConnector
}

// NewGitHubConnector creates a new GitHub connector
func NewGitHubConnector() *GitHubConnector {
	return &GitHubConnector{
		BaseConnector: NewBaseConnector(
			"github",
			"Fetches git commits and GitHub activities",
		),
	}
}

// getHTTPClient returns a pooled HTTP client for GitHub API requests
// GetHTTPClient is now used directly from BaseConnector

// GetRequiredConfig returns the required configuration for GitHub
func (g *GitHubConnector) GetRequiredConfig() []ConfigField {
	// Define GitHub-specific required fields
	requiredFields := []ConfigField{
		{
			Key:         "token",
			Type:        "secret",
			Required:    true,
			Description: "GitHub Personal Access Token",
		},
		{
			Key:         "username",
			Type:        "string",
			Required:    true,
			Description: "GitHub username",
		},
		{
			Key:         "include_private",
			Type:        "bool",
			Required:    false,
			Description: "Include private repositories",
			Default:     false,
		},
	}

	// Merge with common fields
	return MergeConfigFields(requiredFields)
}

// ValidateConfig validates the GitHub configuration
// Uses the common validation helper to check required fields
func (g *GitHubConnector) ValidateConfig(config map[string]interface{}) error {
	// Use the common validation helper that checks all required fields
	if err := ValidateConfigFields(config, g.GetRequiredConfig()); err != nil {
		return err
	}

	return nil
}

// TestConnection tests the GitHub connection
func (g *GitHubConnector) TestConnection(ctx context.Context) error {
	token := g.GetConfigString("token")
	if token == "" {
		return fmt.Errorf("no token configured")
	}

	// HTTP client timeout is already configured in BaseConnector

	req, err := g.CreateBearerRequest(ctx, "GET", "https://api.github.com/user", token)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.GetHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned error: %s, status: %d", string(body), resp.StatusCode)
	}

	return nil
}

// GetActivities retrieves GitHub activities for the specified date
func (g *GitHubConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	var activities []timeline.Activity

	// HTTP client timeout is already configured in BaseConnector

	// Enable debug logging if configured
	if g.IsDebugMode() {
		log.Printf("GitHub Debug: Fetching activities for date %s", date.Format("2006-01-02"))
	}

	// Get commits
	commits, err := g.getCommits(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}
	activities = append(activities, commits...)

	// Get issues and PRs
	issuesAndPRs, err := g.getIssuesAndPRs(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get issues and PRs: %w", err)
	}
	activities = append(activities, issuesAndPRs...)

	// Limit the number of returned activities if max_items is set
	maxItems := g.GetConfigInt(CommonConfigKeys.MaxItems)
	if maxItems > 0 && len(activities) > maxItems {
		if g.IsDebugMode() {
			log.Printf("GitHub Debug: Limiting activities from %d to %d", len(activities), maxItems)
		}
		activities = activities[:maxItems]
	}

	return activities, nil
}

// getCommits retrieves commits for the specified date
func (g *GitHubConnector) getCommits(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	username := g.GetConfigString("username")
	token := g.GetConfigString("token")

	// Format date for GitHub API (ISO 8601)
	since := date.Format("2006-01-02T15:04:05Z")
	until := date.Add(24 * time.Hour).Format("2006-01-02T15:04:05Z")

	url := fmt.Sprintf("https://api.github.com/search/commits?q=committer:%s+author-date:%s..%s&sort=author-date&order=desc",
		username, since, until)

	req, err := g.CreateBearerRequest(ctx, "GET", url, token)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github commits API returned status %d", resp.StatusCode)
	}

	var searchResult struct {
		Items []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
			Repository struct {
				Name     string `json:"name"`
				FullName string `json:"full_name"`
				HTMLURL  string `json:"html_url"`
			} `json:"repository"`
			HTMLURL string `json:"html_url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	var activities []timeline.Activity
	for _, item := range searchResult.Items {
		commitTime, err := time.Parse(time.RFC3339, item.Commit.Author.Date)
		if err != nil {
			continue
		}

		activity := timeline.Activity{
			ID:          fmt.Sprintf("github-commit-%s", item.SHA[:8]),
			Type:        timeline.ActivityTypeGitCommit,
			Title:       fmt.Sprintf("%s on %s", item.Commit.Message, item.Repository.FullName),
			Description: fmt.Sprintf("Commit to %s", item.Repository.FullName),
			Timestamp:   commitTime,
			Source:      "github",
			URL:         item.HTMLURL,
			Metadata: map[string]string{
				"repository": item.Repository.FullName,
				"sha":        item.SHA,
			},
		}

		activities = append(activities, activity)
	}

	return activities, nil
}

// getIssuesAndPRs retrieves issues and pull requests for the specified date
func (g *GitHubConnector) getIssuesAndPRs(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
	username := g.GetConfigString("username")
	token := g.GetConfigString("token")

	since := date.Format("2006-01-02")
	until := date.Add(24 * time.Hour).Format("2006-01-02")

	// Search for issues and PRs created or updated on this date
	queries := []string{
		fmt.Sprintf("author:%s+created:%s", username, since),
		fmt.Sprintf("assignee:%s+updated:%s..%s", username, since, until),
		fmt.Sprintf("mentions:%s+updated:%s..%s", username, since, until),
	}

	var allActivities []timeline.Activity

	for _, query := range queries {
		url := fmt.Sprintf("https://api.github.com/search/issues?q=%s&sort=updated&order=desc&per_page=100", query)

		req, err := g.CreateBearerRequest(ctx, "GET", url, token)
		if err != nil {
			continue
		}

		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := g.GetHTTPClient().Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var searchResult struct {
			Items []struct {
				ID          int       `json:"id"`
				Number      int       `json:"number"`
				Title       string    `json:"title"`
				State       string    `json:"state"`
				CreatedAt   string    `json:"created_at"`
				UpdatedAt   string    `json:"updated_at"`
				HTMLURL     string    `json:"html_url"`
				PullRequest *struct{} `json:"pull_request"`
				Repository  struct {
					Name     string `json:"name"`
					FullName string `json:"full_name"`
				} `json:"repository"`
				User struct {
					Login string `json:"login"`
				} `json:"user"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, item := range searchResult.Items {
			updatedTime, err := time.Parse(time.RFC3339, item.UpdatedAt)
			if err != nil {
				continue
			}

			// Check if the update was on the target date
			if !isSameDay(updatedTime, date) {
				continue
			}

			activityType := timeline.ActivityTypeJira // Using as generic issue type
			title := fmt.Sprintf("#%d: %s", item.Number, item.Title)

			if item.PullRequest != nil {
				title = fmt.Sprintf("PR #%d: %s", item.Number, item.Title)
			}

			activity := timeline.Activity{
				ID:          fmt.Sprintf("github-issue-%d", item.ID),
				Type:        activityType,
				Title:       title,
				Description: fmt.Sprintf("Updated in %s", item.Repository.Name),
				Timestamp:   updatedTime,
				Source:      "github",
				URL:         item.HTMLURL,
				Metadata: map[string]string{
					"repository": item.Repository.FullName,
					"number":     strconv.Itoa(item.Number),
					"state":      item.State,
					"author":     item.User.Login,
				},
			}

			allActivities = append(allActivities, activity)
		}
	}

	return allActivities, nil
}

// isSameDay checks if two times are on the same day
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
