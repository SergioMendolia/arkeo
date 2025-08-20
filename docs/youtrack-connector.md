# YouTrack Connector

The YouTrack connector fetches user activities from YouTrack using the REST API. This allows you to track your YouTrack activities including issue updates, comments, work items (time tracking), and other contributions in your daily timeline.

## Features

- Fetches user activities from YouTrack Cloud or self-hosted YouTrack instances
- Supports various activity types: issue field changes, comments, work items, and link updates
- **Displays actual field values** when changes are made (e.g., "Updated State to In Progress", "Changed Priority from Normal to High")
- Uses YouTrack's permanent tokens for secure authentication
- Filters activities by date and user to show only relevant daily activities
- Extracts issue and project information for better organization
- Uses human-readable issue keys (PROJ-123) instead of internal IDs for better readability
- Properly handles issue keys for comment activities by extracting issue information from the comment's associated issue
- Configurable activity categories to include only what you need

## Configuration

### 1. Enable the Connector

```bash
autotime connectors enable youtrack
```

### 2. Get Your YouTrack Permanent Token

1. Go to your YouTrack instance and click your avatar → **Profile**
2. Go to the **Account Security** tab
3. Click **New token**
4. Fill in the token details:
   - **Name**: `autotime` (or any descriptive name)
   - **Scope**: Select **YouTrack** (and **YouTrack Administration** if needed)
   - Remove any other services from the scope
5. Click **Create**
6. Click **Copy token** and save it securely - you won't be able to see it again
7. Also copy your YouTrack base URL from the browser (e.g., `https://mycompany.youtrack.cloud/`)

### 3. Configure the Connector

Edit your configuration file:

```bash
autotime config edit
```

Add your YouTrack configuration:

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      # YouTrack instance URL (required)
      base_url: "https://mycompany.youtrack.cloud/"
      
      # Your YouTrack permanent token (required)
      token: "perm:your-youtrack-token-here"
      
      # Username to filter activities for (optional, defaults to token owner)
      username: "your-username"
      
      # Include work items (time tracking entries)
      include_work_items: true
      
      # Include comment activities
      include_comments: true
      
      # Include issue field changes
      include_issues: true
```

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `base_url` | string | Yes | - | YouTrack instance URL |
| `token` | string | Yes | - | YouTrack permanent token |
| `username` | string | No | token owner | Username to filter activities for |
| `include_work_items` | boolean | No | `true` | Include work items (time tracking) |
| `include_comments` | boolean | No | `true` | Include comment activities |
| `include_issues` | boolean | No | `true` | Include issue field changes |

## Usage

### Test Connection

Verify your configuration works:

```bash
autotime connectors test youtrack
```

### View Activities

Show your YouTrack activities for today:

```bash
autotime timeline
```

Show activities for a specific date:

```bash
autotime timeline --date 2024-01-15
```

Filter to show only YouTrack activities:

```bash
autotime timeline --source youtrack
```

## Supported Activity Types

The YouTrack connector recognizes and categorizes these activities:

| Activity Category | Description | Timeline Type |
|-------------------|-------------|---------------|
| **Custom Field Changes** | Updates to issue fields (status, assignee, etc.) | `youtrack` |
| **Comments** | Comments added to issues | `youtrack` |
| **Work Items** | Time tracking entries | `youtrack` |
| **Link Changes** | Issue link modifications | `youtrack` |
| **General Activities** | Other YouTrack activities | `youtrack` |

## For Self-Hosted YouTrack

If you're using a self-hosted YouTrack instance:

1. Set the `base_url` to your instance URL:

```yaml
base_url: "https://youtrack.example.com/"
```

2. Make sure your YouTrack instance allows API access
3. Your permanent token works the same way as YouTrack Cloud
4. The API endpoint will be: `https://youtrack.example.com/api/activities`

## Authentication

YouTrack uses permanent tokens for API authentication. These tokens:

- Provide secure access without complex OAuth flows
- Can be easily managed in your user profile
- Have the same permissions as your user account
- Can be revoked and regenerated if compromised

### Token Scopes

Your permanent token should have the **YouTrack** scope at minimum. For full functionality, you may also need:

- **YouTrack Administration** - for some administrative API endpoints
- Ensure no other unnecessary scopes are selected

## Debug Mode

If you're having issues with the YouTrack connector, enable debug mode using one of these methods:

### Method 1: Configuration File

Add debug logging to your YouTrack connector configuration:

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://mycompany.youtrack.cloud/"
      token: "perm:your-token-here"
      log_level: "debug"  # Enable debug logging
```

### Method 2: Environment Variables

You can enable debug mode using environment variables without modifying your configuration:

```bash
# Option 1: Using AUTOTIME_DEBUG
export AUTOTIME_DEBUG=1
./autotime timeline --source youtrack

# Option 2: Using LOG_LEVEL
export LOG_LEVEL=debug
./autotime timeline --source youtrack

# Option 3: Using AUTOTIME_DEBUG with true
export AUTOTIME_DEBUG=true
./autotime timeline --source youtrack
```

**Priority Order**: Configuration file settings override environment variables. If you have `log_level: "info"` in your config, it will take precedence over `AUTOTIME_DEBUG=1`.

### Method 3: Global Debug Mode

Set debug logging globally in your configuration:

```yaml
app:
  log_level: "debug"  # Enables debug for all connectors
```

### Debug Output

Debug mode will show detailed information including:
- Connection test results and API endpoints
- User authentication and username resolution
- API request URLs and parameters
- HTTP response status codes and headers
- Number of activities found and parsed
- Activity category filtering
- Individual activity conversion details (first 5 activities)
- Activity parsing and timestamp conversion
- Total activities before and after conversion

### Sample Debug Output

```
YouTrack Debug: Testing connection to https://mycompany.youtrack.cloud/
YouTrack Debug: Response status: 200 OK
YouTrack Debug: Successfully connected to YouTrack
YouTrack Debug: Fetching activities for date 2024-01-15
YouTrack Debug: Using username: johndoe
YouTrack Debug: Fetching activities for user johndoe from 1705276800000 to 1705363200000
YouTrack Debug: Including categories: CustomFieldCategory, CommentsCategory, WorkItemCategory
YouTrack Debug: Making API request to https://mycompany.youtrack.cloud/api/activities?...
YouTrack Debug: Successfully received API response
YouTrack Debug: Parsed 12 activities from API response
YouTrack Debug: Converting activity 1: ID=activity-123, Category=CustomFieldCategory
YouTrack Debug: Created activity: activity-123 -> Updated State in PROJ-456
YouTrack Debug: Converted 12 activities for timeline
YouTrack Debug: Total activities found: 12
```

**Note**: Activities now display human-readable issue keys (like "PROJ-456") instead of internal IDs for better readability. This includes comment activities, which now properly extract the issue key from the associated issue.

**Security Note**: Debug logs may contain sensitive information like API URLs and activity details. Disable debug mode in production and don't share debug logs publicly.

## Troubleshooting

### Connection Test Fails

**Error**: `base_url and token must be configured`
- **Solution**: Make sure you've set both `base_url` and `token` in your configuration

**Error**: `youtrack base_url is required`
- **Solution**: Ensure you've specified the base URL of your YouTrack instance

**Error**: `youtrack base_url must start with http:// or https://`
- **Solution**: Your base URL must include the protocol (https:// recommended)

**Error**: `invalid YouTrack token`
- **Solution**: Your permanent token may be invalid or expired. Generate a new one from your YouTrack profile

**Error**: `youtrack API returned status 403`
- **Solution**: Your token doesn't have sufficient permissions. Check the token scope includes YouTrack

**Error**: `youtrack API bad request (400)`
- **Solution**: The API request parameters are invalid. Common causes:
  - **No categories specified**: YouTrack requires at least one activity category. Enable at least one of: `include_work_items`, `include_comments`, or `include_issues`
  - **Invalid username**: Check if the username exists in YouTrack
  - **Unsupported field names**: Your YouTrack version may not support some API fields
  - **Invalid activity categories**: Check if the categories are valid for your YouTrack version
  - **Invalid date range**: Ensure the date is valid and not too far in the past
  - **Invalid URL format**: Check your base_url configuration

**Error**: `No requested categories specified as a filter parameter`
- **Solution**: This specific error means all activity categories are disabled. You must enable at least one:
  ```yaml
  include_work_items: true   # OR
  include_comments: true     # OR  
  include_issues: true       # OR any combination
  ```

### No Activities Found

If the connector isn't finding activities:

1. **Check the date range**: Activities are filtered to the specific date you're querying
2. **Verify your username**: Make sure it matches your YouTrack username exactly
3. **Check token permissions**: Ensure your token has the correct scopes
4. **Activity categories**: Check if the activity categories you want are enabled in the config
5. **Enable debug mode**: Add `log_level: "debug"` to your YouTrack configuration
6. **Manual verification**: Test the API directly using curl:
   ```bash
   # Test user authentication
   curl -H "Authorization: Bearer your-token" \
        "https://mycompany.youtrack.cloud/api/admin/users/me"
   
   # Test activities API
   curl -H "Authorization: Bearer your-token" \
        "https://mycompany.youtrack.cloud/api/activities?per_page=1"
   ```

### Token Issues

**Token not working after creation:**
- Wait a few minutes - new tokens may take time to propagate
- Verify you copied the complete token string
- Check that the token scope includes YouTrack

**Token expired:**
- Generate a new permanent token from your profile
- Update your AutoTime configuration with the new token
- Consider setting longer expiration dates for future tokens

### Debug Mode Troubleshooting

1. **Enable debug logging** in your configuration:
   ```yaml
   connectors:
     youtrack:
       config:
         log_level: "debug"
   ```
   Or use environment variables:
   ```bash
   export LOG_LEVEL=debug
   ```

2. **Run the connector** and check debug output:
   ```bash
   ./autotime connectors test youtrack
   # or
   ./autotime timeline --source youtrack --date 2024-01-15
   ```
   
   Or using environment variables:
   ```bash
   # Using AUTOTIME_DEBUG
   export AUTOTIME_DEBUG=1
   ./autotime connectors test youtrack
   
   # Using LOG_LEVEL
   export LOG_LEVEL=debug
   ./autotime timeline --source youtrack --date 2024-01-15
   ```

3. **Check debug output** for:
   - Connection test results (should show "Successfully connected")
   - API request URLs and parameters
   - HTTP status codes (should be 200)
   - Number of activities found and parsed
   - Activity conversion details
   - Category filtering information

### Common Debug Scenarios

**API connects but zero activities:**
- Check debug output for "Parsed X activities from API response"
- Verify date filtering: Look for timestamp ranges in milliseconds
- Check category filtering: Ensure desired categories are included
- Look for "Converted X activities for timeline" vs parsed count

**Authentication issues:**
- Debug shows "Response status: 401" - token is invalid
- Debug shows "Response status: 403" - token lacks permissions
- Check "Successfully connected to YouTrack" message

**Activity parsing issues:**
- Look for "Failed to decode JSON response" errors
- Check for "Skipped activity" messages with reasons
- Verify timestamp conversion in debug logs

**Field value extraction issues:**
If you see generic type names instead of actual values (e.g., "Updated Assignee to User" instead of "Updated Assignee to John Doe"):
- Enable debug mode to see field extraction details:
  ```yaml
  connectors:
    youtrack:
      config:
        log_level: "debug"
  ```
- Look for debug messages like:
  - "Extracting field value from: ..."
  - "Processing object with keys: ..."
  - "Could not extract value from object: ..."
- Common causes:
  - YouTrack API not returning expected field names
  - Missing field specifications in API request
  - Older YouTrack versions with different object structures
- Solutions:
  - Ensure your YouTrack version supports the required API fields
  - Try enabling debug mode and check what field structures are actually returned
  - For custom YouTrack configurations, you may need to adjust the `api_fields` setting

**400 Bad Request errors:**
- Check debug output for "Request parameters" details
- **"No requested categories specified"**: Enable at least one activity category:
  ```yaml
  include_work_items: true  # Enable at least one
  include_comments: true    # of these options
  include_issues: true
  ```
- Verify username exists in YouTrack system
- Check if your YouTrack version supports the API fields being requested
- Ensure activity categories are valid for your YouTrack version
- Look for "Response body" in debug output for specific error details
- Common fixes:
  - **Enable activity categories**: Make sure at least one category is enabled
  - Update YouTrack to a newer version
  - Check spelling of username in configuration
  - Try disabling specific activity categories to isolate the issue

## YouTrack Version Compatibility

The YouTrack connector is designed to work with different versions of YouTrack, but some features may vary:

### Supported Versions

- **YouTrack Cloud**: All current versions (fully supported)
- **YouTrack Server 2022.3+**: Full feature support
- **YouTrack Server 2020.1-2022.2**: Basic support (some fields may not be available)
- **YouTrack Server < 2020.1**: Limited support (REST API v2 required)

### Common Compatibility Issues

**Field not supported errors:**
- Some API fields may not exist in older YouTrack versions
- Solution: Use the `api_fields` configuration option with a reduced field set

**Category not supported errors:**
- Older versions may not support all activity categories
- Solution: Disable specific category types in configuration

### Version-Specific Configuration

#### For YouTrack Server 2020.1-2021.3

Use reduced API fields for better compatibility:

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://youtrack.company.com/"
      token: "perm:your-token-here"
      api_fields: "id,timestamp,author(login,name),category(id,name),target(id,summary)"
      include_work_items: false  # May not be fully supported
      include_comments: false    # Comment issue keys may not be supported
```

**Note**: If your YouTrack version doesn't support `idReadable` or comment issue references, the connector will automatically fall back to using internal IDs. You can use this reduced field set for maximum compatibility:

```yaml
api_fields: "id,timestamp,author(login,name),category(id,name),target(id,summary)"
include_comments: false  # Disable comments if issue references aren't supported
```

#### For YouTrack Server 2022.1+

Full configuration with all features:

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://youtrack.company.com/"
      token: "perm:your-token-here"
      # Use default api_fields (no need to specify)
      include_work_items: true
      include_comments: true
      include_issues: true
```

### Checking Your YouTrack Version

To check your YouTrack version:
1. Log into your YouTrack instance
2. Click the gear icon (Settings) in the top-right
3. Look for "About" or check the bottom of any page
4. Note the version number (e.g., "2023.1.15")

### Troubleshooting Version Issues

If you encounter version-related errors:

1. **Enable debug mode** to see the exact error:
   ```yaml
   log_level: "debug"
   ```

2. **Try minimal API fields** for older versions:
   ```yaml
   api_fields: "id,timestamp,author(login),category(id),target(id)"
   include_comments: false  # Disable if comment issue references cause issues
   ```

3. **Disable advanced features** if needed:
   ```yaml
   include_work_items: false
   include_comments: false
   include_issues: true  # Keep basic issue tracking
   ```

4. **Check API documentation** for your specific YouTrack version at:
   `https://your-youtrack-instance/help/api/`

## Activity Categories

YouTrack activities are organized into categories. You can control which categories to include:

**Important**: YouTrack requires at least one activity category to be enabled. If you disable all categories, the connector will automatically enable `CustomFieldCategory` as a fallback.

### CustomFieldCategory
- Field value changes (status, assignee, priority, etc.) with new values displayed
- Custom field updates showing before/after values
- State transitions with clear value changes
- Examples:
  - "Updated State to In Progress in PROJ-123"
  - "Changed Priority from Normal to High"
  - "Set Assignee to john.doe"

### CommentsCategory
- Comments added to issues
- Comment edits and updates
- @mentions in comments

### WorkItemCategory
- Time tracking entries
- Work item additions and modifications
- Time spent logging

### LinkCategory
- Issue link creation and removal
- Dependency updates
- Related issue connections

## Examples

### Basic Setup

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://mycompany.youtrack.cloud/"
      token: "perm:abc123def456..."
```

### Minimal Activities (Only Time Tracking)

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://mycompany.youtrack.cloud/"
      token: "perm:abc123def456..."
      include_work_items: true   # At least one category must be enabled
      include_comments: false
      include_issues: false
```

### Self-Hosted YouTrack

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://youtrack.company.com/"
      token: "perm:abc123def456..."
      username: "johndoe"
```

### Sample Timeline Output

Field changes now display the actual values that were changed:

```
2024-01-15 14:30:22 - Updated State to In Progress in MYPROJECT-456
2024-01-15 14:28:15 - Changed Assignee from alice.smith to bob.jones
2024-01-15 14:25:30 - Set Priority to High in MYPROJECT-455
2024-01-15 14:20:10 - Commented on MYPROJECT-454
2024-01-15 13:45:05 - Logged work on MYPROJECT-453
```

### Sample Timeline Output (Legacy Format)

```
📅 January 15, 2024

🕘 09:30  [youtrack] Updated State in PROJ-123 (youtrack)
          → Updated field State

🕙 10:45  [youtrack] Commented on PROJ-124 (youtrack)
          → Added a comment

🕐 13:20  [youtrack] Logged work on PROJ-123 (youtrack)
          → Added work item

🕒 15:15  [youtrack] Updated Assignee in PROJ-125 (youtrack)
          → Updated field Assignee

🕘 16:00  [youtrack] Updated links for PROJ-126 (youtrack)
          → Modified issue links
```

**Note**: All issue references use human-readable keys (PROJ-123) for easy identification and navigation. This applies to all activity types including comments, field changes, work items, and link updates.

## Field Value Display

The YouTrack connector now displays the actual field values when changes are made to issues. This enhancement provides much more context about what was changed:

### Supported Value Types
- **Simple text fields**: State, Priority, Resolution
- **User fields**: Assignee, Reporter (displays name or login)
- **Multi-value fields**: Components (displays comma-separated list)
- **Custom fields**: Any custom field type supported by YouTrack

### Display Format
- **Field changes**: "Changed [Field] from [Old Value] to [New Value]"
- **Field assignments**: "Set [Field] to [New Value]"
- **Field clearing**: "Cleared [Field] (was [Old Value])"

### Metadata
Field change activities include additional metadata:
- `field_name`: The name of the changed field
- `field_new_value`: The new value (if available)
- `field_old_value`: The previous value (if available)

## API Rate Limits

YouTrack has API rate limits that may affect the connector:

- **YouTrack Cloud**: Varies by plan and usage
- **Self-hosted**: Configurable by administrators

The connector is designed to be efficient:
- Makes targeted requests for specific date ranges
- Uses appropriate field selections to minimize data transfer
- Implements reasonable request patterns

## Security Considerations

### Token Security

**Important**: Your permanent token can access YouTrack data according to its scopes. Keep it secure:

- Don't share your permanent token
- Don't commit it to version control
- Use file permissions to protect your config file (e.g., `chmod 600 ~/.config/autotime/config.yaml`)
- Revoke and regenerate if compromised
- Set appropriate expiration dates

### Token Management

To revoke/regenerate your permanent token:
1. Go to YouTrack Profile → Account Security
2. Find your token and click **Revoke**
3. Create a new token with the same scopes
4. Update your AutoTime configuration with the new token

### Network Security

- Always use HTTPS URLs for your YouTrack instance
- Ensure your YouTrack instance is properly secured
- Consider network restrictions if using self-hosted YouTrack

## Limitations

- Activities are limited to what YouTrack includes in the Activities API
- Real-time updates depend on YouTrack's activity processing
- Some activities may not be available depending on YouTrack configuration
- Date filtering is based on activity timestamps in YouTrack
- The connector requires permanent token authentication (no OAuth support currently)

## YouTrack Versions

This connector is compatible with:

- **YouTrack Cloud** - All current versions
- **YouTrack Server** - Version 2020.1 and later (REST API v2)

For older YouTrack versions, you may need to:
- Check API endpoint compatibility
- Verify permanent token functionality
- Test authentication methods

## Support

If you encounter issues with the YouTrack connector:

1. Test your connection: `autotime connectors test youtrack`
2. Check your configuration: `autotime config show`
3. Verify your YouTrack permanent token is valid and has correct scopes
4. Check that your YouTrack instance is accessible
5. Enable debug mode for detailed troubleshooting information
6. Verify your YouTrack version supports the REST API endpoints used

For YouTrack-specific issues:
- Check YouTrack documentation: https://www.jetbrains.com/help/youtrack/devportal/
- Verify your instance configuration and permissions
- Test API access manually using curl or similar tools

## Advanced Configuration

### Custom Username Filtering

If you want to track activities for a different user (requires appropriate permissions):

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://mycompany.youtrack.cloud/"
      token: "perm:admin-token-here"
      username: "other-user"  # Track another user's activities
```

### Fine-tuned Activity Types

Enable only specific activity types for focused tracking:

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://mycompany.youtrack.cloud/"
      token: "perm:abc123def456..."
      include_work_items: true   # Only time tracking
      include_comments: false    # No comments
      include_issues: false      # No field changes
```

**Note**: At least one activity category must be enabled. If you try to disable all categories, the connector will automatically enable `CustomFieldCategory` to prevent API errors.

This configuration is useful for time tracking focused workflows where you only want to see logged work items in your timeline.
### Debug Mode Configuration

Enable debug logging for troubleshooting:

```yaml
connectors:
  youtrack:
    enabled: true
    config:
      base_url: "https://mycompany.youtrack.cloud/"
      token: "perm:abc123def456..."
      log_level: "debug"         # Enable detailed debug logging
```

This configuration is useful for time tracking focused workflows where you only want to see logged work items in your timeline.