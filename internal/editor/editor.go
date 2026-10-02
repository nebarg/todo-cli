// Package editor opens a file at a line in the user's editor.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	if runtime.GOOS == "windows" {
		// Without sh to run it, the editor's value is split at spaces.
		parts := strings.Fields(editor)
		return exec.Command(parts[0], append(parts[1:], fileArgs(parts[0], path, line)...)...)
	}
	// As Git does, sh runs the editor, so its value can quote a path with
	// spaces and add arguments of its own.
	return exec.Command("sh", append([]string{"-c", editor + ` "$@"`, editor}, fileArgs(program(editor), path, line)...)...)
}

// fileArgs are the arguments that open path at line in program, or just
// path when program isn't known to take a line number.
func fileArgs(program, path string, line int) []string {
	switch filepath.Base(program) {
	case "code", "codium", "cursor":
		return []string{"--wait", "--goto", fmt.Sprintf("%s:%d", path, line)}
	case "vi", "vim", "nvim", "view":
		return []string{fmt.Sprintf("+%d", line), path}
	}
	return []string{path}
}

// program is the first word of a shell command without its quotes and
// escapes: /opt/My Editor/vim for "/opt/My Editor/vim" -n.
func program(command string) string {
	var word strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case r == '\\' && quote != '\'':
			escaped = true
		case quote == 0 && (r == ' ' || r == '\t'):
			return word.String()
		default:
			word.WriteRune(r)
		}
	}
	return word.String()
}
