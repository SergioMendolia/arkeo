package formatters

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// jsonActivity is a projection of timeline.Activity that omits the Metadata
// field from JSON output.
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

// jsonTimeline is a projection of timeline.Timeline that uses jsonActivity.
type jsonTimeline struct {
	Date       time.Time      `json:"date"`
	Activities []jsonActivity `json:"activities"`
}

// toJSONActivity converts a timeline.Activity to a jsonActivity (no metadata).
func toJSONActivity(a timeline.Activity) jsonActivity {
	return jsonActivity{
		ID:          a.ID,
		Type:        a.Type,
		Title:       a.Title,
		Description: a.Description,
		Timestamp:   a.Timestamp,
		Duration:    a.Duration,
		Source:      a.Source,
		URL:         a.URL,
	}
}

// MarshalTimelineJSON converts a timeline and its activities to JSON,
// omitting the Metadata field from each activity.
func MarshalTimelineJSON(tl *timeline.Timeline, activities []timeline.Activity) ([]byte, error) {
	activitiesOut := make([]jsonActivity, len(activities))
	for i, a := range activities {
		activitiesOut[i] = toJSONActivity(a)
	}

	output := &jsonTimeline{
		Date:       tl.Date,
		Activities: activitiesOut,
	}

	return json.MarshalIndent(output, "", "  ")
}

// DisplayJSON outputs timeline as JSON for a single day, omitting metadata.
func DisplayJSON(tl *timeline.Timeline, activities []timeline.Activity) error {
	jsonData, err := MarshalTimelineJSON(tl, activities)
	if err != nil {
		return fmt.Errorf("failed to marshal timeline to JSON: %v", err)
	}

	fmt.Print(string(jsonData))
	return nil
}