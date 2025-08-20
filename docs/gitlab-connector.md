# GitLab Connector

The GitLab connector fetches user activities from GitLab using the Events API. This allows you to track your GitLab activities including commits, merge requests, issues, comments, and other contributions in your daily timeline.

## Features

- Fetches user activities from GitLab.com or self-hosted GitLab instances
- Supports various activity types: commits (all branches), merge requests, issues, comments, project creation, and more
- Uses GitLab's Events API with personal access tokens for authentication
- Filters activities by date to show only relevant daily activities
- Extracts project names and activity metadata for better organization
- Handles pagination to capture all activities for the target date

## Configuration

### 1. Enable the Connector

```bash
autotime connectors enable gitlab
```

### 2. Get Your GitLab Personal Access Token

1. Go to your GitLab profile (click your avatar → **Edit Profile**)
2. In the left sidebar, click **Access tokens**
3. Click **Add new token**
4. Fill in the token details:
   - **Token name**: `autotime` (or any descriptive name)
   - **Scopes**: Select `read_api` (this gives read access to the API)
   - **Expiration date**: Set an appropriate expiration date
5. Click **Create personal access token**
6. Copy the token (it should look like `glpat-...`) - you won't be able to see it again

### 3. Configure the Connector

Edit your configuration file:

```bash
autotime config edit
```

Add your GitLab configuration:

```yaml
connectors:
  gitlab:
    enabled: true
    config:
      # GitLab instance URL (optional, defaults to https://gitlab.com)
      gitlab_url: "https://gitlab.com"
      
      # Your GitLab username (required)
      username: "your-username"
      
      # Your GitLab personal access token (required)
      access_token: "glpat-your-access-token-here"
    
    refresh_interval: "15m"
```

**Note**: The connector uses the GitLab Events API endpoint `https://gitlab.com/api/v4/events` to access your activity events.

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `gitlab_url` | string | No | `https://gitlab.com` | GitLab instance URL |
| `username` | string | Yes | - | Your GitLab username |
| `access_token` | string | Yes | - | Your GitLab personal access token |

## Usage

### Test Connection

Verify your configuration works:

```bash
autotime connectors test gitlab
```

### View Activities

Show your GitLab activities for today:

```bash
autotime timeline
```

Show activities for a specific date:

```bash
autotime timeline --date 2024-01-15
```

Filter to show only GitLab activities:

```bash
autotime timeline --source gitlab
```

## Supported Activity Types

The GitLab connector recognizes and categorizes these activities:

| Activity Type | Description | Timeline Type |
|---------------|-------------|---------------|
| **Push Events** | Git commits pushed to any branch | `git_commit` |
| **Merge Requests** | Opened, merged, or closed MRs | `jira` |
| **Issues** | Opened or closed issues | `jira` |
| **Comments** | Comments on issues, MRs, commits | `custom` |
| **Projects** | Created new projects | `custom` |
| **General Activities** | Other GitLab activities | `custom` |

## For Self-Hosted GitLab

If you're using a self-hosted GitLab instance:

1. Set the `gitlab_url` to your instance URL:

```yaml
gitlab_url: "https://gitlab.example.com"
```

2. Make sure your GitLab instance allows API access
3. Your personal access token works the same way as GitLab.com
4. The Events API endpoint will be: `https://gitlab.example.com/api/v4/events`

## Debug Mode

If you're having issues with the GitLab connector, enable debug mode using environment variables:

```bash
# Enable debug mode for GitLab connector
export AUTOTIME_DEBUG=1
./autotime timeline --source gitlab

# Or using LOG_LEVEL
export LOG_LEVEL=debug
./autotime timeline --source gitlab
```

Debug mode will show:
- The exact API URLs being accessed
- HTTP response status and headers
- Number of events found on each page
- Date parsing and filtering details
- Activity conversion process
- Pagination details

**Security Note**: Debug logs may contain sensitive information. Disable debug mode in production and don't share debug logs publicly.

## Troubleshooting

### Connection Test Fails

**Error**: `no access token configured`
- **Solution**: Make sure you've set the `access_token` in your configuration

**Error**: `gitlab username is required`  
- **Solution**: Make sure you've set the `username` in your configuration

**Error**: `GitLab API returned status 401`
- **Solution**: Your access token may be invalid or expired. Generate a new one from your GitLab profile
- **Check scope**: Ensure your token has the `read_api` scope

**Error**: `GitLab API returned status 403`
- **Solution**: Your token doesn't have sufficient permissions. Make sure it has the `read_api` scope

**Error**: `GitLab API returned status 404`
- **Solution**: Check that the GitLab URL is correct and accessible

### No Activities Found

If the connector isn't finding activities:

1. **Check the date range**: The Events API shows recent activities (typically last few weeks)
2. **Verify your username**: Make sure it matches your GitLab username exactly
3. **Check access token**: Ensure your token is current and has the right scope
4. **Activity privacy**: Some activities might be private and not included in your events
5. **Manual verification**: Test the API directly using curl:
   ```bash
   curl -H "Authorization: Bearer your-token" \
        "https://gitlab.com/api/v4/events?per_page=10"
   ```
   You should see JSON data with your recent activities

### Debug Mode Troubleshooting

1. **Enable debug logging** using environment variables:
   ```bash
   export AUTOTIME_DEBUG=1
   # or
   export LOG_LEVEL=debug
   ```
2. **Run the connector** and check the debug output:
   ```bash
   ./autotime connectors test gitlab
   # or
   ./autotime timeline --source gitlab --date 2024-01-15
   ```
3. **Check debug output** for:
   - HTTP status codes (should be 200)
   - Number of events found per page
   - Date parsing issues
   - Activities filtered by date
   - Pagination behavior

### Common Debug Scenarios

**API loads but zero activities:**
- Check if events exist: Look for "found X events" messages
- Check date filtering: Verify target date matches event dates
- Check for date parsing errors: Look for "Failed to parse event time" messages

**API returns 401:**
- Check if access token is correctly configured
- Verify token hasn't expired
- Ensure token has `read_api` scope

**API returns 403:**
- Token exists but lacks proper permissions
- Generate a new token with `read_api` scope

**Date parsing issues:**
- The connector handles events with invalid date formats gracefully
- Debug output shows which events are skipped due to date parsing errors

### Access Token Security

**Important**: Your personal access token can access your GitLab data according to its scopes. Keep it secure:

- Don't share your access token
- Don't commit it to version control
- Use file permissions to protect your config file
- Revoke and regenerate if compromised
- Set appropriate expiration dates

To revoke/regenerate your access token:
1. Go to GitLab Profile → Edit Profile → Access tokens
2. Find your token and click **Revoke**
3. Create a new token with the same scopes
4. Update your AutoTime configuration with the new token

## Examples

### Basic Setup

```yaml
connectors:
  gitlab:
    enabled: true
    config:
      username: "johndoe"
      access_token: "glpat-abc123def456..."
```

### Self-Hosted GitLab

```yaml
connectors:
  gitlab:
    enabled: true
    config:
      gitlab_url: "https://git.company.com"
      username: "johndoe"
      access_token: "glpat-abc123def456..."
```

### Debug Mode Example

```bash
# Enable debug mode with environment variables
export AUTOTIME_DEBUG=1

# Or use log level
export LOG_LEVEL=debug

# Then run your commands
./autotime timeline --source gitlab --date 2024-01-15
```

### Sample Timeline Output

```
📅 January 15, 2024

🕘 09:30  [git_commit] Fix authentication bug in user service (gitlab)
          → myproject/backend • main

🕙 10:45  [jira] Opened merge request: Add new API endpoints (gitlab)
          → myproject/api • !42

🕐 13:20  [custom] Commented on issue: Database migration issues (gitlab)
          → myproject/infrastructure • #15

🕒 15:15  [jira] Merged merge request: Update dependencies (gitlab)
          → myproject/frontend • !41

🕘 16:00  [git_commit] Pushed 3 commits to feature-branch (gitlab)
          → myproject/backend • feature-branch
```

## API Rate Limits

GitLab has API rate limits that may affect the connector:

- **GitLab.com**: 2,000 requests per minute per user
- **Self-hosted**: Varies by instance configuration

The connector is designed to be efficient:
- Uses pagination to minimize requests
- Stops fetching when it reaches events older than the target date
- Implements a maximum page limit to prevent infinite loops

If you encounter rate limit errors, the connector will show appropriate error messages in debug mode.

## Limitations

- Activities are limited to what GitLab includes in the Events API
- Event data is typically limited to recent activities (a few weeks)
- Some private activities may not be included depending on GitLab settings
- Real-time updates depend on GitLab's event processing
- The connector fetches up to 10 pages of events to prevent excessive API usage
- Events are processed in reverse chronological order (newest first)

## Support

If you encounter issues with the GitLab connector:

1. Test your connection: `autotime connectors test gitlab`
2. Check your configuration: `autotime config show`
3. Verify your GitLab personal access token is valid and has `read_api` scope
4. Check that your GitLab instance is accessible
5. Enable debug mode for detailed troubleshooting information