package colors

import (
	"fmt"
	"time"

	"github.com/arkeo/arkeo/internal/timeline"
)

// ANSI color codes
const (
	Reset    = "\033[0m"
	Bold     = "\033[1m"
	Red      = "\033[31m"
	Green    = "\033[32m"
	Yellow   = "\033[33m"
	Blue     = "\033[34m"
	Magenta  = "\033[35m"
	Cyan     = "\033[36m"
	Gray     = "\033[37m"
	DarkGray = "\033[90m"
	White    = "\033[97m"
)

// Color mapping for different activity types
var TypeColors = map[timeline.ActivityType]string{
	timeline.ActivityTypeGitCommit:   Green,
	timeline.ActivityTypeCalendar:    Blue,
	timeline.ActivityTypeSlack:       Magenta,
	timeline.ActivityTypeJira:        Yellow,
	timeline.ActivityTypeYouTrack:    Cyan,
	timeline.ActivityTypeSystem:      Gray,
	timeline.ActivityTypeCustom:      White,
	timeline.ActivityTypeFile:        Yellow,
	timeline.ActivityTypeBrowser:     Blue,
	timeline.ActivityTypeApplication: Cyan,
}

// SourceLabels provides short labels for activity sources
var SourceLabels = map[string]string{
	"github":       "GH",
	"gitlab":       "GL",
	"calendar":     "CAL",
	"youtrack":     "YT",
	"slack":        "SLK",
	"jira":         "JRA",
	"macos_system": "MAC",
	"system":       "SYS",
	"file":         "FILE",
	"browser":      "WEB",
}

// Colorize adds color codes to text
func Colorize(text, color string) string {
	if color == "" {
		return text
	}
	return color + text + Reset
}

// GetActivityColor returns the appropriate color for an activity
func GetActivityColor(activity timeline.Activity) string {
	if color, exists := TypeColors[activity.Type]; exists {
		return color
	}
	return White
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

