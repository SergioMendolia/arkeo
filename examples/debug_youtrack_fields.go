package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/autotime/autotime/internal/connectors"
)

// This script helps debug field value extraction issues with real YouTrack data.
// It enables debug mode and shows you exactly what the API returns and how
// values are being extracted.

func main() {
	fmt.Println("YouTrack Field Value Extraction Debug Tool")
	fmt.Println("==========================================")
	fmt.Println()

	// Check for required environment variables
	baseURL := os.Getenv("YOUTRACK_BASE_URL")
	token := os.Getenv("YOUTRACK_TOKEN")
	username := os.Getenv("YOUTRACK_USERNAME")

	if baseURL == "" || token == "" {
		fmt.Println("ERROR: Missing required environment variables")
		fmt.Println("Please set the following environment variables:")
		fmt.Println("  YOUTRACK_BASE_URL - Your YouTrack URL (e.g., https://company.youtrack.cloud/)")
		fmt.Println("  YOUTRACK_TOKEN - Your YouTrack permanent token")
		fmt.Println("  YOUTRACK_USERNAME - Your YouTrack username (optional, defaults to current user)")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  export YOUTRACK_BASE_URL='https://company.youtrack.cloud/'")
		fmt.Println("  export YOUTRACK_TOKEN='perm:abc123def456...'")
		fmt.Println("  export YOUTRACK_USERNAME='john.doe'")
		fmt.Println("  go run debug_youtrack_fields.go")
		os.Exit(1)
	}

	fmt.Printf("Base URL: %s\n", baseURL)
	fmt.Printf("Username: %s\n", username)
	fmt.Println()

	// Create and configure the connector
	connector := connectors.NewYouTrackConnector()
	config := map[string]interface{}{
		"base_url":           baseURL,
		"token":              token,
		"include_issues":     true,
		"include_comments":   false,
		"include_work_items": false,
		"log_level":          "debug", // Enable debug mode
	}

	if username != "" {
		config["username"] = username
	}

	err := connector.Configure(config)
	if err != nil {
		log.Fatalf("Failed to configure connector: %v", err)
	}

	fmt.Println("Testing connection...")
	err = connector.TestConnection()
	if err != nil {
		log.Fatalf("Connection test failed: %v", err)
	}
	fmt.Println("✅ Connection successful!")
	fmt.Println()

	// Get activities for today
	fmt.Println("Fetching activities for today with debug logging...")
	fmt.Println("Look for debug messages below that show field extraction details:")
	fmt.Println("=================================================================")

	activities, err := connector.GetActivities(time.Now())
	if err != nil {
		log.Fatalf("Failed to get activities: %v", err)
	}

	fmt.Println("=================================================================")
	fmt.Printf("Found %d activities\n", len(activities))
	fmt.Println()

	// Show results
	if len(activities) == 0 {
		fmt.Println("No activities found for today.")
		fmt.Println("Try checking a different date or ensure you have field changes in YouTrack.")
		return
	}

	fmt.Println("Activity Results:")
	fmt.Println("=================")
	for i, activity := range activities {
		fmt.Printf("%d. %s\n", i+1, activity.Title)
		fmt.Printf("   Description: %s\n", activity.Description)

		// Show field-related metadata
		if fieldName, exists := activity.Metadata["field_name"]; exists {
			fmt.Printf("   Field Name: %s\n", fieldName)
		}
		if newValue, exists := activity.Metadata["field_new_value"]; exists {
			fmt.Printf("   New Value: %s\n", newValue)
		}
		if oldValue, exists := activity.Metadata["field_old_value"]; exists {
			fmt.Printf("   Old Value: %s\n", oldValue)
		}
		fmt.Println()
	}

	fmt.Println("Debug Analysis:")
	fmt.Println("===============")
	fmt.Println("1. Check the debug messages above for 'Extracting field value from:' lines")
	fmt.Println("2. Look for 'Processing object with keys:' to see what fields are available")
	fmt.Println("3. If you see 'Could not extract value from object:' messages, that indicates missing field mappings")
	fmt.Println()
	fmt.Println("Common Issues:")
	fmt.Println("- If activities show 'User' instead of usernames, the API isn't returning user details")
	fmt.Println("- If activities show 'StateBundleElement' instead of state names, field names are missing")
	fmt.Println("- Missing values indicate the API fields specification needs adjustment")
	fmt.Println()
	fmt.Println("Solutions:")
	fmt.Println("- Ensure your YouTrack version supports the requested API fields")
	fmt.Println("- Check if your token has sufficient permissions")
	fmt.Println("- Try updating YouTrack to a newer version if using an older instance")
}

// Example usage and output:
//
// $ export YOUTRACK_BASE_URL='https://company.youtrack.cloud/'
// $ export YOUTRACK_TOKEN='perm:abc123def456...'
// $ export YOUTRACK_USERNAME='john.doe'
// $ go run debug_youtrack_fields.go
//
// Expected output with working field extraction:
// YouTrack Field Value Extraction Debug Tool
// ==========================================
//
// Base URL: https://company.youtrack.cloud/
// Username: john.doe
//
// Testing connection...
// ✅ Connection successful!
//
// Fetching activities for today with debug logging...
// Look for debug messages below that show field extraction details:
// =================================================================
// YouTrack Debug: Extracting field value from: map[$type:User fullName:John Doe id:1-1 login:john.doe] (type: map[string]interface {})
// YouTrack Debug: Processing object with keys: [$type fullName id login]
// YouTrack Debug: Extracted fullName: John Doe
// YouTrack Debug: Extracting field value from: map[$type:StateBundleElement id:1-5 name:In Progress] (type: map[string]interface {})
// YouTrack Debug: Processing object with keys: [$type id name]
// YouTrack Debug: Extracted name: In Progress
// =================================================================
// Found 2 activities
//
// Activity Results:
// =================
// 1. Updated Assignee to John Doe in PROJ-123
//    Description: Set Assignee to John Doe
//    Field Name: Assignee
//    New Value: John Doe
//
// 2. Updated State to In Progress in PROJ-124
//    Description: Changed State from Open to In Progress
//    Field Name: State
//    New Value: In Progress
//    Old Value: Open
