package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProgressBar represents a progress bar with customizable appearance
type ProgressBar struct {
	total       int
	current     int
	width       int
	prefix      string
	suffix      string
	fill        string
	empty       string
	showPercent bool
	showCount   bool
	useColors   bool
	mu          sync.Mutex
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int) *ProgressBar {
	return &ProgressBar{
		total:       total,
		current:     0,
		width:       40,
		prefix:      "",
		suffix:      "",
		fill:        "█",
		empty:       "░",
		showPercent: true,
		showCount:   true,
		useColors:   true,
	}
}

// SetWidth sets the width of the progress bar
func (p *ProgressBar) SetWidth(width int) *ProgressBar {
	p.width = width
	return p
}

// SetPrefix sets the prefix text
func (p *ProgressBar) SetPrefix(prefix string) *ProgressBar {
	p.prefix = prefix
	return p
}

// SetSuffix sets the suffix text
func (p *ProgressBar) SetSuffix(suffix string) *ProgressBar {
	p.suffix = suffix
	return p
}

// SetFill sets the fill and empty characters
func (p *ProgressBar) SetFill(fill, empty string) *ProgressBar {
	p.fill = fill
	p.empty = empty
	return p
}

// SetShowPercent controls whether to show percentage
func (p *ProgressBar) SetShowPercent(show bool) *ProgressBar {
	p.showPercent = show
	return p
}

// SetShowCount controls whether to show count
func (p *ProgressBar) SetShowCount(show bool) *ProgressBar {
	p.showCount = show
	return p
}

// SetUseColors controls whether to use colors
func (p *ProgressBar) SetUseColors(use bool) *ProgressBar {
	p.useColors = use
	return p
}

// Update updates the progress and displays the bar
func (p *ProgressBar) Update(current int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = current
	if p.current > p.total {
		p.current = p.total
	}
	p.render()
}

// Increment increments the progress by 1
func (p *ProgressBar) Increment() {
	p.Update(p.current + 1)
}

// Finish completes the progress bar
func (p *ProgressBar) Finish() {
	p.Update(p.total)
	fmt.Println() // New line after completion
}

// render displays the current state of the progress bar (for single bar usage)
func (p *ProgressBar) render() {
	display := p.buildDisplay()

	// Clear the line and print (for single progress bar)
	fmt.Printf("\r%s", strings.Repeat(" ", 100)) // Clear line
	fmt.Printf("\r%s", display)
}

// buildDisplay builds the display string without printing it
func (p *ProgressBar) buildDisplay() string {
	percentage := 0
	if p.total > 0 {
		percentage = (p.current * 100) / p.total
	}

	filled := (p.current * p.width) / p.total
	if filled > p.width {
		filled = p.width
	}

	bar := strings.Repeat(p.fill, filled) + strings.Repeat(p.empty, p.width-filled)

	// Add colors if enabled
	if p.useColors {
		if percentage >= 100 {
			bar = "\033[32m" + bar + "\033[0m" // Green
		} else if percentage >= 50 {
			bar = "\033[33m" + bar + "\033[0m" // Yellow
		} else {
			bar = "\033[31m" + bar + "\033[0m" // Red
		}
	}

	// Build the display string
	display := ""

	if p.prefix != "" {
		display += p.prefix + " "
	}

	display += "[" + bar + "]"

	if p.showPercent {
		display += fmt.Sprintf(" %3d%%", percentage)
	}

	if p.showCount {
		display += fmt.Sprintf(" (%d/%d)", p.current, p.total)
	}

	if p.suffix != "" {
		display += " " + p.suffix
	}

	return display
}

// Spinner represents a spinning progress indicator
type Spinner struct {
	chars  []string
	index  int
	prefix string
	suffix string
	active bool
	mu     sync.Mutex
	done   chan bool
}

// NewSpinner creates a new spinner
func NewSpinner() *Spinner {
	return &Spinner{
		chars:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index:  0,
		prefix: "",
		suffix: "",
		active: false,
		done:   make(chan bool),
	}
}

// SetChars sets custom spinner characters
func (s *Spinner) SetChars(chars []string) *Spinner {
	s.chars = chars
	return s
}

// SetPrefix sets the prefix text
func (s *Spinner) SetPrefix(prefix string) *Spinner {
	s.prefix = prefix
	return s
}

// SetSuffix sets the suffix text
func (s *Spinner) SetSuffix(suffix string) *Spinner {
	s.suffix = suffix
	return s
}

// Start starts the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				if !s.active {
					s.mu.Unlock()
					return
				}
				s.render()
				s.index = (s.index + 1) % len(s.chars)
				s.mu.Unlock()
			case <-s.done:
				return
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	s.mu.Unlock()

	s.done <- true
	fmt.Printf("\r%s\r", strings.Repeat(" ", 100)) // Clear the line
}

// UpdateSuffix updates the suffix text while spinning
func (s *Spinner) UpdateSuffix(suffix string) {
	s.mu.Lock()
	s.suffix = suffix
	s.mu.Unlock()
}

// render displays the current spinner state
func (s *Spinner) render() {
	display := ""

	if s.prefix != "" {
		display += s.prefix + " "
	}

	display += s.chars[s.index]

	if s.suffix != "" {
		display += " " + s.suffix
	}

	fmt.Printf("\r%s", strings.Repeat(" ", 100)) // Clear line
	fmt.Printf("\r%s", display)
}

// MultiProgress manages multiple progress bars
type MultiProgress struct {
	bars    []*NamedProgressBar
	mu      sync.Mutex
	started bool
	aligned bool
}

// NamedProgressBar wraps a progress bar with a name
type NamedProgressBar struct {
	name string
	bar  *ProgressBar
}

// NewMultiProgress creates a new multi-progress manager
func NewMultiProgress() *MultiProgress {
	return &MultiProgress{
		bars:    make([]*NamedProgressBar, 0),
		started: false,
		aligned: false,
	}
}

// AddBar adds a named progress bar
func (mp *MultiProgress) AddBar(name string, total int) *ProgressBar {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	bar := NewProgressBar(total).
		SetPrefix(name).
		SetUseColors(true).
		SetWidth(30)

	mp.bars = append(mp.bars, &NamedProgressBar{
		name: name,
		bar:  bar,
	})

	return bar
}

// updateAlignment updates all bar prefixes to be aligned
func (mp *MultiProgress) updateAlignment() {
	if len(mp.bars) == 0 || mp.aligned {
		return
	}

	// Find the longest name
	maxLength := 0
	for _, namedBar := range mp.bars {
		if len(namedBar.name) > maxLength {
			maxLength = len(namedBar.name)
		}
	}

	// Update all bars with padded prefixes
	for _, namedBar := range mp.bars {
		paddedName := fmt.Sprintf("%-*s", maxLength, namedBar.name)
		namedBar.bar.SetPrefix(paddedName)
	}

	mp.aligned = true
}

// AlignBars manually triggers alignment (call after all bars are added)
func (mp *MultiProgress) AlignBars() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.aligned = false // Reset alignment flag
	mp.updateAlignment()
}

// Render displays all progress bars
func (mp *MultiProgress) Render() {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Update alignment if this is the first render or bars were added
	if !mp.started {
		mp.updateAlignment()
	}

	// Move cursor up to overwrite previous output
	if mp.started {
		fmt.Printf("\033[%dA", len(mp.bars))
	}
	mp.started = true

	// Clear and render each progress bar line
	for i, namedBar := range mp.bars {
		// Move to beginning of line and clear it
		fmt.Print("\r\033[K")

		// Build and print the display string
		display := namedBar.bar.buildDisplay()
		fmt.Print(display)

		// Move to next line (except for the last one)
		if i < len(mp.bars)-1 {
			fmt.Print("\n")
		}
	}
}

// Status represents a status message with different levels
type Status struct {
	useColors bool
}

// NewStatus creates a new status reporter
func NewStatus(useColors bool) *Status {
	return &Status{useColors: useColors}
}

// Info displays an info message
func (s *Status) Info(msg string) {
	icon := "INFO"
	color := "\033[36m" // Cyan
	s.print(icon, color, "INFO", msg)
}

// Success displays a success message
func (s *Status) Success(msg string) {
	icon := "SUCCESS"
	color := "\033[32m" // Green
	s.print(icon, color, "SUCCESS", msg)
}

// Warning displays a warning message
func (s *Status) Warning(msg string) {
	icon := "WARNING"
	color := "\033[33m" // Yellow
	s.print(icon, color, "WARNING", msg)
}

// Error displays an error message
func (s *Status) Error(msg string) {
	icon := "ERROR"
	color := "\033[31m" // Red
	s.print(icon, color, "ERROR", msg)
}

// Progress displays a progress message
func (s *Status) Progress(msg string) {
	icon := "PROGRESS"
	color := "\033[34m" // Blue
	s.print(icon, color, "PROGRESS", msg)
}

// print formats and displays a status message
func (s *Status) print(icon, color, level, msg string) {
	if s.useColors {
		fmt.Printf("%s%s%s %s\n", color, level, "\033[0m", msg)
	} else {
		fmt.Printf("%s %s\n", level, msg)
	}
}

// ConnectorProgress tracks progress for multiple connectors
type ConnectorProgress struct {
	connectors      map[string]*ConnectorStatus
	progressBars    map[string]*ProgressBar
	multiProgress   *MultiProgress
	status          *Status
	mu              sync.Mutex
	useProgressBars bool
}

// ConnectorStatus represents the status of a single connector
type ConnectorStatus struct {
	name      string
	status    string
	error     error
	count     int
	total     int
	startTime time.Time
}

// NewConnectorProgress creates a new connector progress tracker
func NewConnectorProgress(useColors bool) *ConnectorProgress {
	return &ConnectorProgress{
		connectors:      make(map[string]*ConnectorStatus),
		progressBars:    make(map[string]*ProgressBar),
		multiProgress:   NewMultiProgress(),
		status:          NewStatus(useColors),
		useProgressBars: true,
	}
}

// AlignProgressBars triggers alignment of all progress bars (call after all connectors are added)
func (cp *ConnectorProgress) AlignProgressBars() {
	if cp.useProgressBars {
		cp.multiProgress.AlignBars()
		cp.multiProgress.Render() // Initial render after alignment
	}
}

// NewConnectorProgressSimple creates a connector progress tracker without progress bars (legacy mode)
func NewConnectorProgressSimple(useColors bool) *ConnectorProgress {
	return &ConnectorProgress{
		connectors:      make(map[string]*ConnectorStatus),
		status:          NewStatus(useColors),
		useProgressBars: false,
	}
}

// StartConnector starts tracking a connector
func (cp *ConnectorProgress) StartConnector(name string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.connectors[name] = &ConnectorStatus{
		name:      name,
		status:    "connecting",
		startTime: time.Now(),
	}

	if cp.useProgressBars {
		// Create a progress bar for this connector
		bar := cp.multiProgress.AddBar(name, 1)
		bar.Update(0) // Start at 0%
		cp.progressBars[name] = bar
		// Don't render immediately - wait for alignment
	} else {
		cp.status.Progress(fmt.Sprintf("Starting %s connector...", name))
	}
}

// UpdateConnector updates connector status
func (cp *ConnectorProgress) UpdateConnector(name, status string, count, total int) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if connector, exists := cp.connectors[name]; exists {
		connector.status = status
		connector.count = count
		connector.total = total

		if cp.useProgressBars {
			if bar, exists := cp.progressBars[name]; exists {
				bar.SetSuffix(status)
				if total > 0 {
					bar.Update(count)
				}
				cp.multiProgress.Render()
			}
		} else {
			if total > 0 {
				percentage := (count * 100) / total
				cp.status.Progress(fmt.Sprintf("%s: %s (%d%%, %d/%d)", name, status, percentage, count, total))
			} else {
				cp.status.Progress(fmt.Sprintf("%s: %s (%d items)", name, status, count))
			}
		}
	}
}

// FinishConnector marks a connector as finished
func (cp *ConnectorProgress) FinishConnector(name string, count int, err error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if connector, exists := cp.connectors[name]; exists {
		connector.count = count
		connector.error = err
		duration := time.Since(connector.startTime)

		if cp.useProgressBars {
			if bar, exists := cp.progressBars[name]; exists {
				if err != nil {
					connector.status = "failed"
					bar.SetSuffix("failed")
				} else {
					connector.status = "completed"
					bar.SetSuffix(fmt.Sprintf("completed (%d activities)", count))
				}
				bar.Update(1) // Complete the bar
				cp.multiProgress.Render()
			}
		} else {
			if err != nil {
				connector.status = "failed"
				cp.status.Error(fmt.Sprintf("%s failed: %v", name, err))
			} else {
				connector.status = "completed"
				cp.status.Success(fmt.Sprintf("%s completed: %d activities in %v", name, count, duration.Round(time.Millisecond)))
			}
		}
	}
}

// HasConnectorError returns true if a connector had an error
func (cp *ConnectorProgress) HasConnectorError(name string) bool {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if connector, exists := cp.connectors[name]; exists {
		return connector.error != nil
	}
	return false
}

// IsConnectorFinished returns true if a connector is already finished (completed or failed)
func (cp *ConnectorProgress) IsConnectorFinished(name string) bool {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if connector, exists := cp.connectors[name]; exists {
		return connector.status == "completed" || connector.status == "failed"
	}
	return false
}

// PrintSummary prints a summary of all connectors
func (cp *ConnectorProgress) PrintSummary() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if len(cp.connectors) == 0 {
		return
	}

	// Add extra space after progress bars if we used them
	if cp.useProgressBars {
		fmt.Println("\n") // Two newlines to ensure separation
	}

	totalActivities := 0
	successCount := 0
	errorCount := 0

	for _, connector := range cp.connectors {
		switch connector.status {
		case "completed":
			successCount++
			totalActivities += connector.count
		case "failed":
			errorCount++
		}
	}

	cp.status.Info(fmt.Sprintf("Total activities fetched: %d", totalActivities))
	cp.status.Info(fmt.Sprintf("Successful connectors: %d", successCount))

	if errorCount > 0 {
		cp.status.Warning(fmt.Sprintf("Failed connectors: %d", errorCount))
	}

	fmt.Println()
}

// TaskProgress represents progress for a long-running task
type TaskProgress struct {
	name      string
	steps     []string
	current   int
	spinner   *Spinner
	status    *Status
	startTime time.Time
}

// NewTaskProgress creates a new task progress tracker
func NewTaskProgress(name string, steps []string, useColors bool) *TaskProgress {
	return &TaskProgress{
		name:      name,
		steps:     steps,
		current:   0,
		spinner:   NewSpinner().SetPrefix("PROGRESS"),
		status:    NewStatus(useColors),
		startTime: time.Now(),
	}
}

// Start starts the task
func (tp *TaskProgress) Start() {
	tp.status.Info(fmt.Sprintf("Starting %s...", tp.name))
	if len(tp.steps) > 0 {
		tp.NextStep()
	}
}

// NextStep moves to the next step
func (tp *TaskProgress) NextStep() {
	if tp.current < len(tp.steps) {
		tp.spinner.Stop()

		stepName := tp.steps[tp.current]
		tp.spinner.SetSuffix(fmt.Sprintf("Step %d/%d: %s", tp.current+1, len(tp.steps), stepName))
		tp.spinner.Start()
		tp.current++
	}
}

// Finish completes the task
func (tp *TaskProgress) Finish(success bool, message string) {
	tp.spinner.Stop()
	duration := time.Since(tp.startTime)

	if success {
		tp.status.Success(fmt.Sprintf("%s completed in %v: %s", tp.name, duration.Round(time.Millisecond), message))
	} else {
		tp.status.Error(fmt.Sprintf("%s failed after %v: %s", tp.name, duration.Round(time.Millisecond), message))
	}
}

// UpdateStep updates the current step message
func (tp *TaskProgress) UpdateStep(message string) {
	if tp.current > 0 {
		stepName := fmt.Sprintf("Step %d/%d: %s", tp.current, len(tp.steps), message)
		tp.spinner.UpdateSuffix(stepName)
	}
}
