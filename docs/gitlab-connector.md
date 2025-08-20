# GitLab Connector

The GitLab connector fetches user activities from GitLab using Atom feeds. This allows you to track your GitLab activities including commits, merge requests, issues, comments, and other contributions in your daily timeline.

## Features

- Fetches user activities from GitLab.com or self-hosted GitLab instances
- Supports various activity types: commits, merge requests, issues, comments, wiki updates, and more
- Uses GitLab's built-in Atom feed with user feed tokens for authentication
- Filters activities by date to show only relevant daily activities
- Extracts project names and activity metadata for better organization

## Configuration

### 1. Enable the Connector

```bash
autotime connectors enable gitlab
```

### 2. Get Your GitLab Feed Token

1. Go to your GitLab profile (click your avatar → profile)
2. Click **Edit Profile**
3. In the left sidebar, click **Access tokens**
4. Scroll down to the **Feed token** section
5. Copy your feed token (it should look like `glft-...`)

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
      
      # Your GitLab feed token (required)
      feed_token: "glft-your-feed-token-here"
    
    refresh_interval: "15m"
```

**Note**: The connector uses the URL format `https://gitlab.com/username.atom?feed_token=TOKEN` to access your activity feed.

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `gitlab_url` | string | No | `https://gitlab.com` | GitLab instance URL |
| `username` | string | Yes | - | Your GitLab username |
| `feed_token` | string | Yes | - | Your GitLab feed token |

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
| **Commits** | Git commits pushed to repositories | `git_commit` |
| **Merge Requests** | Created, merged, or closed MRs | `jira` |
| **Issues** | Created or closed issues | `jira` |
| **Comments** | Comments on issues, MRs, commits | `custom` |
| **Projects** | Created new projects | `custom` |
| **Wiki** | Created or updated wiki pages | `custom` |
| **Milestones** | Created or closed milestones | `custom` |
| **General Activities** | Other GitLab activities | `custom` |

## For Self-Hosted GitLab

If you're using a self-hosted GitLab instance:

1. Set the `gitlab_url` to your instance URL:

```yaml
gitlab_url: "https://gitlab.example.com"
```

2. Make sure your GitLab instance allows external access to user feeds
3. Your feed token works the same way as GitLab.com
4. The feed URL will be: `https://gitlab.example.com/username.atom?feed_token=TOKEN`

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
- The exact URL being accessed
- HTTP response status and headers
- Raw feed content (first 500 characters)
- Number of entries found in the feed
- Date parsing and filtering details
- Activity conversion process

**Security Note**: Debug logs may contain sensitive information. Disable debug mode in production and don't share debug logs publicly.

## Troubleshooting

### Connection Test Fails

**Error**: `no feed token configured`
- **Solution**: Make sure you've set the `feed_token` in your configuration

**Error**: `no username configured`  
- **Solution**: Make sure you've set the `username` in your configuration

**Error**: `GitLab feed returned status 401`
- **Solution**: Your feed token may be invalid or expired. Generate a new one from your GitLab profile

**Error**: `GitLab feed returned status 404`
- **Solution**: Check that your username is correct and the GitLab URL is valid. The feed should be accessible at `https://gitlab.com/your-username.atom`
- **Test manually**: Try accessing `https://gitlab.com/your-username.atom?feed_token=your-token` in your browser
- **Common causes**: 
  - Username doesn't exist or is incorrect
  - User profile is set to private
  - GitLab instance URL is wrong
  - Feed token is malformed

### No Activities Found

If the connector isn't finding activities:

1. **Check the date range**: GitLab feeds typically show recent activities (last few weeks)
2. **Verify your username**: Make sure it matches your GitLab username exactly
3. **Check feed token**: Ensure your feed token is current and hasn't been reset
4. **Activity privacy**: Some activities might be private and not included in the feed
5. **Manual verification**: Test the feed URL directly in your browser:
   ```
   https://gitlab.com/your-username.atom?feed_token=your-token
   ```
   You should see XML content with your recent activities

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
   - Response body length (should be > 0)
   - Number of feed entries found
   - Date parsing issues
   - Activities filtered by date

### Common Debug Scenarios

**Feed loads but zero activities:**
- Check if entries exist: Look for "parsed feed with X entries"
- Check date filtering: Verify target date matches entry dates
- Check date formats: Look for "Failed to parse date" messages
- Check for empty dates: Some entries may have empty `published` dates but valid `updated` dates

**Feed returns 404:**
- Verify the URL format in debug output
- Test the exact URL shown in logs in your browser

**Feed returns 401:**
- Check if feed token is correctly included in URL
- Verify token hasn't expired

**XML parsing errors:**
- Debug mode saves failed responses to temp files
- Check if GitLab returned HTML login page instead of XML

**Date parsing issues:**
- The connector handles entries with empty `published` dates by falling back to `updated` dates
- Debug output shows which date source is used: "Using published date" vs "Using updated date"
- Entries with both empty `published` and `updated` dates are skipped

### Feed Token Security

**Important**: Your feed token can access your GitLab activity feed, which may include information about private projects and activities. Keep it secure:

- Don't share your feed token
- Don't commit it to version control
- Reset it if you suspect it's been compromised
- Use file permissions to protect your config file

To reset your feed token:
1. Go to GitLab Profile → Edit Profile → Access tokens
2. In the Feed token section, click **reset this token**
3. Update your AutoTime configuration with the new token

## Examples

### Basic Setup

```yaml
connectors:
  gitlab:
    enabled: true
    config:
      username: "johndoe"
      feed_token: "glft-abc123def456..."
```

### Self-Hosted GitLab

```yaml
connectors:
  gitlab:
    enabled: true
    config:
      gitlab_url: "https://git.company.com"
      username: "johndoe"
      feed_token: "glft-abc123def456..."
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
          → myproject/backend

🕙 10:45  [jira] Opened merge request #42: Add new API endpoints (gitlab)
          → myproject/api

🕐 13:20  [custom] Commented on issue #15: Database migration issues (gitlab)
          → myproject/infrastructure

🕒 15:15  [jira] Merged merge request #41: Update dependencies (gitlab)
          → myproject/frontend
```

## Limitations

- Activities are limited to what GitLab includes in user activity feeds
- Feed data is typically limited to recent activities (a few weeks)
- Some private activities may not be included depending on GitLab settings
- Real-time updates depend on GitLab's feed refresh intervals
- XML parsing relies on GitLab's Atom feed format stability
- Some GitLab feed entries may have empty published dates (handled automatically by using updated dates)

## Support

If you encounter issues with the GitLab connector:

1. Test your connection: `autotime connectors test gitlab`
2. Check your configuration: `autotime config show`
3. Verify your GitLab feed token is valid
4. Check that your GitLab instance is accessible