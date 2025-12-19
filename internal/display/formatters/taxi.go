package formatters

import (
	"fmt"
	"sort"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// TaxiEntry represents a single taxi format entry
type TaxiEntry struct {
	Project     string
	StartTime   time.Time
	EndTime     time.Time
	Description string
}

// DisplayTaxi outputs timeline in taxi format for a single date
func DisplayTaxi(tl *timeline.Timeline, activities []timeline.Activity) error {
	if len(activities) == 0 {
		// Still output date header even if no activities
		fmt.Printf("%s\n\n", tl.Date.Format("02/01/2006"))
		return nil
	}

	// Sort activities by timestamp
	sortActivitiesByTime(activities)

	// Calculate time ranges
	entries := calculateTimeRanges(activities)

	// Output date header
	fmt.Printf("%s\n\n", tl.Date.Format("02/01/2006"))

	// Output entries
	var lastEndTime time.Time
	for i, entry := range entries {
		startStr := entry.StartTime.Format("15:04")
		endStr := entry.EndTime.Format("15:04")

		// Check if we can use continuation format (-HH:MM)
		useContinuation := false
		if i > 0 {
			// Check if this entry starts close to where the previous ended
			gap := entry.StartTime.Sub(lastEndTime)
			if gap <= 5*time.Minute && gap >= -5*time.Minute {
				useContinuation = true
			}
		}

		if useContinuation {
			// Use continuation format: project -HH:MM description
			fmt.Printf("%-10s -%s %s\n", entry.Project, endStr, entry.Description)
		} else {
			// Use full format: project HH:MM-HH:MM description
			fmt.Printf("%-10s %s-%s %s\n", entry.Project, startStr, endStr, entry.Description)
		}

		lastEndTime = entry.EndTime
	}

	return nil
}

// roundUpToNextQuarter rounds a time up to the next quarter hour (00, 15, 30, 45)
// Always rounds up, even if already on a quarter hour
func roundUpToNextQuarter(t time.Time) time.Time {
	minutes := t.Minute()
	remainder := minutes % 15
	if remainder == 0 {
		// Already on a quarter hour, round up to next quarter
		return t.Add(15 * time.Minute).Truncate(time.Minute)
	}
	// Round up to next quarter
	minutesToAdd := 15 - remainder
	return t.Add(time.Duration(minutesToAdd) * time.Minute).Truncate(time.Minute)
}

// roundDownToPreviousQuarter rounds a time down to the previous quarter hour (00, 15, 30, 45)
// Rounds down to the current quarter if already on a quarter hour
func roundDownToPreviousQuarter(t time.Time) time.Time {
	minutes := t.Minute()
	remainder := minutes % 15
	if remainder == 0 {
		// Already on a quarter hour, return as is
		return t.Truncate(time.Minute)
	}
	// Round down to previous quarter
	return t.Add(-time.Duration(remainder) * time.Minute).Truncate(time.Minute)
}

// calculateTimeRanges converts activities to taxi entries
func calculateTimeRanges(activities []timeline.Activity) []TaxiEntry {
	if len(activities) == 0 {
		return []TaxiEntry{}
	}

	entries := make([]TaxiEntry, 0, len(activities))
	const defaultDuration = 15 * time.Minute
	const gapThreshold = 30 * time.Minute

	for i, activity := range activities {
		startTime := activity.Timestamp
		var endTime time.Time

		// Determine end time
		if activity.Duration != nil && *activity.Duration > 0 {
			endTime = startTime.Add(*activity.Duration)
		} else if i < len(activities)-1 {
			// Use next activity's timestamp if it's close enough
			nextTime := activities[i+1].Timestamp
			gap := nextTime.Sub(startTime)
			if gap <= gapThreshold {
				endTime = nextTime
			} else {
				endTime = startTime.Add(defaultDuration)
			}
		} else {
			// Last activity, use default duration
			endTime = startTime.Add(defaultDuration)
		}

		// Round start time down to the previous quarter hour
		startTime = roundDownToPreviousQuarter(startTime)
		// Round end time up to the next quarter hour
		endTime = roundUpToNextQuarter(endTime)

		// Build description
		description := fmt.Sprintf("%s (%s)", activity.Title, activity.Source)

		entries = append(entries, TaxiEntry{
			Project:     "??",
			StartTime:   startTime,
			EndTime:     endTime,
			Description: description,
		})
	}

	return entries
}

// sortActivitiesByTime sorts activities by timestamp
func sortActivitiesByTime(activities []timeline.Activity) {
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.Before(activities[j].Timestamp)
	})
}
