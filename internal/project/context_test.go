package project

import (
	"errors"
	"os"
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

func TestDefaultFileUsesAnExistingTaskFileOfAnyCase(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TODO.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if got := (Context{Root: root}).DefaultFile(); got != filepath.Join(root, "TODO.md") {
		t.Fatalf("project file = %q, want the existing TODO.md", got)
	}
	t.Chdir(root)
	if got := (Context{}).DefaultFile(); got != "TODO.md" {
		t.Fatalf("file outside Git = %q, want the existing TODO.md", got)
	}

	withDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(withDir, "TODO.md"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := (Context{Root: withDir}).DefaultFile(); got != filepath.Join(withDir, "todo.md") {
		t.Fatalf("a TODO.md directory was taken for the task file: %q", got)
	}
}

func TestDefaultFilePrefersTodoMd(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"TODO.md", "todo.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 2 {
		t.Skip("the file system ignores case, so TODO.md and todo.md are one file")
	}
	if got := (Context{Root: root}).DefaultFile(); got != filepath.Join(root, "todo.md") {
		t.Fatalf("project file = %q, want todo.md", got)
	}
}
