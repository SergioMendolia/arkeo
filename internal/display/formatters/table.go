package formatters

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arkeo/arkeo/internal/display/colors"
	"github.com/arkeo/arkeo/internal/timeline"
)

// terminalWidth is the assumed terminal width for truncation.
// Activities are truncated to fit on a single line.
const terminalWidth = 120

// DisplayTable shows the timeline in a formatted table with colors
func DisplayTable(tl *timeline.Timeline, activities []timeline.Activity, format string) error {
	if len(activities) == 0 {
		fmt.Printf("No activities found for %s\n", colors.Colorize(tl.Date.Format("January 2, 2006"), colors.Bold))
		return nil
	}

	// Display header
	DisplayHeader(tl, activities)

	// Display activities
	return DisplayChronological(activities, format)
}

// DisplayHeader shows an enhanced header with summary info
func DisplayHeader(tl *timeline.Timeline, activities []timeline.Activity) {
	title := fmt.Sprintf("Timeline for %s", tl.Date.Format("Monday, January 2, 2006"))
	fmt.Printf("%s\n", colors.Colorize(title, colors.Bold+colors.Blue))

	if len(activities) > 0 {
		start := activities[0].Timestamp.Format("15:04")
		end := activities[len(activities)-1].Timestamp.Format("15:04")
		duration := activities[len(activities)-1].Timestamp.Sub(activities[0].Timestamp)

		fmt.Printf("%s activities from %s to %s (span: %s)\n\n",
			colors.Colorize(fmt.Sprintf("%d", len(activities)), colors.Bold),
			colors.Colorize(start, colors.Green),
			colors.Colorize(end, colors.Green),
			colors.Colorize(colors.FormatDuration(duration), colors.Cyan))
	}
}

// DisplayChronological shows activities in chronological order with enhancements
func DisplayChronological(activities []timeline.Activity, format string) error {
	fmt.Printf("Activities (chronological order):\n")
	fmt.Println(colors.Colorize(strings.Repeat("═", 60), colors.Gray))

	var lastTime time.Time
	for i, activity := range activities {
		// Show time gaps (always enabled)
		if i > 0 {
			gap := activity.Timestamp.Sub(lastTime)
			if gap > 1*time.Hour {
				DisplayTimeGap(gap)
			}
		}

		DisplayActivity(activity, format, "", i == len(activities)-1)
		lastTime = activity.Timestamp
	}

	return nil
}

// DisplayActivity shows a single activity on one line.
// Format: "HH:MM  SRC  Title — Description"
// The line is truncated to terminalWidth to ensure it fits on one line.
func DisplayActivity(activity timeline.Activity, format string, prefix string, isLast bool) {
	timeStr := activity.Timestamp.Format("15:04")
	sourceLabel := colors.SourceLabels[activity.Source]
	if sourceLabel == "" {
		sourceLabel = strings.ToUpper(activity.Source[:min(3, len(activity.Source))])
	}

	// Build the main text: "Title — Description"
	mainText := activity.Title
	if activity.Duration != nil {
		mainText = fmt.Sprintf("%s (%s)", mainText, activity.FormatDuration())
	}
	if activity.Description != "" && activity.Description != activity.Title {
		mainText = fmt.Sprintf("%s — %s", mainText, activity.Description)
	}

	// Calculate visible width of the prefix parts: "HH:MM  SRC  "
	// Time is 5 chars, 2 spaces, source label is variable, 2 spaces
	prefixWidth := 5 + 2 + len(sourceLabel) + 2
	// Truncate main text to fit
	maxMainWidth := terminalWidth - prefixWidth
	if maxMainWidth < 20 {
		maxMainWidth = 20
	}
	mainText = truncateString(mainText, maxMainWidth)

	fmt.Printf("%s%s  %s  %s\n",
		prefix,
		colors.Colorize(timeStr, colors.Bold+colors.Green),
		colors.Colorize(sourceLabel, colors.DarkGray),
		colors.Colorize(mainText, colors.GetActivityColor(activity)))
}

// DisplayTimeGap shows a visual indicator for time gaps
func DisplayTimeGap(gap time.Duration) {
	gapStr := fmt.Sprintf("── %s gap ──", colors.FormatDuration(gap))
	fmt.Printf("     %s\n", colors.Colorize(gapStr, colors.Gray))
}

// truncateString truncates s to maxVisibleWidth visible (non-ANSI) characters,
// adding an ellipsis "…" if truncation occurs. ANSI color codes are not counted.
func truncateString(s string, maxVisibleWidth int) string {
	// Strip ANSI codes to measure visible width
	stripped := stripANSI(s)
	visibleWidth := utf8.RuneCountInString(stripped)

	if visibleWidth <= maxVisibleWidth {
		return s
	}

	// Truncate the visible text and add ellipsis
	if maxVisibleWidth <= 1 {
		return "…"
	}

	runes := []rune(stripped)
	truncated := string(runes[:maxVisibleWidth-1]) + "…"

	// Re-apply color: since we colorize the whole string after truncation
	// in DisplayActivity, we can just return the truncated plain text here.
	// The caller will wrap it in color codes.
	return truncated
}

// stripANSI removes ANSI escape codes from a string to measure visible width.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ANSI escape sequence: ESC [ ... m
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // skip the 'm'
			}
			i = j
		} else {
			// Copy one rune
			_, size := utf8.DecodeRuneInString(s[i:])
			b.WriteString(s[i : i+size])
			i += size
		}
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}