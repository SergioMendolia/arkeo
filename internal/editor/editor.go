package editor

import (
	"os"
	"os/exec"
	"runtime"
)

// OpenFile opens the specified file with the user's default editor
func OpenFile(filepath string) error {
	editor := getEditor()

	cmd := exec.Command(editor, filepath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getEditor returns the user's preferred editor
func getEditor() string {
	// Check environment variables in order of preference
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Fall back to platform defaults
	switch runtime.GOOS {
	case "windows":
		return "notepad"
	case "darwin":
		return "nano"
	default:
		// Linux and other Unix-like systems
		return "nano"
	}
}
