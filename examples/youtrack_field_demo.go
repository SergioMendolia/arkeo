package main

import (
	"fmt"
	"time"

	"github.com/autotime/autotime/internal/connectors"
	"github.com/autotime/autotime/internal/timeline"
)

// This example demonstrates the new field value display functionality
// in the YouTrack connector. It shows how field changes are now displayed
// with actual values instead of just generic "field updated" messages.

func main() {
	fmt.Println("YouTrack Field Value Display Demo")
	fmt.Println("=================================")
	fmt.Println()

	// Create a YouTrack connector
	connector := connectors.NewYouTrackConnector()

	// Example field change activities that would come from YouTrack API
	examples := []struct {
		name        string
		fieldName   string
		oldValue    interface{}
		newValue    interface{}
		description string
	}{
		{
			name:        "State Change",
			fieldName:   "State",
			oldValue:    "Open",
			newValue:    "In Progress",
			description: "Simple string field change",
		},
		{
			name:      "Priority Change with Objects",
			fieldName: "Priority",
			oldValue: map[string]interface{}{
				"name": "Normal",
				"id":   "priority-2",
			},
			newValue: map[string]interface{}{
				"name": "High",
				"id":   "priority-1",
			},
			description: "Object field change (extracts 'name' property)",
		},
		{
			name:        "Assignee Assignment",
			fieldName:   "Assignee",
			oldValue:    nil,
			newValue:    "john.doe",
			description: "Field set with no previous value",
		},
		{
			name:        "Assignee Cleared",
			fieldName:   "Assignee",
			oldValue:    "jane.smith",
			newValue:    nil,
			description: "Field cleared (old value shown)",
		},
		{
			name:        "Tags Update",
			fieldName:   "Tags",
			oldValue:    []interface{}{"frontend"},
			newValue:    []interface{}{"backend", "api", "database"},
			description: "Multi-value field change (arrays)",
		},
		{
			name:      "User Assignment with Full Object",
			fieldName: "Assignee",
			oldValue: map[string]interface{}{
				"login": "alice.jones",
				"name":  "Alice Jones",
				"id":    "user-1",
			},
			newValue: map[string]interface{}{
				"login": "bob.smith",
				"name":  "Bob Smith",
				"id":    "user-2",
			},
			description: "User object change (prioritizes 'name' over 'login')",
		},
	}

	// Simulate converting these field changes to activities
	for i, example := range examples {
		fmt.Printf("Example %d: %s\n", i+1, example.name)
		fmt.Printf("Description: %s\n", example.description)

		// Create a mock YouTrack activity
		ytActivity := createMockActivity(example.fieldName, example.oldValue, example.newValue)

		// Convert to timeline activity (this would normally be done by the connector)
		activity := convertToTimelineActivity(connector, ytActivity)

		fmt.Printf("Result:\n")
		fmt.Printf("  Title: %s\n", activity.Title)
		fmt.Printf("  Description: %s\n", activity.Description)

		// Show metadata
		if fieldName, exists := activity.Metadata["field_name"]; exists {
			fmt.Printf("  Field Name: %s\n", fieldName)
		}
		if newVal, exists := activity.Metadata["field_new_value"]; exists {
			fmt.Printf("  New Value: %s\n", newVal)
		}
		if oldVal, exists := activity.Metadata["field_old_value"]; exists {
			fmt.Printf("  Old Value: %s\n", oldVal)
		}

		fmt.Println()
	}

	fmt.Println("Key Benefits:")
	fmt.Println("- Users can see exactly what changed without opening YouTrack")
	fmt.Println("- Field changes are now actionable and informative")
	fmt.Println("- Both old and new values are preserved in metadata for further processing")
	fmt.Println("- Handles various data types: strings, objects, arrays")
	fmt.Println("- Gracefully handles missing values (field sets/clears)")
}

// Mock YouTrack activity structure for demonstration
type mockYouTrackActivity struct {
	ID        string
	Timestamp int64
	Author    struct {
		Login string
		Name  string
	}
	Category struct {
		ID   string
		Name string
	}
	Target interface{}
	Field  *struct {
		Name string
	}
	Added   interface{}
	Removed interface{}
}

func createMockActivity(fieldName string, oldValue, newValue interface{}) mockYouTrackActivity {
	return mockYouTrackActivity{
		ID:        "demo-activity",
		Timestamp: time.Now().Unix() * 1000,
		Author: struct {
			Login string
			Name  string
		}{
			Login: "demo.user",
			Name:  "Demo User",
		},
		Category: struct {
			ID   string
			Name string
		}{
			ID:   "CustomFieldCategory",
			Name: "Custom Field",
		},
		Target: map[string]interface{}{
			"id":         "1-123",
			"idReadable": "DEMO-123",
			"summary":    "Demo Issue",
			"project": map[string]interface{}{
				"name":      "Demo Project",
				"shortName": "DEMO",
			},
		},
		Field: &struct {
			Name string
		}{
			Name: fieldName,
		},
		Added:   newValue,
		Removed: oldValue,
	}
}

// This simulates what the connector does internally
func convertToTimelineActivity(connector *connectors.YouTrackConnector, ytActivity mockYouTrackActivity) *timeline.Activity {
	// This is a simplified version of the actual conversion logic
	// In reality, this would use the connector's convertActivity method

	fieldName := ytActivity.Field.Name
	newValue := extractFieldValue(ytActivity.Added)
	oldValue := extractFieldValue(ytActivity.Removed)

	var title, description string

	// Build title with new value if available
	if newValue != "" {
		title = fmt.Sprintf("Updated %s to %s in DEMO-123", fieldName, newValue)
	} else {
		title = fmt.Sprintf("Updated %s in DEMO-123", fieldName)
	}

	// Build description with old and new values if available
	if newValue != "" && oldValue != "" {
		description = fmt.Sprintf("Changed %s from %s to %s", fieldName, oldValue, newValue)
	} else if newValue != "" {
		description = fmt.Sprintf("Set %s to %s", fieldName, newValue)
	} else if oldValue != "" {
		description = fmt.Sprintf("Cleared %s (was %s)", fieldName, oldValue)
	} else {
		description = fmt.Sprintf("Modified %s", fieldName)
	}

	// Build metadata
	metadata := map[string]string{
		"category":   "CustomFieldCategory",
		"field_name": fieldName,
		"author":     ytActivity.Author.Login,
		"issue_key":  "DEMO-123",
		"project":    "Demo Project",
	}

	if newValue != "" {
		metadata["field_new_value"] = newValue
	}
	if oldValue != "" {
		metadata["field_old_value"] = oldValue
	}

	return &timeline.Activity{
		ID:          fmt.Sprintf("youtrack-%s", ytActivity.ID),
		Type:        timeline.ActivityTypeYouTrack,
		Title:       title,
		Description: description,
		Timestamp:   time.Unix(ytActivity.Timestamp/1000, 0),
		Source:      "youtrack",
		URL:         "https://demo.youtrack.cloud/issue/DEMO-123",
		Tags:        []string{"youtrack", "issue", "field-update", "demo project"},
		Metadata:    metadata,
	}
}

// Simplified version of the extractFieldValue function for demo purposes
func extractFieldValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if name, exists := v["name"]; exists {
			if nameStr, ok := name.(string); ok {
				return nameStr
			}
		}
		if login, exists := v["login"]; exists {
			if loginStr, ok := login.(string); ok {
				return loginStr
			}
		}
		return ""
	case []interface{}:
		var values []string
		for _, item := range v {
			if extracted := extractFieldValue(item); extracted != "" {
				values = append(values, extracted)
			}
		}
		if len(values) > 0 {
			return fmt.Sprintf("%v", values)
		}
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
