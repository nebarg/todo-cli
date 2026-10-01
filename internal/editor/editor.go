// Package editor opens a file at a line in the user's editor.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ClosedMsg reports that the editor exited, with any error running it.
type ClosedMsg struct{ Err error }

// Open suspends the program, edits path at line, and sends ClosedMsg when
// the editor exits.
func Open(path string, line int) tea.Cmd {
	return tea.ExecProcess(Command(path, line), func(err error) tea.Msg { return ClosedMsg{Err: err} })
}

// Command runs $VISUAL, then $EDITOR, falling back to vi. Editors known to
// take a line number open at line.
func Command(path string, line int) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append([]string{}, parts[1:]...)
	switch filepath.Base(parts[0]) {
	case "code", "codium", "cursor":
		args = append(args, "--wait", "--goto", fmt.Sprintf("%s:%d", path, line))
	case "vi", "vim", "nvim", "view":
		args = append(args, fmt.Sprintf("+%d", line), path)
	default:
		args = append(args, path)
	}
	return exec.Command(parts[0], args...)
}
