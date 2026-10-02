package project

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitOutputReportsGitError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	_, err := gitOutput(t.TempDir(), "rev-parse", "--show-toplevel")
	if err == nil || !strings.Contains(err.Error(), "git rev-parse:") || !strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
		t.Fatalf("git error lost its message: %v", err)
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 128 {
		t.Fatalf("git error lost its exit status: %v", err)
	}
}

func TestDefaultFileUsesLowercaseName(t *testing.T) {
	if got := (Context{}).DefaultFile(); got != "todo.md" {
		t.Fatalf("default file = %q", got)
	}
	if got := (Context{Root: "/project"}).DefaultFile(); got != filepath.Join("/project", "todo.md") {
		t.Fatalf("project file = %q", got)
	}
}
