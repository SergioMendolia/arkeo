package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/config"
	"github.com/arkeo/arkeo/internal/connectors"
)

// browserCmd is the top-level command for browser-related operations.
var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Browser history tools",
	Long: `Tools for working with browser history.

Currently supports scanning Chrome/Chromium and Firefox history databases
and interactively managing domain exclusion lists.`,
}

var browserDomainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "List visited domains and manage exclusions interactively",
	Long: `Scans browser history and displays all visited domains with visit counts.

In interactive mode (default), you can toggle which domains to exclude from
the timeline using the space bar, then press 's' to save the exclusion list
to your configuration file.

Use --no-tui for a plain table output suitable for scripting.`,
	Args: cobra.NoArgs,
	Run:  runBrowserDomains,
}

var (
	domainsDays    int
	domainsBrowser string
	domainsNoTUI   bool
)

func init() {
	browserDomainsCmd.Flags().IntVar(&domainsDays, "days", 90, "Number of days of history to scan")
	browserDomainsCmd.Flags().StringVar(&domainsBrowser, "browser", "all", "Browser to scan (chrome, firefox, or all)")
	browserDomainsCmd.Flags().BoolVar(&domainsNoTUI, "no-tui", false, "Output plain table without interactive TUI")

	browserCmd.AddCommand(browserDomainsCmd)
}

func runBrowserDomains(cmd *cobra.Command, args []string) {
	// Determine which browsers to scan
	var browsers []string
	switch strings.ToLower(domainsBrowser) {
	case "chrome":
		browsers = []string{"chrome"}
	case "firefox":
		browsers = []string{"firefox"}
	case "all", "":
		browsers = []string{"chrome", "firefox"}
	default:
		fmt.Fprintf(os.Stderr, "Invalid browser '%s'. Use chrome, firefox, or all.\n", domainsBrowser)
		os.Exit(1)
	}

	// Scan browser history for domain statistics
	fmt.Fprintf(os.Stderr, "Scanning %s browser history for the last %d days...\n",
		strings.Join(browsers, ", "), domainsDays)

	domainStats, err := connectors.ScanBrowserDomains(browsers, domainsDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning browser history: %v\n", err)
		os.Exit(1)
	}

	if len(domainStats) == 0 {
		fmt.Println("No browser history found. Make sure Chrome or Firefox is installed and has history data.")
		return
	}

	fmt.Fprintf(os.Stderr, "Found %d unique domains.\n\n", len(domainStats))

	// Load config to get current exclude list
	configManager := config.NewManager()
	if err := configManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
	}

	excludeSet := loadExcludeDomains(configManager)

	if domainsNoTUI {
		printDomainTable(domainStats, excludeSet)
		return
	}

	// Run the interactive TUI
	runDomainTUI(domainStats, excludeSet, configManager)
}

// loadExcludeDomains reads the current exclude_domains from config.
func loadExcludeDomains(configManager *config.Manager) map[string]bool {
	excludeSet := make(map[string]bool)
	if configManager == nil {
		return excludeSet
	}
	connectorConfig, exists := configManager.GetConnectorConfig("browser_history")
	if !exists || connectorConfig.Config == nil {
		return excludeSet
	}
	raw, ok := connectorConfig.Config["exclude_domains"].(string)
	if !ok {
		return excludeSet
	}
	for _, d := range strings.Split(raw, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			excludeSet[connectors.NormalizeDomain(d)] = true
		}
	}
	return excludeSet
}

// printDomainTable outputs a plain table of domains for non-interactive mode.
func printDomainTable(stats []connectors.DomainStats, excludeSet map[string]bool) {
	fmt.Printf("%-40s %8s  %6s  %s\n", "DOMAIN", "VISITS", "PAGES", "EXCLUDED")
	fmt.Println(strings.Repeat("-", 70))
	for _, s := range stats {
		excluded := "no"
		if excludeSet[s.Domain] {
			excluded = "yes"
		}
		fmt.Printf("%-40s %8d  %6d  %s\n", s.Domain, s.VisitCount, s.PageCount, excluded)
	}
}

// --- Bubble Tea TUI ---

type domainItem struct {
	domain     string
	visitCount int
	pageCount  int
	excluded   bool
}

type domainModel struct {
	items       []domainItem
	cursor      int
	filterMode  bool
	filterText  string
	filteredIdx []int
	width       int
	height      int
	statusMsg   string
	configMgr   *config.Manager
	quitting    bool
}

func runDomainTUI(stats []connectors.DomainStats, excludeSet map[string]bool, configManager *config.Manager) {
	items := make([]domainItem, len(stats))
	for i, s := range stats {
		items[i] = domainItem{
			domain:     s.Domain,
			visitCount: s.VisitCount,
			pageCount:   s.PageCount,
			excluded:    excludeSet[s.Domain],
		}
	}

	m := domainModel{
		items:     items,
		configMgr: configManager,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func (m domainModel) Init() tea.Cmd {
	return nil
}

func (m domainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.filterMode {
			return m.updateFilter(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m domainModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleItems()
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
	case " ", "enter":
		if len(visible) > 0 && m.cursor < len(visible) {
			idx := visible[m.cursor]
			m.items[idx].excluded = !m.items[idx].excluded
			m.statusMsg = ""
		}
	case "s":
		cmd := m.saveExclusions()
		return m, cmd
	case "/":
		m.filterMode = true
		m.filterText = ""
		m.cursor = 0
	case "a":
		for _, idx := range visible {
			m.items[idx].excluded = true
		}
		m.statusMsg = "Excluded all visible domains"
	case "n":
		for _, idx := range visible {
			m.items[idx].excluded = false
		}
		m.statusMsg = "Un-excluded all visible domains"
	}
	return m, nil
}

func (m domainModel) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterMode = false
		m.filterText = ""
		m.cursor = 0
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
		m.cursor = 0
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.cursor = 0
		}
	}
	return m, nil
}

func (m domainModel) saveExclusions() tea.Cmd {
	var excluded []string
	for _, item := range m.items {
		if item.excluded {
			excluded = append(excluded, item.domain)
		}
	}
	sort.Strings(excluded)

	if m.configMgr != nil {
		m.configMgr.SetConnectorConfigValue("browser_history", "exclude_domains", strings.Join(excluded, ", "))
		if err := m.configMgr.Save(); err != nil {
			m.statusMsg = fmt.Sprintf("Error saving: %v", err)
			return nil
		}
	}
	m.statusMsg = fmt.Sprintf("Saved — %d domains excluded", len(excluded))
	return nil
}

// visibleItems returns the indices of items matching the current filter.
// If no filter is active, returns all indices.
func (m domainModel) visibleItems() []int {
	if !m.filterMode || m.filterText == "" {
		idx := make([]int, len(m.items))
		for i := range m.items {
			idx[i] = i
		}
		return idx
	}
	var idx []int
	for i, item := range m.items {
		if strings.Contains(item.domain, m.filterText) {
			idx = append(idx, i)
		}
	}
	return idx
}

func (m domainModel) View() string {
	if m.quitting {
		return ""
	}

	var styles = struct {
		header    lipgloss.Style
		domain    lipgloss.Style
		count     lipgloss.Style
		excluded  lipgloss.Style
		cursor    lipgloss.Style
		status    lipgloss.Style
		filterBox lipgloss.Style
	}{
		header:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		domain:    lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		count:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		excluded:  lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		cursor:    lipgloss.NewStyle().Bold(true),
		status:    lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		filterBox: lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
	}

	var b strings.Builder

	// Header
	b.WriteString(styles.header.Render(" Browser Domain Manager"))
	b.WriteString("\n")
	b.WriteString(styles.count.Render(" Toggle exclusion with space, press 's' to save, 'q' to quit"))
	b.WriteString("\n\n")

	// Filter bar
	if m.filterMode {
		b.WriteString(styles.filterBox.Render(" Filter: "+m.filterText+"_"))
		b.WriteString("\n\n")
	}

	visible := m.visibleItems()

	// Calculate how many rows we can display
	maxRows := m.height - 7 // header + footer
	if maxRows < 1 {
		maxRows = 1
	}
	if maxRows > len(visible) {
		maxRows = len(visible)
	}

	// Determine scroll offset
	scrollOffset := 0
	if m.cursor >= maxRows {
		scrollOffset = m.cursor - maxRows + 1
	}

	for i := 0; i < maxRows; i++ {
		idx := scrollOffset + i
		if idx >= len(visible) {
			break
		}
		itemIdx := visible[idx]
		item := m.items[itemIdx]

		cursor := " "
		if idx == m.cursor {
			cursor = styles.cursor.Render(">")
		}

		check := "[ ]"
		if item.excluded {
			check = styles.excluded.Render("[x]")
		}

		domainStr := styles.domain.Render(fmt.Sprintf("%-35s", item.domain))
		countStr := styles.count.Render(fmt.Sprintf("%6d visits  %4d pages", item.visitCount, item.pageCount))

		b.WriteString(fmt.Sprintf(" %s %s %s  %s\n", cursor, check, domainStr, countStr))
	}

	// Footer
	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(styles.status.Render(" " + m.statusMsg))
		b.WriteString("\n")
	}

	// Help
	helpText := " j/k move  space toggle  s save  / filter  a all  n none  q quit"
	if m.filterMode {
		helpText = " esc exit filter  type to search"
	}
	b.WriteString(styles.count.Render(helpText))

	return b.String()
}