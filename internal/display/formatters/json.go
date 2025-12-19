package formatters

import (
	"encoding/json"
	"fmt"

	"github.com/arkeo/arkeo/internal/timeline"
)

// DisplayJSON outputs timeline as JSON for a single day
func DisplayJSON(tl *timeline.Timeline, activities []timeline.Activity) error {
	// Create a timeline copy with the limited activities
	output := &timeline.Timeline{
		Date:       tl.Date,
		Activities: activities,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal timeline to JSON: %v", err)
	}

	fmt.Print(string(jsonData))
	return nil
}

