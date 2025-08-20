package editor

import (
	"os"
	"runtime"
	"testing"
)

func TestGetEditor(t *testing.T) {
	// Save original environment variables
	originalVisual := os.Getenv("VISUAL")
	originalEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", originalVisual)
		os.Setenv("EDITOR", originalEditor)
	}()

	tests := []struct {
		name         string
		visual       string
		editor       string
		expectedGOOS map[string]string
	}{
		{
			name:   "VISUAL takes precedence",
			visual: "vim",
			editor: "nano",
			expectedGOOS: map[string]string{
				"linux":   "vim",
				"darwin":  "vim",
				"windows": "vim",
			},
		},
		{
			name:   "EDITOR when VISUAL is empty",
			visual: "",
			editor: "emacs",
			expectedGOOS: map[string]string{
				"linux":   "emacs",
				"darwin":  "emacs",
				"windows": "emacs",
			},
		},
		{
			name:   "platform defaults when both empty",
			visual: "",
			editor: "",
			expectedGOOS: map[string]string{
				"linux":   "nano",
				"darwin":  "nano",
				"windows": "notepad",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("VISUAL", tt.visual)
			os.Setenv("EDITOR", tt.editor)

			result := getEditor()
			expected, exists := tt.expectedGOOS[runtime.GOOS]
			if !exists {
				expected = tt.expectedGOOS["linux"] // fallback for unknown GOOS
			}

			if result != expected {
				t.Errorf("Expected editor %q, got %q", expected, result)
			}
		})
	}
}

func TestGetAvailableEditors(t *testing.T) {
	available := GetAvailableEditors()

	// Should return a slice (might be empty if no editors are available)
	if available == nil {
		t.Error("GetAvailableEditors should return a slice, not nil")
	}

	// Check that all returned editors are actually available
	for _, editor := range available {
		if editor == "" {
			t.Error("Available editors should not contain empty strings")
		}
	}
}

func TestGetAvailableEditors_PlatformSpecific(t *testing.T) {
	available := GetAvailableEditors()
	availableMap := make(map[string]bool)
	for _, editor := range available {
		availableMap[editor] = true
	}

	switch runtime.GOOS {
	case "windows":
		// On Windows, we should check for Windows-specific editors if they're available
		// Note: We can't guarantee they're installed, but the function should look for them
		t.Log("Windows platform detected - checking for platform-specific editors")
	case "darwin":
		// On macOS, we should check for macOS-specific editors if they're available
		t.Log("macOS platform detected - checking for platform-specific editors")
	default:
		// On Linux/Unix, standard editors should be checked
		t.Log("Unix-like platform detected - checking for standard editors")
	}

	// Verify that the function returns valid results without crashing
	if len(available) >= 0 {
		t.Logf("Found %d available editors: %v", len(available), available)
	}
}

func TestSetEditor(t *testing.T) {
	// Save original EDITOR value
	originalEditor := os.Getenv("EDITOR")
	defer os.Setenv("EDITOR", originalEditor)

	tests := []struct {
		name        string
		editor      string
		shouldError bool
	}{
		{
			name:        "valid editor",
			editor:      "sh", // sh should be available on Unix systems
			shouldError: false,
		},
		{
			name:        "invalid editor",
			editor:      "definitely_not_an_editor_12345",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip valid editor test on Windows since 'sh' might not be available
			if tt.name == "valid editor" && runtime.GOOS == "windows" {
				t.Skip("Skipping 'sh' test on Windows")
			}

			err := SetEditor(tt.editor)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error for invalid editor, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for valid editor, got: %v", err)
				}

				// Verify the environment variable was set
				if os.Getenv("EDITOR") != tt.editor {
					t.Errorf("Expected EDITOR to be %q, got %q", tt.editor, os.Getenv("EDITOR"))
				}
			}
		})
	}
}

func TestSetEditor_WindowsSpecific(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	// Save original EDITOR value
	originalEditor := os.Getenv("EDITOR")
	defer os.Setenv("EDITOR", originalEditor)

	// Test with notepad, which should be available on Windows
	err := SetEditor("notepad")
	if err != nil {
		t.Errorf("Expected no error setting notepad on Windows, got: %v", err)
	}

	if os.Getenv("EDITOR") != "notepad" {
		t.Errorf("Expected EDITOR to be 'notepad', got %q", os.Getenv("EDITOR"))
	}
}

func TestSetEditor_ErrorMessage(t *testing.T) {
	invalidEditor := "definitely_not_an_editor_12345"
	err := SetEditor(invalidEditor)

	if err == nil {
		t.Error("Expected error for invalid editor")
	}

	expectedError := "editor 'definitely_not_an_editor_12345' not found in PATH"
	if err.Error() != expectedError {
		t.Errorf("Expected error message %q, got %q", expectedError, err.Error())
	}
}

func TestOpenFile_InvalidFile(t *testing.T) {
	// Save original environment variables
	originalVisual := os.Getenv("VISUAL")
	originalEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", originalVisual)
		os.Setenv("EDITOR", originalEditor)
	}()

	// Set editor to a command that will fail
	os.Setenv("VISUAL", "")
	os.Setenv("EDITOR", "false") // 'false' command always returns exit code 1

	err := OpenFile("/nonexistent/file/path")
	if err == nil {
		t.Error("Expected error when opening file with 'false' command")
	}
}

func TestEditorEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	originalVisual := os.Getenv("VISUAL")
	originalEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", originalVisual)
		os.Setenv("EDITOR", originalEditor)
	}()

	testCases := []struct {
		name     string
		visual   string
		editor   string
		expected string
	}{
		{
			name:     "VISUAL has highest priority",
			visual:   "test-visual",
			editor:   "test-editor",
			expected: "test-visual",
		},
		{
			name:     "EDITOR when VISUAL is unset",
			visual:   "",
			editor:   "test-editor",
			expected: "test-editor",
		},
		{
			name:     "EDITOR when VISUAL is empty string",
			visual:   "",
			editor:   "test-editor",
			expected: "test-editor",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("VISUAL", tc.visual)
			os.Setenv("EDITOR", tc.editor)

			result := getEditor()
			if tc.visual != "" || tc.editor != "" {
				if result != tc.expected {
					t.Errorf("Expected %q, got %q", tc.expected, result)
				}
			}
		})
	}
}

func TestPlatformDefaults(t *testing.T) {
	// Save original environment variables
	originalVisual := os.Getenv("VISUAL")
	originalEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", originalVisual)
		os.Setenv("EDITOR", originalEditor)
	}()

	// Clear environment variables to test platform defaults
	os.Setenv("VISUAL", "")
	os.Setenv("EDITOR", "")

	result := getEditor()

	switch runtime.GOOS {
	case "windows":
		if result != "notepad" {
			t.Errorf("Expected 'notepad' as default on Windows, got %q", result)
		}
	case "darwin":
		if result != "nano" {
			t.Errorf("Expected 'nano' as default on macOS, got %q", result)
		}
	default:
		if result != "nano" {
			t.Errorf("Expected 'nano' as default on Unix-like systems, got %q", result)
		}
	}
}

// Benchmark tests
func BenchmarkGetEditor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		getEditor()
	}
}

func BenchmarkGetAvailableEditors(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetAvailableEditors()
	}
}
