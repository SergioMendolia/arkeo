# Arkeo - Daily Activity Timeline Builder

Arkeo is a command-line tool and web application that connects to various services to automatically gather information about your daily activities and presents them in a chronological timeline. Instead of manually logging your activities or monitoring your computer actively, Arkeo collects data from the connected services after they have happened so you don't need to worry about having an application running in the background.

The tool is designed to answer the question "What the hell did I do on that day?" when you need to recall your daily activities for timesheeting.

## Features

- **Web UI**: Interactive dark-themed web interface for browsing timelines, managing connectors, and configuring browser domain exclusions
- **Multiple Connectors**: GitHub, GitLab, Google Calendar, YouTrack, macOS system events, browser history (Chrome/Chromium/Firefox), and custom webhooks
- **Daily Timeline**: View all your activities in chronological order, one activity per line
- **Output Formats**: Table (default, with colors) and JSON (metadata-free)
- **Date Ranges**: Single day, work week (Mon-Fri), or arbitrary date ranges (e.g. last 6 months)
- **Activity Caching**: Past days are cached in a local SQLite database for instant re-display
- **Browser Domain Manager**: Interactive TUI or web UI to browse visited domains and manage exclusions
- **Easy Configuration**: Manage connectors through YAML configuration or the web UI

## Installation

### From Source

```bash
git clone https://github.com/sergiomendolia/arkeo.git
cd arkeo
go build -o arkeo .
```

### Using Go Install

```bash
go install github.com/sergiomendolia/arkeo@latest
```

### Pre-built Binaries

Download the latest release from [GitHub Releases](https://github.com/sergiomendolia/arkeo/releases)

## Quick Start

1. **Launch the web UI** (default when running `arkeo` with no arguments):
   ```bash
   arkeo
   # Opens http://localhost:7878 in your browser automatically
   ```

2. **Configure connectors** — either in the web UI under Connectors, or via CLI:
   ```bash
   # Edit ~/.config/arkeo/config.yaml to add API tokens
   # Then enable a connector
   arkeo connectors enable github
   ```

3. **View your timeline** — in the web UI or via CLI:
   ```bash
   # Show yesterday's activities (default)
   arkeo timeline

   # Show timeline for a specific date
   arkeo timeline 2024-01-15

   # Show activities for the entire work week (Monday-Friday)
   arkeo timeline --week

   # Fetch the last 6 months of history
   arkeo timeline --range 180

   # Output in JSON format
   arkeo timeline --format json

   # Limit number of activities shown
   arkeo timeline --max-items 100
   ```

4. **Manage browser domain exclusions**:
   ```bash
   # Interactive TUI to browse domains and toggle exclusions
   arkeo browser domains

   # Plain table output (for scripting)
   arkeo browser domains --no-tui

   # Scan a specific time range
   arkeo browser domains --days 30
   ```

## Web UI

Running `arkeo` with no arguments launches a local web application at `http://localhost:7878`. The browser opens automatically.

### Pages

- **Timeline** (`/`) — Browse activities by date with prev/next day navigation. The URL is bookmarkable: `/?date=2024-01-15&format=table`. Supports table and JSON views. Cached days load instantly.
- **Connectors** (`/connectors`) — Enable, disable, test, and configure connectors. Each connector has an inline settings panel for editing API tokens, URLs, and other config fields. Secret fields (tokens) are masked.
- **Browser** (`/browser`) — Scan browser history, view domain visit counts, and toggle domain exclusions with switch toggles. Save exclusions to config.

### Web UI Flags

| Flag | Description |
|------|-------------|
| `--addr` | Address to listen on (default: `localhost:7878`) |

## Available Connectors

### GitHub Connector
Fetches your commits, issues, and pull request activities from GitHub.

### GitLab Connector
Fetches user activities from GitLab (push events, new branches, branch deletions, merge requests, issues, comments).

### Google Calendar
Retrieves calendar events and meetings via iCal URLs.

### YouTrack Connector
Fetches activities and issue updates from YouTrack. Issue summaries are included in activity titles (e.g. `Updated State to Review in ZBR-7696: Infomaniak outage...`).

### macOS System Events Connector
Fetches screen lock/unlock events on macOS systems using system logs. Only works on macOS.

### Browser History Connector
Fetches browsing history from Chrome/Chromium and Firefox. Visits to the same domain within a configurable time window are grouped into a single activity. Subdomains are normalized (e.g. `docs.github.com` → `github.com`).

Use `arkeo browser domains` or the web UI's Browser page to interactively manage which domains to exclude from the timeline.

### Webhooks Connector
Fetches activities from custom HTTP webhook endpoints. Each webhook is called with `GET {url}?date=YYYY-MM-DD` and should return a JSON array of activities.

## Output Formats

- **table** (default): Human-readable format with colors, time gaps, and one activity per line. Each line shows: `HH:MM  SRC  Title — Description`. Long lines are truncated with `…`.
- **json**: Machine-readable JSON format. Metadata is excluded from the output.

## Caching

Arkeo caches fetched activities in a local SQLite database at `~/.config/arkeo/cache.db`. Once a day has been fetched from connectors, subsequent runs load instantly from cache — even if some connectors returned zero activities for that day.

```bash
# Normal run (uses cache for past days)
arkeo timeline --range 180

# Force re-fetch by clearing cache for the date range
arkeo timeline --range 180 --reset-cache

# Skip cache entirely (always fetch, don't store)
arkeo timeline --no-cache
```

## Configuration

Arkeo stores configuration in `~/.config/arkeo/config.yaml` (XDG_CONFIG_HOME is respected). Edit this file directly with your preferred editor, or use the web UI's Connectors page to edit connector settings interactively.

### Example Configuration

See [config.example.yaml](config.example.yaml) for a complete configuration example with all connectors.

## Commands

```
arkeo                             # Launch the web UI (default)
arkeo web                         # Launch the web UI (explicit)
arkeo web --addr :8080            # Launch web UI on a custom port
arkeo timeline [date]             # Show activity timeline for a date
arkeo connectors list              # List all available connectors
arkeo connectors enable <name>   # Enable a connector
arkeo connectors disable <name>  # Disable a connector
arkeo connectors info <name>     # Show connector info and config
arkeo connectors test <name>     # Test a connector's connection
arkeo browser domains             # Interactive domain manager (TUI)
```

### Timeline Flags

| Flag | Description |
|------|-------------|
| `--format` | Output format: `table` (default) or `json` |
| `--week` | Show the work week (Mon-Fri) containing the selected date |
| `--range N` | Fetch the last N days ending at the selected date |
| `--max-items N` | Maximum activities to display (0 = unlimited) |
| `--reset-cache` | Clear cached activities for the selected date range |
| `--no-cache` | Skip cache (always fetch from connectors) |

### Browser Domains Flags

| Flag | Description |
|------|-------------|
| `--days N` | Number of days of history to scan (default: 90) |
| `--browser` | Browser to scan: `chrome`, `firefox`, or `all` (default: `all`) |
| `--no-tui` | Output plain table without interactive TUI |

## Development

### Building

```bash
# Build for current platform
go build -o arkeo .

# Build all packages (must work with CGO disabled)
CGO_ENABLED=0 go build ./...

# Run tests
go test ./...

# Run vet + tests (matches CI)
go vet ./... && go test -race ./...
```

### Adding Custom Connectors

Create a new connector by implementing the `Connector` interface in `internal/connectors/`, then register it in `cmd/root.go`:

```go
availableConnectors := []connectors.Connector{
    // ...
    connectors.NewMyConnector(),
}
```

Also add the default config to `internal/config/config.go` in `DefaultConfig()` and `GenerateExampleConfigYAML()`.

## Troubleshooting

### Common Issues

1. **"No connectors are enabled"**
   - Enable connectors: `arkeo connectors enable <name>`
   - Or use the web UI at `/connectors`

2. **"Connection test failed"**
   - Verify API tokens and credentials in `~/.config/arkeo/config.yaml`
   - Test connection: `arkeo connectors test <name>`
   - Or use the web UI's Test button on the Connectors page

3. **"No activities found"**
   - Check that the date has activity
   - Verify connector is properly configured
   - Try `--reset-cache` to force re-fetch

### Debug Mode

Enable debug logging by setting `log_level: "debug"` in config or `ARKEO_DEBUG=1` environment variable.

## Creating a Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions will automatically run tests, build cross-platform binaries (CGO_ENABLED=0), and publish a GitHub release.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.