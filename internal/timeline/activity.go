package timeline

import (
	"fmt"
	"sort"
	"time"
)

// ActivityType represents the type/source of an activity
type ActivityType string

const (
	ActivityTypeGitCommit   ActivityType = "git_commit"
	ActivityTypeCalendar    ActivityType = "calendar"
	ActivityTypeSlack       ActivityType = "slack"
	ActivityTypeJira        ActivityType = "jira"
	ActivityTypeYouTrack    ActivityType = "youtrack"
	ActivityTypeCustom      ActivityType = "custom"
	ActivityTypeFile        ActivityType = "file"
	ActivityTypeBrowser     ActivityType = "browser"
	ActivityTypeApplication ActivityType = "application"
)

// Activity represents a single activity/event in the timeline
type Activity struct {
	ID          string            `json:"id"`
	Type        ActivityType      `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    *time.Duration    `json:"duration,omitempty"`
	Source      string            `json:"source"` // e.g., "github", "google-calendar"
	URL         string            `json:"url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Timeline represents a collection of activities for a specific day
type Timeline struct {
	Date       time.Time  `json:"date"`
	Activities []Activity `json:"activities"`
}

// NewTimeline creates a new timeline for the given date
func NewTimeline(date time.Time) *Timeline {
	return &Timeline{
		Date:       date,
		Activities: make([]Activity, 0),
	}
}

// AddActivity adds an activity to the timeline
func (t *Timeline) AddActivity(activity Activity) {
	t.Activities = append(t.Activities, activity)
	t.Sort()
}

// AddActivities adds multiple activities to the timeline
func (t *Timeline) AddActivities(activities []Activity) {
	t.Activities = append(t.Activities, activities...)
	t.Sort()
}

// Sort sorts activities by timestamp
func (t *Timeline) Sort() {
	sort.Slice(t.Activities, func(i, j int) bool {
		return t.Activities[i].Timestamp.Before(t.Activities[j].Timestamp)
	})
}

// FilterByType returns activities of a specific type
func (t *Timeline) FilterByType(activityType ActivityType) []Activity {
	var filtered []Activity
	for _, activity := range t.Activities {
		if activity.Type == activityType {
			filtered = append(filtered, activity)
		}
	}
	return filtered
}

// FilterBySource returns activities from a specific source
func (t *Timeline) FilterBySource(source string) []Activity {
	var filtered []Activity
	for _, activity := range t.Activities {
		if activity.Source == source {
			filtered = append(filtered, activity)
		}
	}
	return filtered
}

// FilterByTimeRange returns activities within a specific time range
func (t *Timeline) FilterByTimeRange(start, end time.Time) []Activity {
	var filtered []Activity
	for _, activity := range t.Activities {
		if activity.Timestamp.After(start) && activity.Timestamp.Before(end) {
			filtered = append(filtered, activity)
		}
	}
	return filtered
}

// GetTimeRange returns the earliest and latest timestamps in the timeline
func (t *Timeline) GetTimeRange() (start, end time.Time) {
	if len(t.Activities) == 0 {
		return time.Time{}, time.Time{}
	}

	start = t.Activities[0].Timestamp
	end = t.Activities[len(t.Activities)-1].Timestamp

	return start, end
}

// GetSummary returns a summary of the timeline
func (t *Timeline) GetSummary() TimelineSummary {
	summary := TimelineSummary{
		Date:            t.Date,
		TotalActivities: len(t.Activities),
		ByType:          make(map[ActivityType]int),
		BySource:        make(map[string]int),
	}

	for _, activity := range t.Activities {
		summary.ByType[activity.Type]++
		summary.BySource[activity.Source]++
	}

	start, end := t.GetTimeRange()
	summary.TimeRange.Start = start
	summary.TimeRange.End = end

	return summary
}

// TimelineSummary provides an overview of the timeline
type TimelineSummary struct {
	Date            time.Time            `json:"date"`
	TotalActivities int                  `json:"total_activities"`
	ByType          map[ActivityType]int `json:"by_type"`
	BySource        map[string]int       `json:"by_source"`
	TimeRange       struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"time_range"`
}

// String returns a string representation of the activity
func (a *Activity) String() string {
	return fmt.Sprintf("[%s] %s - %s (%s)",
		a.Timestamp.Format("15:04"),
		a.Type,
		a.Title,
		a.Source,
	)
}

// FormatDuration returns a human-readable duration string
func (a *Activity) FormatDuration() string {
	if a.Duration == nil {
		return "N/A"
	}

	duration := *a.Duration
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}
