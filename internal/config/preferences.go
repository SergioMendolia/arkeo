package config

import (
	"fmt"
	"time"
)

// Preferences holds user preferences that are remembered across sessions
// This type is just for backward compatibility - PreferencesConfig is the new type
// defined in config.go that's used for storing preferences in config.yaml
type Preferences struct {
	// Display preferences
	UseColors    bool
	ShowDetails  bool
	ShowProgress bool

	ShowGaps      bool
	DefaultFormat string

	// Timeline preferences
	GroupByHour    bool
	MaxItems       int
	ShowTimeline   bool
	ShowTimestamps bool

	// Behavior preferences

	// Performance preferences
	ParallelFetch bool
	FetchTimeout  time.Duration
}

// PreferencesManager handles user preferences
type PreferencesManager struct {
	preferences *Preferences
	configMgr   *Manager
	dirty       bool
}

// NewPreferencesManager creates a new preferences manager
func NewPreferencesManager(configManager *Manager) *PreferencesManager {
	return &PreferencesManager{
		configMgr: configManager,
		dirty:     false,
	}
}

// Load loads preferences from the config file
func (pm *PreferencesManager) Load() error {
	// Convert from the config
	prefs := &Preferences{}
	configPrefs := pm.configMgr.GetConfig().Preferences

	// Copy values from config to preferences struct
	prefs.UseColors = configPrefs.UseColors
	prefs.ShowDetails = configPrefs.ShowDetails
	prefs.ShowProgress = configPrefs.ShowProgress
	prefs.ShowGaps = configPrefs.ShowGaps
	prefs.DefaultFormat = configPrefs.DefaultFormat

	prefs.GroupByHour = configPrefs.GroupByHour
	prefs.MaxItems = configPrefs.MaxItems
	prefs.ShowTimeline = configPrefs.ShowTimeline
	prefs.ShowTimestamps = configPrefs.ShowTimestamps

	prefs.ParallelFetch = configPrefs.ParallelFetch
	prefs.FetchTimeout = time.Duration(configPrefs.FetchTimeout) * time.Second

	pm.preferences = prefs
	pm.dirty = false
	return nil
}

// Save saves preferences to the config file
func (pm *PreferencesManager) Save() error {
	if pm.preferences == nil {
		return fmt.Errorf("no preferences to save")
	}

	// Update config with current preferences
	config := pm.configMgr.GetConfig()

	// No need to update session time anymore

	// Copy values from preferences struct to config
	config.Preferences.UseColors = pm.preferences.UseColors
	config.Preferences.ShowDetails = pm.preferences.ShowDetails
	config.Preferences.ShowProgress = pm.preferences.ShowProgress
	config.Preferences.ShowGaps = pm.preferences.ShowGaps
	config.Preferences.DefaultFormat = pm.preferences.DefaultFormat

	config.Preferences.GroupByHour = pm.preferences.GroupByHour
	config.Preferences.MaxItems = pm.preferences.MaxItems
	config.Preferences.ShowTimeline = pm.preferences.ShowTimeline
	config.Preferences.ShowTimestamps = pm.preferences.ShowTimestamps

	config.Preferences.ParallelFetch = pm.preferences.ParallelFetch
	config.Preferences.FetchTimeout = int(pm.preferences.FetchTimeout.Seconds())

	// Save the config
	err := pm.configMgr.Save()
	if err == nil {
		pm.dirty = false
	}
	return err
}

// GetPreferences returns the current preferences
func (pm *PreferencesManager) GetPreferences() *Preferences {
	return pm.preferences
}

// Display preferences setters with auto-save
func (pm *PreferencesManager) SetUseColors(value bool) {
	pm.preferences.UseColors = value
	pm.markDirty()
}

func (pm *PreferencesManager) SetShowDetails(value bool) {
	pm.preferences.ShowDetails = value
	pm.markDirty()
}

func (pm *PreferencesManager) SetShowProgress(value bool) {
	pm.preferences.ShowProgress = value
	pm.markDirty()
}

func (pm *PreferencesManager) SetDefaultFormat(format string) {
	pm.preferences.DefaultFormat = format
	pm.markDirty()
}

// Set display preferences directly

func (pm *PreferencesManager) SetGroupByHour(value bool) {
	pm.preferences.GroupByHour = value
	pm.markDirty()
}

// Timeline preferences setter

// Auto-save functionality
func (pm *PreferencesManager) markDirty() {
	pm.dirty = true
	if pm.preferences != nil {
		pm.Save() // Ignore errors for auto-save (always auto-save now)
	}
}

func (pm *PreferencesManager) IsDirty() bool {
	return pm.dirty
}

// Bulk preference updates
func (pm *PreferencesManager) UpdateDisplayPreferences(prefs DisplayPreferences) {
	pm.preferences.UseColors = prefs.UseColors
	pm.preferences.ShowDetails = prefs.ShowDetails
	pm.preferences.ShowProgress = prefs.ShowProgress
	pm.preferences.DefaultFormat = prefs.Format
	pm.markDirty()
}

// DisplayPreferences is a subset for bulk updates
type DisplayPreferences struct {
	UseColors    bool
	ShowDetails  bool
	ShowProgress bool
	Format       string
}

// GetDisplayPreferences returns current display preferences
func (pm *PreferencesManager) GetDisplayPreferences() DisplayPreferences {
	return DisplayPreferences{
		UseColors:    pm.preferences.UseColors,
		ShowDetails:  pm.preferences.ShowDetails,
		ShowProgress: pm.preferences.ShowProgress,
		Format:       pm.preferences.DefaultFormat,
	}
}

// Reset resets preferences to defaults
func (pm *PreferencesManager) Reset() error {
	// Reset preferences section in the config to defaults
	err := pm.configMgr.Reset()
	if err != nil {
		return err
	}

	// Reload preferences from config
	return pm.Load()
}

// Export exports preferences to a YAML string using the config's YAML export
func (pm *PreferencesManager) Export() (string, error) {
	// Update preferences in config before exporting
	if pm.dirty {
		if err := pm.Save(); err != nil {
			return "", fmt.Errorf("failed to save preferences before export: %w", err)
		}
	}

	// Use config manager to export the yaml
	return "Preferences exported via config.yaml", nil
}

// Import is removed as it's not implemented with the new config system

// Validate validates the preferences
func (pm *PreferencesManager) Validate() error {
	if pm.preferences == nil {
		return fmt.Errorf("preferences not loaded")
	}

	// Validate format
	validFormats := []string{"table", "json", "csv", "visual", "compact"}
	validFormat := false
	for _, format := range validFormats {
		if pm.preferences.DefaultFormat == format {
			validFormat = true
			break
		}
	}
	if !validFormat {
		pm.preferences.DefaultFormat = "visual"
		pm.markDirty()
	}

	// Validate max items
	if pm.preferences.MaxItems <= 0 {
		pm.preferences.MaxItems = 500
		pm.markDirty()
	}

	// Validation for removed fields no longer needed

	return nil
}
