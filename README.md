# AutoTime - Daily Activity Timeline Builder

AutoTime is a command-line tool that connects to various services to automatically gather information about your daily activities and presents them in a chronological timeline. Built with Go for cross-platform compatibility and ease of use.

## Features

- 🔗 **Multiple Connectors**: Connect to GitHub, GitLab, Google Calendar, File System, and more
- 📅 **Daily Timeline**: View all your activities in chronological order
- ⚙️ **Easy Configuration**: Manage connectors through YAML configuration
- 🔒 **Secure Storage**: API tokens and sensitive data stored securely
- 📊 **Multiple Output Formats**: Table, CSV, JSON output options
- 🔍 **Search & Filter**: Find specific activities by type, source, or date
- 💾 **Data Export**: Export your timeline data for further analysis
- 🚀 **Fast & Lightweight**: Built with Go for performance

## Installation

### From Source

```bash
git clone https://github.com/autotime/autotime.git
cd autotime
make build
```

### Using Go Install

```bash
go install github.com/autotime/autotime@latest
```

### Pre-built Binaries

Download the latest release from [GitHub Releases](https://github.com/autotime/autotime/releases)

## Quick Start

1. **Run the demo to see AutoTime in action**:
   ```bash
   ./demo.sh
   ```

2. **Configure your first connector**:
   ```bash
   # Edit the configuration file
   autotime config edit

   # Enable a connector
   autotime connectors enable github
   ```

3. **View your timeline**:
   ```bash
   # Show today's activities
   autotime timeline

   # Show detailed timeline
   autotime timeline --details

   # Show timeline for specific date
   autotime timeline --date 2023-12-25
   ```

## Available Connectors

### GitHub Connector
Fetches your commits, issues, and pull request activities.

**Required Configuration:**
- `token`: GitHub Personal Access Token
- `username`: Your GitHub username
- `include_private`: Include private repositories (optional)

**Setup:**
1. Create a [Personal Access Token](https://github.com/settings/tokens)
2. Add it to your config file under `connectors.github.config.token`

### GitLab Connector
Fetches user activities from GitLab using Atom feeds.

**Required Configuration:**
- `username`: Your GitLab username
- `feed_token`: GitLab user feed token
- `gitlab_url`: GitLab instance URL (optional, defaults to https://gitlab.com)

**Setup:**
1. Go to your GitLab profile → Edit Profile → Access tokens
2. Copy your feed token from the "Feed token" section
3. Add it to your config file under `connectors.gitlab.config.feed_token`

### Google Calendar
Retrieves calendar events and meetings.

**Required Configuration:**
- `client_id`: OAuth Client ID
- `client_secret`: OAuth Client Secret
- `refresh_token`: OAuth Refresh Token
- `calendar_ids`: Calendar IDs to monitor (default: "primary")

## CLI Commands

### Timeline Commands

```bash
# Show today's timeline
autotime timeline

# Show timeline for specific date
autotime timeline --date 2023-12-25

# Show detailed information
autotime timeline --details

# Limit number of items
autotime timeline --max 20

# Export as CSV
autotime timeline --format csv

# Group activities by hour (default is chronological)
autotime timeline --group=true
```

### Connector Management

```bash
# List all available connectors
autotime connectors list

# Get information about a connector
autotime connectors info github

# Enable a connector
autotime connectors enable github

# Disable a connector
autotime connectors disable github

# Test connector connection
autotime connectors test github
```

### Configuration Management

```bash
# Show current configuration
autotime config show

# Edit configuration file
autotime config edit

# Validate configuration
autotime config validate

# Reset configuration to defaults
autotime config reset
```

## Configuration

AutoTime stores configuration in `~/.config/autotime/config.yaml`. You can edit this file directly or use `autotime config edit` to open it in your default editor.

### Example Configuration
See [the example config](example_config.yaml)

## Output Formats

### Table Format (Default)
```
Timeline for Monday, December 25, 2023
Found 15 activities

📅 09:00 (4 activities)
──────────────────────────────────────────────────────
  09:15 [github] Fixed authentication bug
  09:45 [calendar] Team standup meeting

📅 10:00 (3 activities)
──────────────────────────────────────────────────────
  10:00 [github] Updated README documentation
```

### CSV Format
```bash
autotime timeline --format csv
# timestamp,type,source,title,description,duration,url
# 2023-12-25 09:15:00,git_commit,github,Fixed authentication bug,,5m,https://github.com/...
# 2023-12-25 09:20:00,git_commit,gitlab,MR !42: Add new feature implementation,,15m,https://gitlab.com/...
```

## Development

### Adding Custom Connectors

Create a new connector by implementing the `Connector` interface:

```go
package connectors

import (
    "context"
    "time"
    "github.com/autotime/autotime/internal/timeline"
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
   - Enable connectors: `autotime connectors enable <name>`
   - Check configuration: `autotime config show`

2. **"Connection test failed"**
   - Verify API tokens and credentials
   - Test connection: `autotime connectors test <name>`

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

### Getting Help

```bash
# Command help
autotime --help
autotime timeline --help

# Connector information
autotime connectors info <name>

# Configuration validation
autotime config validate
```

## GitHub Actions & CI/CD

This project includes automated workflows for continuous integration and deployment:

### Test Workflow
- **Trigger**: Push to `main`/`develop` branches, pull requests
- **Actions**:
  - Runs tests on Go 1.21 and 1.22
  - Performs code linting with golangci-lint
  - Checks code formatting and vetting
  - Uploads test coverage to Codecov

### Release Workflow  
- **Trigger**: Git tags matching `v*` (e.g., `v1.0.0`)
- **Actions**:
  - Builds binaries for multiple platforms (Linux, macOS, Windows)
  - Creates release archives (.tar.gz for Unix, .zip for Windows)
  - Automatically generates changelog from git commits
  - Creates GitHub release with downloadable assets

### Creating a Release
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

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Quick Start for Contributors

```bash
git clone https://github.com/autotime/autotime.git
cd autotime
make setup    # Install development dependencies
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
