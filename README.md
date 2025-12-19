# Arkeo - Daily Activity Timeline Builder

Arkeo is a command-line tool that connects to various services to automatically gather information about your daily activities and presents them in a chronological timeline. Instead of manually logging your activities or monitoring your computer actively, Arkeo collects data from the connected services after they have happened so you don't need to worry about having an application running in the background.

The tool is designed to answer the question "What the hell did I do on that day?" when you need to recall your daily activities for timesheeting.

## Features

- 🔗 **Multiple Connectors**: Connect to GitHub, GitLab, Google Calendar, YouTrack, macOS system events, custom webhooks and more
- 📅 **Daily Timeline**: View all your activities in chronological order
- 📊 **Multiple Output Formats**: Table, JSON, CSV, and Taxi (timesheet) formats
- 📆 **Week View**: Display activities for an entire work week (Monday-Friday)
- ⚙️ **Easy Configuration**: Manage connectors through YAML configuration
- 🔒 **Secure Storage**: API tokens and sensitive data stored locally

## Installation

### From Source

```bash
git clone https://github.com/sergiomendolia/arkeo.git
cd arkeo
make build
```

### Using Go Install

```bash
go install github.com/sergiomendolia/arkeo@latest
```

### Pre-built Binaries

Download the latest release from [GitHub Releases](https://github.com/sergiomendolia/arkeo/releases)

## Quick Start

1. **Configure your first connector**:
   ```bash
   # Edit the configuration file
   # Edit ~/.config/arkeo/config.yaml directly

   # Enable a connector
   arkeo connectors enable github
   ```

2. **View your timeline**:
   ```bash
   # Show yesterday's activities (default)
   arkeo timeline

   # Show timeline for specific date
   arkeo timeline 2023-12-25

   # Show activities for the entire work week (Monday-Friday)
   arkeo timeline --week

   # Output in different formats
   arkeo timeline --format json    # JSON output
   arkeo timeline --format csv     # CSV output
   arkeo timeline --format taxi    # Taxi format (timesheet format)
   arkeo timeline --format table   # Table format (default)

   # Limit number of activities shown
   arkeo timeline --max-items 100
   ```



## Available Connectors

### GitHub Connector
Fetches your commits, issues, and pull request activities.

### GitLab Connector
Fetches user activities from GitLab.

### Google Calendar
Retrieves calendar events and meetings.

### YouTrack Connector
Fetches activities and issue updates from YouTrack.

### macOS System Events Connector
Fetches screen lock/unlock events on macOS systems using system logs. This connector:
- Monitors when your computer becomes idle (screen locks)
- Tracks when your computer becomes active (screen unlocks)
- Only works on macOS systems
- Requires no additional configuration beyond enabling it
- Uses the macOS `log show` command to retrieve loginwindow events

**Activities Generated:**
- "Computer is idle" - when the screen is locked
- "Computer is active" - when the screen is unlocked

### Webhooks Connector
Fetches activities from custom HTTP webhook endpoints. This connector allows you to integrate any service that can provide activity data via HTTP API.

**Features:**
- Support for multiple webhook endpoints
- Bearer token authentication
- Configurable display names for each webhook source
- Flexible activity data format
- Error resilience (continues with other webhooks if one fails)

**Configuration:**
Each webhook requires:
- `name`: Display name for activities from this webhook
- `url`: HTTP endpoint URL 
- `token`: Bearer token for authentication

**API Contract:**
Arkeo calls your webhook with: `GET {url}?date=YYYY-MM-DD`

Your webhook should respond with JSON array of activities:
```json
[
  {
    "timestamp": "2023-12-25T10:30:00Z",
    "title": "Completed task XYZ",
    "description": "Optional detailed description",
    "type": "task",
    "metadata": {
      "project": "MyProject",
      "priority": "high"
    }
  }
]
```

**Activity Fields:**
- `timestamp` (required): ISO 8601 timestamp (RFC3339 format preferred)
- `title` (required): Activity title/summary
- `description` (optional): Detailed description
- `type` (optional): Activity type (defaults to "webhook")
- `metadata` (optional): Additional key-value data

**Supported timestamp formats:**
- `2023-12-25T10:30:00Z` (RFC3339 - preferred)
- `2023-12-25T10:30:00+01:00` (RFC3339 with timezone)
- `2023-12-25 10:30:00` (Simple format)

**Example Configuration:**
```yaml
webhooks:
  enabled: true
  config:
    webhooks:
      - name: "JIRA Tasks"
        url: "https://api.mycompany.com/jira-activities"
        token: "Bearer-token-for-jira-api"
      - name: "Time Tracker"
        url: "https://timetracker.mycompany.com/api/activities"
        token: "another-bearer-token"
```



## Output Formats

Arkeo supports multiple output formats for different use cases:

- **table** (default): Human-readable formatted table with colors and time gaps
- **json**: Machine-readable JSON format for integration with other tools
- **csv**: Comma-separated values for spreadsheet import
- **taxi**: Timesheet format with time ranges rounded to quarter hours, suitable for time tracking systems

### Taxi Format

The taxi format is designed for timesheet entry. It:
- Rounds time ranges to quarter hours (00, 15, 30, 45)
- Groups activities into time blocks
- Uses continuation format (`-HH:MM`) when activities are consecutive
- Includes project placeholder (`??`) for manual project assignment

Example taxi output:
```
25/12/2023

??         09:00-09:15 Commit: Fix bug in API (github)
??         -09:30 Review PR #123 (github)
??         10:00-10:30 Team standup (calendar)
```

## Configuration

arkeo stores configuration in `~/.config/arkeo/config.yaml`. You can edit this file directly with your preferred editor.

### Example Configuration
See [config.example.yaml](config.example.yaml) for a complete configuration example.


## Development

### Adding Custom Connectors

Create a new connector by implementing the `Connector` interface:

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
            "myservice",
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
    // Implement your connector logic here
    return []timeline.Activity{}, nil
}
```

Then register it in `cmd/root.go`:
```go
registry.Register(NewMyConnector())
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run all checks (format, vet, test)
make check

# Create release package
make release
```

## Troubleshooting

### Common Issues

1. **"No connectors are enabled"**
   - Enable connectors: `arkeo connectors enable <name>`
   - Check configuration: `arkeo config show`

2. **"Connection test failed"**
   - Verify API tokens and credentials
   - Test connection: `arkeo connectors test <name>`

3. **"No activities found"**
   - Check date range: activities are fetched for the specific date
   - Verify connector is properly configured
   - Test connector connection



### Debug Mode

Enable debug logging:
```yaml
app:
  log_level: "debug"
```

## Creating a Release
```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions will automatically:
# 1. Run all tests
# 2. Build multi-platform binaries
# 3. Create release archives
# 4. Publish GitHub release
```

The release will include binaries for:
- Linux (AMD64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (AMD64)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
