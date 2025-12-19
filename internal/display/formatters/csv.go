package formatters

import (
	"fmt"
	"strings"

	"github.com/arkeo/arkeo/internal/timeline"
)

// DisplayCSV outputs timeline as CSV for a single day
func DisplayCSV(activities []timeline.Activity) error {
	// CSV header
	fmt.Println("timestamp,type,source,title,description,duration,url")

	for _, activity := range activities {
		duration := ""
		if activity.Duration != nil {
			duration = activity.FormatDuration()
		}

		fmt.Printf("%s,%s,%s,%s,%s,%s,%s\n",
			activity.Timestamp.Format("2006-01-02 15:04:05"),
			activity.Type,
			activity.Source,
			CSVEscape(activity.Title),
			CSVEscape(activity.Description),
			duration,
			activity.URL,
		)
	}

	return nil
}

// CSVEscape escapes CSV fields
func CSVEscape(field string) string {
	if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
		return "\"" + strings.ReplaceAll(field, "\"", "\"\"") + "\""
	}
	return field
}

