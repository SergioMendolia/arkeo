package editor

import (
	"fmt"
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

// GetAvailableEditors returns a list of commonly available editors
func GetAvailableEditors() []string {
	editors := []string{"nano", "vim", "vi", "emacs", "code", "subl", "atom"}

	if runtime.GOOS == "windows" {
		editors = append(editors, "notepad", "notepad++")
	}

	if runtime.GOOS == "darwin" {
		editors = append(editors, "TextEdit")
	}

	var available []string
	for _, editor := range editors {
		if _, err := exec.LookPath(editor); err == nil {
			available = append(available, editor)
		}
	}

	return available
}

// SetEditor sets the EDITOR environment variable
func SetEditor(editor string) error {
	// Verify the editor exists
	if _, err := exec.LookPath(editor); err != nil {
		return fmt.Errorf("editor '%s' not found in PATH", editor)
	}

	return os.Setenv("EDITOR", editor)
}
