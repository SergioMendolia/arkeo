# AutoTime - Daily Activity Timeline Builder

AutoTime is a command-line tool that connects to various services to automatically gather information about your daily activities and presents them in a chronological timeline.

## Features

- 🔗 **Multiple Connectors**: Connect to GitHub, GitLab, Google Calendar, YouTrack and more
- 📅 **Daily Timeline**: View all your activities in chronological order
- 🤖 **AI Analysis**: Send your timeline to OpenAI-compatible LLMs for productivity insights
- ⚙️ **Easy Configuration**: Manage connectors through YAML configuration
- 🔒 **Secure Storage**: API tokens and sensitive data stored locally

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

1. **Configure your first connector**:
   ```bash
   # Edit the configuration file
   autotime config edit

   # Enable a connector
   autotime connectors enable github
   ```

2. **View your timeline**:
   ```bash
   # Show today's activities
   autotime timeline

   # Show detailed timeline
   autotime timeline --details

   # Show timeline for specific date
   autotime timeline --date 2023-12-25
   ```

3. **Analyze your timeline with AI**:
   ```bash
   # Analyze today's timeline
   autotime analyze
   # Analyze specific date
   autotime analyze --date 2023-12-25
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

## AI Timeline Analysis

AutoTime can send your daily timeline to OpenAI-compatible language models to interpret a list of timesheet. This feature helps you:

### Supported LLM Services

- **OpenAI** (GPT-3.5, GPT-4, GPT-4-turbo)
- **Local models** via Ollama (llama2, mistral, etc.)
- Any **OpenAI-compatible API**

### Configuration

Configure the LLM settings in your config file:

```yaml
llm:
  # Base URL for OpenAI-compatible API
  base_url: "https://api.openai.com/v1"

  # API key for authentication
  api_key: "your-api-key-here"

  # Model name to use
  model: "gpt-3.5-turbo"

  # Maximum tokens in response
  max_tokens: 1000

  # Temperature for response creativity (0.0-2.0)
  temperature: 0.7

  # Default analysis prompt
  default_prompt: "Please analyze this daily timeline..."

  # Skip TLS certificate verification (for local development or self-signed certs)
  # WARNING: Only enable for trusted local environments
  skip_tls_verify: false
```

### LLM Commands

```bash
# Test LLM connection
autotime llm test

# Show LLM configuration
autotime llm info

# Analyze timeline
autotime analyze
```

## Configuration

AutoTime stores configuration in `~/.config/autotime/config.yaml`. You can edit this file directly or use `autotime config edit` to open it in your default editor.

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

4. **"LLM API key not configured"**
   - Set API key: `autotime config edit`
   - Test connection: `autotime llm test`
   - Verify model name is correct

5. **"Connection test failed" (LLM)**
   - Check API key is valid
   - Verify base URL is correct
   - Ensure model name exists
   - Check network connectivity
   - For self-signed certificates, set `skip_tls_verify: true` in config

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
