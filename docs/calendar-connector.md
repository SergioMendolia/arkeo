# Google Calendar Connector

The Google Calendar connector integrates with Google Calendar to fetch your meetings, events, and appointments into your daily timeline using Google Calendar's secret iCal URLs. This approach provides a simple, OAuth-free way to access your calendar data.

## Features

- Fetches calendar events from Google Calendar using secret iCal URLs
- No OAuth setup required - uses Google Calendar's built-in sharing feature
- Supports multiple calendars from the same Google account
- Filters events by date to show only relevant daily activities
- Extracts meeting details including duration, location, and organizer
- Categorizes events as virtual or in-person meetings
- Option to include or exclude declined events
- Automatic deduplication prevents the same event from appearing multiple times
- Works with any Google Calendar that has sharing enabled

## How It Works

Google Calendar provides secret iCal URLs for each calendar that allow read-only access to calendar data without requiring OAuth authentication. These URLs look like:

```
https://calendar.google.com/calendar/ical/[calendar-id]/[secret-key]/basic.ics
```

The connector fetches these iCal feeds, parses the calendar events, and converts them into timeline activities.

## Configuration

### 1. Enable the Connector

```bash
autotime connectors enable calendar
```

### 2. Get Your Google Calendar Secret URLs

For each calendar you want to include:

#### Step 1: Open Google Calendar Settings

1. Go to [Google Calendar](https://calendar.google.com/)
2. Click the three dots next to the calendar you want to share
3. Select **Settings and sharing**

#### Step 2: Make Calendar Accessible

1. Scroll down to **Access permissions for events**
2. Check **Make available to public** (this enables the secret URL)
3. **Important**: This makes your calendar publicly accessible via the secret URL, but the URL itself contains a secret key that's hard to guess

#### Step 3: Get the Secret iCal URL

1. Scroll down to **Integrate calendar**
2. Copy the **Secret address in iCal format**
3. The URL should look like: `https://calendar.google.com/calendar/ical/your-email@gmail.com/private-abc123def456/basic.ics`

#### Step 4: Repeat for Additional Calendars

If you want to include multiple calendars (work, personal, etc.), repeat steps 1-3 for each calendar.

### 3. Configure the Connector

Edit your configuration file:

```bash
autotime config edit
```

Add your calendar configuration:

```yaml
connectors:
  calendar:
    enabled: true
    config:
      # Comma-separated list of Google Calendar secret iCal URLs
      ical_urls: "https://calendar.google.com/calendar/ical/your-email@gmail.com/private-abc123def456/basic.ics"

      # Include declined events (optional, defaults to false)
      include_declined: false
```

### Multiple Calendars

To include multiple calendars, separate the URLs with commas:

```yaml
connectors:
  calendar:
    enabled: true
    config:
      ical_urls: "https://calendar.google.com/calendar/ical/personal@gmail.com/private-abc123/basic.ics,https://calendar.google.com/calendar/ical/work@company.com/private-def456/basic.ics"
      include_declined: false
```

**Note**: When using multiple calendars, shared events (like meetings where you're invited to both personal and work calendars) are automatically deduplicated by their unique event ID, so they'll only appear once in your timeline.

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `ical_urls` | string | Yes | - | Comma-separated Google Calendar secret iCal URLs |
| `include_declined` | bool | No | `false` | Include events you've declined |

## Usage

### Test Connection

Verify your configuration works:

```bash
autotime connectors test calendar
```

This will test connectivity to all configured calendar URLs and verify they return valid calendar data.

### View Activities

Show your calendar activities for today:

```bash
autotime timeline
```

Show activities for a specific date:

```bash
autotime timeline --date 2024-01-15
```

Filter to show only calendar activities:

```bash
autotime timeline --source calendar
```

## Event Deduplication

The calendar connector automatically prevents duplicate events from appearing in your timeline. This is important when:

- **Multiple calendars contain the same event** (e.g., shared meetings)
- **Overlapping calendar URLs** point to calendars with common events
- **Recurring events** appear multiple times in iCal feeds

### How Deduplication Works

1. **Event UID tracking**: Each calendar event has a unique identifier (UID) from the iCal data
2. **Cross-calendar deduplication**: Events are deduplicated across all configured calendars
3. **First occurrence wins**: If the same event appears in multiple calendars, only the first occurrence is kept
4. **Debug visibility**: In debug mode, you can see which duplicate events are being skipped

### Debug Output Example

```
Calendar Debug: Found 5 events from calendar 1
Calendar Debug: Found 8 events from calendar 2
Calendar Debug: Skipping duplicate event 'Team Meeting' with UID: abc123def456
Calendar Debug: Skipped 2 duplicate events from calendar 2
Calendar Debug: Total activities found: 11 (after deduplication)
Calendar Debug: Unique events tracked: 11
```

## Activity Details

The calendar connector creates timeline activities with the following information:

| Field | Description | Example |
|-------|-------------|---------|
| **Type** | Always `calendar` | - |
| **Title** | Event summary/title | "Team Standup Meeting" |
| **Description** | Event description | "Daily sync with development team" |
| **Timestamp** | Event start time | 2024-01-15 09:00:00 |
| **Duration** | Event duration | 30 minutes |
| **Location** | Meeting location | "Conference Room A" or meeting URL |
| **Tags** | Auto-generated tags | `["calendar", "meeting", "virtual"]` |
| **Metadata** | Additional event data | Event ID, status, organizer |

### Auto-Generated Tags

- `calendar` - All calendar events
- `meeting` - All calendar events are tagged as meetings
- `virtual` - Events without a physical location
- `in-person` - Events with a physical location specified

## Security Considerations

### Understanding Public Calendar Sharing

When you enable the secret iCal URL for a calendar:

- **The calendar becomes publicly accessible** via the secret URL
- **The URL contains a secret key** that's difficult to guess
- **Anyone with the URL** can view your calendar events
- **The URL doesn't expire** unless you regenerate it

### Best Practices

1. **Only share calendars you're comfortable making public** with a secret URL
2. **Don't share the secret URLs** with others unless necessary
3. **Regenerate URLs if compromised**:
   - Go to calendar settings
   - Uncheck "Make available to public"
   - Check it again to get a new secret URL
4. **Consider using a separate calendar** for events you want to track in AutoTime
5. **Protect your configuration file**:
   ```bash
   chmod 600 ~/.config/autotime/config.yaml
   ```

### Privacy Considerations

- Event titles, descriptions, and locations are included in the iCal feed
- Attendee information may be included depending on your calendar settings
- Consider what information you're comfortable having in a publicly accessible (but secret) URL

## Troubleshooting

### Connection Test Fails

**Error**: `invalid Google Calendar iCal URL`
- **Solution**: Ensure your URL follows the correct format: `https://calendar.google.com/calendar/ical/[calendar-id]/[secret]/basic.ics`

**Error**: `HTTP 404: failed to fetch calendar data`
- **Solution**:
  - Verify the calendar is set to "Make available to public"
  - Check that you copied the complete secret URL
  - Regenerate the secret URL in calendar settings

**Error**: `HTTP 401/403: failed to fetch calendar data`
- **Solution**: The calendar may not be properly shared or the secret URL may be invalid

**Error**: `unexpected content type`
- **Solution**: The URL may be incorrect or not pointing to an iCal feed

### No Events Found

If the connector isn't finding calendar events:

1. **Check calendar sharing**: Verify "Make available to public" is enabled
2. **Test the URL manually**: Open the secret URL in a browser - you should see iCal data
3. **Check date range**: The connector only fetches events for the specific date requested
4. **Verify calendar has events**: Check that there are events on the date you're querying
5. **Check declined events**: If you want declined events, set `include_declined: true`

### Invalid iCal Data

**Error**: `unable to parse date/time`
- **Solution**: The iCal feed may contain invalid date formats. This usually resolves itself or indicates a temporary issue with Google Calendar

**Events missing details**:
- Some events may have minimal information in the iCal feed
- All-day events may appear differently than timed events
- Private events may have limited details even in your own calendar feed

### Debug Mode Troubleshooting

Enable debug mode to get detailed information about issues:

```bash
export AUTOTIME_DEBUG=1
autotime connectors test calendar
```

#### Common Debug Scenarios

**HTTP Connection Issues:**
Debug output will show the exact HTTP status codes and error responses:
```
Calendar Debug: Response status: 404 Not Found
Calendar Debug: Failed to connect to calendar 1: HTTP 404: failed to fetch calendar data
```
- **Solution**: Check that the calendar is properly shared and the URL is correct

**iCal Parsing Problems:**
Debug mode shows parsing progress and identifies problematic lines:
```
Calendar Debug: Failed to parse DTSTART '20240230T090000': unable to parse date/time: 20240230T090000
Calendar Debug: Skipping event with empty summary
```
- **Solution**: Invalid dates or malformed iCal data - usually resolves itself or indicates temporary Google Calendar issues

**Date Filtering Issues:**
See exactly which events are found and filtered:
```
Calendar Debug: Parsed 25 total events from iCal data
Calendar Debug: Events for target date: 3
Calendar Debug: Declined events skipped: 1
Calendar Debug: Final activities created: 2
```
- **Solution**: Verify you're querying the correct date and check your `include_declined` setting

**Empty Results with Valid Connection:**
Debug shows successful connection but no matching events:
```
Calendar Debug: Successfully connected to calendar 1
Calendar Debug: Parsed 15 total events from iCal data
Calendar Debug: Events for target date: 0
```
- **Solution**: No events exist on the requested date, try a different date or check your calendar

**Duplicate Event Detection:**
Debug shows events being deduplicated across calendars:
```
Calendar Debug: Found 3 events from calendar 1
Calendar Debug: Found 5 events from calendar 2
Calendar Debug: Skipping duplicate event 'Daily Standup' with UID: meeting123
Calendar Debug: Skipped 2 duplicate events from calendar 2
```
- **This is normal**: Multiple calendars often contain the same shared events

## Examples

### Single Calendar

```yaml
connectors:
  calendar:
    enabled: true
    config:
      ical_urls: "https://calendar.google.com/calendar/ical/john.doe@gmail.com/private-abc123def456/basic.ics"
      include_declined: false
```

### Multiple Calendars

```yaml
connectors:
  calendar:
    enabled: true
    config:
      ical_urls: "https://calendar.google.com/calendar/ical/personal@gmail.com/private-abc123/basic.ics,https://calendar.google.com/calendar/ical/work@company.com/private-def456/basic.ics"
      include_declined: false
```

### Including Declined Events

```yaml
connectors:
  calendar:
    enabled: true
    config:
      ical_urls: "https://calendar.google.com/calendar/ical/john.doe@gmail.com/private-abc123def456/basic.ics"
      include_declined: true
```

### Sample Timeline Output

```
📅 January 15, 2024

🕘 09:00  [calendar] Team Standup Meeting (30m)
          → Conference Room A • Daily sync with development team

🕙 10:30  [calendar] Client Presentation (1h)
          → Virtual Meeting • Q4 progress review

🕐 13:00  [calendar] Lunch Meeting (1h)
          → Restaurant Downtown • Networking event

🕒 15:00  [calendar] Sprint Planning (2h)
          → Virtual Meeting • Planning next sprint items

🕓 16:30  [calendar] One-on-One (30m)
          → Manager's Office • Weekly check-in
```

## Debug Mode

The calendar connector includes comprehensive debug logging to help troubleshoot issues with calendar access and event parsing.

### Enabling Debug Mode

Enable debug mode using environment variables:

```bash
export AUTOTIME_DEBUG=1
autotime connectors test calendar
```

Or:

```bash
export LOG_LEVEL=debug
autotime timeline --source calendar --date 2024-01-15
```

### Debug Information

When debug mode is enabled, the calendar connector will log detailed information about:

#### Connection Testing
- Number of calendars being tested
- HTTP requests to each iCal URL (with masked secrets)
- HTTP response status codes and content types
- Connection success/failure for each calendar

Example debug output:
```
Calendar Debug: Testing connection to 2 calendar(s)
Calendar Debug: Testing calendar 1: https://calendar.google.com/calendar/ical/personal@gmail.com/****/basic.ics
Calendar Debug: Making HTTP request to https://calendar.google.com/calendar/ical/personal@gmail.com/****/basic.ics
Calendar Debug: Response status: 200 OK
Calendar Debug: Content-Type: text/calendar; charset=utf-8
Calendar Debug: Successfully validated iCal response
Calendar Debug: Successfully connected to calendar 1
```

#### Event Fetching
- Calendar URLs being processed (with secrets masked)
- HTTP response details
- iCal parsing progress and line counts
- Number of events found in each calendar
- Date filtering and event matching

Example debug output:
```
Calendar Debug: Fetching events for date 2024-01-15 from 2 calendar(s)
Calendar Debug: Processing calendar 1: https://calendar.google.com/calendar/ical/personal@gmail.com/****/basic.ics
Calendar Debug: HTTP response: 200 OK
Calendar Debug: Starting to parse iCal data
Calendar Debug: Found event 1 at line 45
Calendar Debug: Completed parsing event: Team Meeting (Start: 2024-01-15 09:00)
Calendar Debug: Parsed 154 lines, found 12 events, extracted 8 valid events
Calendar Debug: Filtering events for target date: 2024-01-15
Calendar Debug: Found 3 events from calendar 1
```

#### Event Processing
- Event filtering by date
- Declined event handling
- Date/time parsing details
- Activity creation process

Example debug output:
```
Calendar Debug: Events for target date: 5
Calendar Debug: Include declined events: false
Calendar Debug: Skipping declined event: Optional Training Session
Calendar Debug: Added event: Team Standup at 09:00 (duration: 30m0s)
Calendar Debug: Declined events skipped: 1
Calendar Debug: Final activities created: 4
Calendar Debug: Skipping duplicate event 'Weekly Sync' with UID: xyz789abc123
Calendar Debug: Skipped 1 duplicate events from calendar 2
```

#### Error Details
- Failed HTTP requests with status codes
- iCal parsing errors
- Date/time format issues
- Invalid event data

Example debug output:
```
Calendar Debug: Failed to parse DTSTART '20241501T090000': unable to parse date/time: 20241501T090000
Calendar Debug: Failed to connect to calendar 2: HTTP 404: failed to fetch calendar data
```

### Security Note

Debug logs automatically mask the secret portions of iCal URLs for security:
- Original: `https://calendar.google.com/calendar/ical/user@gmail.com/abc123def456/basic.ics`
- Masked: `https://calendar.google.com/calendar/ical/user@gmail.com/***/basic.ics`

However, debug logs may still contain sensitive information such as:
- Event titles and descriptions
- Meeting locations
- Organizer information
- Calendar email addresses

**Important**: Disable debug mode in production and don't share debug logs publicly.

## Debugging

Enable debug mode for detailed troubleshooting:

```bash
export AUTOTIME_DEBUG=1
autotime connectors test calendar
```

Or:

```bash
export LOG_LEVEL=debug
autotime timeline --source calendar --date 2024-01-15
```

Debug mode will show:
- HTTP requests to calendar URLs (with secrets masked)
- Response status codes and content types
- iCal parsing progress and line-by-line processing
- Number of events found in each calendar
- Date parsing and filtering details
- Event conversion process
- Detailed error information for troubleshooting

## Working with Different Calendar Types

### Personal Calendars

For your main Google Calendar:
1. The calendar ID is usually your email address
2. The URL will look like: `https://calendar.google.com/calendar/ical/your-email@gmail.com/private-key/basic.ics`

### Secondary Calendars

For additional calendars you've created:
1. Go to the specific calendar's settings
2. The calendar ID might be a long string like `abc123def456@group.calendar.google.com`
3. Enable public sharing for each calendar individually

### Shared Calendars

For calendars shared with you:
1. You can only get the secret URL if you have appropriate permissions
2. The calendar owner may need to enable public sharing
3. Consider asking the owner to provide the secret URL directly

## Limitations

- **Public sharing required**: Calendars must be made publicly accessible (via secret URL)
- **Read-only access**: Cannot modify calendar events through AutoTime
- **iCal format limitations**: Some advanced calendar features may not be available in iCal format
- **Real-time updates**: Changes to calendar events may take time to appear in iCal feeds
- **Date filtering**: Only events for the specific requested date are included
- **Time zone handling**: Events are displayed in your local time zone
- **Deduplication scope**: Events are deduplicated by UID across all calendars, so shared events appear only once

## Advantages Over OAuth

- **No complex setup**: No need to create Google Cloud projects or OAuth credentials
- **No token management**: Secret URLs don't expire like OAuth tokens
- **Simpler authentication**: Just copy and paste URLs
- **Works immediately**: No approval processes or API quotas to worry about
- **Multiple calendars**: Easy to add multiple calendars from different accounts
- **Automatic deduplication**: Shared events across calendars are automatically deduplicated

## Alternative: Using OAuth

If you prefer OAuth-based authentication (more secure but more complex), you can:
1. Use the Google Calendar API directly
2. Set up OAuth2 credentials in Google Cloud Console
3. Implement token refresh logic
4. This approach doesn't require making calendars public

The current iCal-based approach prioritizes simplicity and ease of setup over maximum security.

## Support

If you encounter issues with the calendar connector:

1. **Test your connection**: `autotime connectors test calendar`
2. **Check your configuration**: `autotime config show`
3. **Verify calendar sharing**: Ensure "Make available to public" is enabled
4. **Test URLs manually**: Open secret URLs in a browser to verify they work
5. **Enable debug mode**: Set `LOG_LEVEL=debug` for detailed troubleshooting
6. **Check debug output**: Look for specific error messages and HTTP status codes
7. **Verify iCal format**: Ensure the URLs return valid iCal data

For Google Calendar-specific issues:
- [Google Calendar Help](https://support.google.com/calendar/)
- [Calendar Sharing Documentation](https://support.google.com/calendar/answer/37083)

## Contributing

The calendar connector is actively maintained. Areas where contributions are welcome:

- **Improved iCal parsing**: Handle more iCal features and edge cases
- **Error handling**: Better error messages and recovery
- **Time zone support**: Improved handling of different time zones
- **Event filtering**: Additional filtering options
- **Performance optimization**: Caching and efficient parsing
- **Testing and bug reports**: Help identify and fix issues

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines on contributing to the project.
