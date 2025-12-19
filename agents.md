# Arkeo - Agent Documentation

This document provides comprehensive information for AI agents and developers working on the Arkeo project. It covers architecture, codebase structure, development practices, and common tasks.

## Project Overview

**Arkeo** is a command-line tool that automatically gathers daily activity information from various services and presents them in a chronological timeline. It's designed to help answer "What did I do on that day?" for timesheeting and activity tracking.

### Key Features

- **Multiple Connectors**: GitHub, GitLab, Google Calendar, YouTrack, macOS system events, and custom webhooks
- **Daily Timeline**: Chronological view of all activities
- **YAML Configuration**: Easy connector management
- **Secure Storage**: API tokens stored locally in `~/.config/arkeo/config.yaml`
- **Parallel Fetching**: Efficient data collection from multiple sources
- **Progress Tracking**: Status messages during data fetching

## Architecture

### High-Level Architecture

```
┌─────────────────┐
│   CLI Commands  │  (cmd/)
│  - timeline     │
│  - connectors   │
│  - config       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Config Manager │  (internal/config/)
│  - Load/Save    │
│  - Validation   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Connector       │  (internal/connectors/)
│ Registry        │
│  - GitHub       │
│  - GitLab       │
│  - Calendar     │
│  - YouTrack     │
│  - macOS System │
│  - Webhooks     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Timeline      │  (internal/timeline/)
│  - Activities   │
│  - Sorting      │
│  - Filtering    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Display      │  (internal/display/)
│  - Timeline     │
│  - Formatters   │
│    - Table      │
│    - JSON       │
│    - CSV        │
│    - Taxi       │
│  - Colors       │
└─────────────────┘
```

### Core Components

#### 1. Command Layer (`cmd/`)
- **`root.go`**: Main entry point, command registration, system initialization
- **`timeline.go`**: Timeline display command
- **`connectors.go`**: Connector management commands
- **`config.go`**: Configuration management commands

#### 2. Configuration (`internal/config/`)
- **`config.go`**: Configuration structure, loading, saving, validation
- Uses Viper for configuration management
- Stores config in `~/.config/arkeo/config.yaml`
- Supports XDG_CONFIG_HOME environment variable

#### 3. Connectors (`internal/connectors/`)
- **`connector.go`**: Base connector interface and implementation
- **`github.go`**: GitHub API integration
- **`gitlab.go`**: GitLab API integration
- **`calendar.go`**: Google Calendar iCal integration
- **`youtrack.go`**: YouTrack API integration
- **`macos_system.go`**: macOS system log integration
- **`webhooks.go`**: Generic HTTP webhook integration

#### 4. Timeline (`internal/timeline/`)
- **`activity.go`**: Activity data structure and timeline operations
- Activities are sorted chronologically
- Supports filtering by type, source, and time range

#### 5. Display (`internal/display/`)
- **`timeline.go`**: Main timeline display logic, handles single day and week views
- **`formatters/`**: Format-specific display implementations
  - **`table.go`**: Table format with colors, time gaps, and activity details
  - **`json.go`**: JSON format output
  - **`csv.go`**: CSV format output
  - **`taxi.go`**: Taxi format (timesheet format with quarter-hour rounding)
- **`colors/`**: Color utilities for terminal output

#### 6. Utilities (`internal/utils/`)
- **`parallel.go`**: Parallel connector execution
- **`http_pool.go`**: HTTP connection pooling

## Codebase Structure

```
arkeo/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command and initialization
│   ├── timeline.go        # Timeline display command
│   ├── connectors.go      # Connector management
│   └── config.go          # Configuration management
├── internal/
│   ├── config/            # Configuration management
│   │   └── config.go      # Config structs and manager
│   ├── connectors/        # Connector implementations
│   │   ├── connector.go   # Base connector interface
│   │   ├── github.go      # GitHub connector
│   │   ├── gitlab.go      # GitLab connector
│   │   ├── calendar.go    # Calendar connector
│   │   ├── youtrack.go    # YouTrack connector
│   │   ├── macos_system.go # macOS system connector
│   │   └── webhooks.go    # Webhooks connector
│   ├── timeline/          # Timeline data structures
│   │   └── activity.go    # Activity and Timeline types
│   ├── display/           # Display formatting
│   │   ├── timeline.go    # Main display logic
│   │   ├── formatters/    # Format-specific implementations
│   │   │   ├── table.go   # Table format
│   │   │   ├── json.go    # JSON format
│   │   │   ├── csv.go     # CSV format
│   │   │   └── taxi.go    # Taxi format
│   │   └── colors/        # Color utilities
│   │       └── colors.go
│   └── utils/             # Utility functions
│       ├── parallel.go    # Parallel execution
│       └── http_pool.go   # HTTP utilities
├── main.go                # Application entry point
├── go.mod                 # Go module definition
├── config.example.yaml    # Example configuration
└── README.md              # User documentation
```

## Key Data Structures

### Activity

```go
type Activity struct {
    ID          string            // Unique identifier
    Type        ActivityType      // Activity type (git_commit, calendar, etc.)
    Title       string            // Activity title
    Description string            // Detailed description
    Timestamp   time.Time         // When the activity occurred
    Duration    *time.Duration    // Optional duration
    Source      string            // Connector name (github, calendar, etc.)
    URL         string            // Optional URL to activity
    Metadata    map[string]string // Additional metadata
}
```

### Timeline

```go
type Timeline struct {
    Date       time.Time   // Date for this timeline
    Activities []Activity  // Chronologically sorted activities
}
```

### Connector Interface

```go
type Connector interface {
    Name() string
    Description() string
    IsEnabled() bool
    SetEnabled(enabled bool)
    Configure(config map[string]interface{}) error
    ValidateConfig(config map[string]interface{}) error
    GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error)
    GetRequiredConfig() []ConfigField
    TestConnection(ctx context.Context) error
}
```

## Development Workflow

### Adding a New Connector

1. **Create the connector file** (`internal/connectors/myconnector.go`):
   ```go
   package connectors
   
   import (
       "context"
       "time"
       "github.com/arkeo/arkeo/internal/timeline"
   )
   
   type MyConnector struct {
       *BaseConnector
   }
   
   func NewMyConnector() *MyConnector {
       return &MyConnector{
           BaseConnector: NewBaseConnector(
               "myconnector",
               "Fetches data from My Service",
           ),
       }
   }
   
   func (c *MyConnector) GetRequiredConfig() []ConfigField {
       return []ConfigField{
           {
               Key:         "api_key",
               Type:        "secret",
               Required:    true,
               Description: "API key for My Service",
           },
       }
   }
   
   func (c *MyConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
       // Implementation here
       return []timeline.Activity{}, nil
   }
   
   func (c *MyConnector) TestConnection(ctx context.Context) error {
       // Test connection logic
       return nil
   }
   ```

2. **Register the connector** in `cmd/root.go`:
   ```go
   availableConnectors := []connectors.Connector{
       // ... existing connectors
       connectors.NewMyConnector(),
   }
   ```

3. **Add configuration** in `internal/config/config.go`:
   - Add to `DefaultConfig()` function
   - Update `GenerateExampleConfigYAML()` if needed

4. **Write tests** (`internal/connectors/myconnector_test.go`)

### Adding a New Formatter

1. **Create the formatter file** (`internal/display/formatters/myformat.go`):
   ```go
   package formatters
   
   import (
       "fmt"
       "github.com/arkeo/arkeo/internal/timeline"
   )
   
   // DisplayMyFormat outputs timeline in custom format
   func DisplayMyFormat(tl *timeline.Timeline, activities []timeline.Activity) error {
       // Implementation here
       return nil
   }
   ```

2. **Register the formatter** in `internal/display/timeline.go`:
   - Add case to `displaySingleDay()` switch statement
   - Add case to `displayMultipleDays()` switch statement
   - Add handler function for multiple days if needed

3. **Update command flags** in `cmd/timeline.go`:
   - Add format option to `--format` flag description

4. **Write tests** (`internal/display/formatters/myformat_test.go`)

### Configuration Management

- Configuration is stored in `~/.config/arkeo/config.yaml`
- Use `config.Manager` to load/save configuration
- Connector configs are nested under `connectors.<name>.config`
- Enable/disable connectors via `connectors.<name>.enabled`

### Testing

- Unit tests use standard Go testing package
- Test files follow `*_test.go` naming convention
- Run tests with: `make test` or `go test ./...`

### Building

- **Build for current platform**: `make build`
- **Build for all platforms**: `make build-all`
- **Run checks**: `make check` (format, vet, test)
- **Create release**: `make release`

## Common Tasks

### Fetching Activities

Activities are fetched in parallel by default:

```go
// In cmd/timeline.go
enabledConnectors := getEnabledConnectors(configManager, registry)
activities := utils.FetchActivitiesParallel(ctx, utilsConnectors, targetDate, true)
```

### Displaying Timeline

The display system uses a formatter pattern to support multiple output formats:

```go
opts := display.TimelineOptions{
    MaxItems: maxItems,
    Format:   format,  // "table", "json", "csv", "taxi"
    Dates:    []time.Time{targetDate},  // Single day or multiple for week view
}
display.DisplayTimeline(activities, opts)
```

**Supported Formats:**
- **table**: Default format with colors, time gaps, and activity details
- **json**: Machine-readable JSON output
- **csv**: Comma-separated values for spreadsheet import
- **taxi**: Timesheet format with quarter-hour rounding

**Week View:**
Pass multiple dates to display a week view (Monday-Friday):
```go
opts.Dates = []time.Time{monday, tuesday, wednesday, thursday, friday}
display.DisplayTimeline(allActivities, opts)
```

### Adding Configuration Fields

1. Add to connector's `GetRequiredConfig()` method
2. Access via `BaseConnector` helper methods:
   - `GetConfigString(key)`
   - `GetConfigBool(key)`
   - `GetConfigInt(key)`

### Error Handling

- Connectors should return descriptive errors
- Use `fmt.Errorf()` with `%w` for error wrapping
- Failed connectors don't stop the entire timeline fetch
- Errors are logged but don't crash the application

## Display Formats

### Table Format (Default)
- Human-readable format with colors and visual enhancements
- Shows time gaps between activities (>1 hour)
- Displays activity source, timestamp, and title
- Includes duration if available
- Supports detailed view with descriptions in JSON mode

### JSON Format
- Machine-readable JSON output
- Full activity details including metadata
- Suitable for integration with other tools
- Supports both single day and week views

### CSV Format
- Comma-separated values for spreadsheet import
- Includes all activity fields: timestamp, type, source, title, description, duration, URL
- Proper CSV escaping for fields containing commas or quotes

### Taxi Format
- Timesheet format designed for time tracking systems
- Rounds time ranges to quarter hours (00, 15, 30, 45)
- Uses continuation format (`-HH:MM`) for consecutive activities
- Default duration: 15 minutes per activity
- Gap threshold: 30 minutes (activities within this gap are merged)
- Project placeholder: `??` (for manual project assignment)
- Format: `project HH:MM-HH:MM description` or `project -HH:MM description`

**Taxi Format Features:**
- Start times rounded down to previous quarter hour
- End times rounded up to next quarter hour
- Activities within 5 minutes of previous end time use continuation format
- Description includes activity title and source: `Title (source)`

## Connector Details

### GitHub Connector
- **Config**: `token` (PAT), `username`, `include_private`
- **Activities**: Commits, issues, pull requests
- **API**: GitHub REST API v3

### GitLab Connector
- **Config**: `gitlab_url`, `username`, `access_token`
- **Activities**: Push events from all branches
- **API**: GitLab API v4

### Calendar Connector
- **Config**: `ical_urls` (comma-separated), `include_declined`
- **Activities**: Calendar events from iCal feeds
- **Format**: iCal (RFC 5545)

### YouTrack Connector
- **Config**: `base_url`, `token`, `username`
- **Activities**: Issue updates and activities
- **API**: YouTrack REST API

### macOS System Connector
- **Config**: None required
- **Activities**: Screen lock/unlock events
- **Method**: `log show` command parsing

### Webhooks Connector
- **Config**: Array of webhook configs (`name`, `url`, `token`)
- **Activities**: Custom activities from HTTP endpoints
- **Format**: JSON array of activity objects

## Configuration Schema

### App Configuration
```yaml
app:
  date_format: "2006-01-02"  # Go time format
  log_level: "info"          # debug, info, warn, error
```

### Timeline Display Options

Display options are controlled via CLI flags, not configuration:

- **Format**: Set via `--format` flag (table, json, csv, taxi)
- **Max Items**: Set via `--max-items` flag (0 = unlimited)
- **Week View**: Enable via `--week` flag (Monday-Friday)

**Note**: Format defaults to "table" if not specified. Parallel fetching is always enabled by default.

### Connector Configuration
```yaml
connectors:
  <connector_name>:
    enabled: true/false
    config:
      # Connector-specific settings
```

## Best Practices

### Code Style
- Follow Go standard formatting (`go fmt`)
- Use `golangci-lint` for linting
- Write clear, descriptive function names
- Add comments for exported functions and types

### Error Handling
- Always check errors
- Use `fmt.Errorf()` with `%w` for wrapping
- Provide context in error messages
- Don't ignore errors silently

### Testing
- Write tests for all connectors
- Test both success and failure cases
- Use table-driven tests where appropriate
- Mock external API calls in tests

### Performance
- Use parallel fetching for multiple connectors
- Pre-allocate slices when size is known
- Use `AddActivitiesUnsorted()` for bulk operations
- Sort only when necessary
- Formatters receive pre-sorted activities when possible

### Security
- Never log API tokens or secrets
- Store sensitive data in config file (not in code)
- Validate all user input
- Use context for request cancellation

## Debugging

### Enable Debug Mode

1. **Via config**:
   ```yaml
   app:
     log_level: "debug"
   ```

2. **Via environment**:
   ```bash
   export ARKEO_DEBUG=1
   export LOG_LEVEL=debug
   ```

3. **Via connector config**:
   ```yaml
   connectors:
     github:
       config:
         debug_mode: true
   ```

### Common Issues

1. **"No connectors are enabled"**
   - Check `connectors.<name>.enabled` in config
   - Use `arkeo connectors list` to see status

2. **"Connection test failed"**
   - Verify API tokens are correct
   - Check network connectivity
   - Review connector-specific requirements

3. **"No activities found"**
   - Verify date range (activities are date-specific)
   - Check connector configuration
   - Test connector connection

## Dependencies

### Core Dependencies
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `gopkg.in/yaml.v3` - YAML parsing
- `golang.org/x/term` - Terminal utilities

### Development Dependencies
- Standard Go testing package
- `golangci-lint` (recommended)

## File Locations

- **Config**: `~/.config/arkeo/config.yaml`
- **Data**: `~/.config/arkeo/data/` (if used)
- **Logs**: Stderr (no file logging currently)

## Environment Variables

- `ARKEO_DEBUG` - Enable debug mode (1, true)
- `LOG_LEVEL` - Set log level (debug, info, warn, error)
- `XDG_CONFIG_HOME` - Override config directory location
- `--config` - Override config file path (CLI flag)

## Contributing Guidelines

1. **Code Changes**
   - Follow existing code style
   - Add tests for new features
   - Update documentation as needed

2. **New Connectors**
   - Implement `Connector` interface
   - Add to registry in `cmd/root.go`
   - Update config defaults
   - Write tests
   - Document in README

3. **Configuration Changes**
   - Update `DefaultConfig()` in `config.go`
   - Update `GenerateExampleConfigYAML()`
   - Update `config.example.yaml`
   - Document in README

## Additional Resources

- **User Documentation**: See `README.md`
- **Example Config**: See `config.example.yaml`
- **Go Module**: `github.com/arkeo/arkeo`
- **License**: MIT (see `LICENSE`)

---

*Last updated: Generated for Arkeo project*
*For questions or issues, refer to the main README.md or project repository*

