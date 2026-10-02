package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testGitProject(t *testing.T, dir, current string, others ...string) projectContext {
	t.Helper()
	if _, err := gitOutput(dir, "init", "-q"); err != nil {
		t.Skipf("Git is unavailable: %v", err)
	}
	if _, err := gitOutput(dir, "symbolic-ref", "HEAD", "refs/heads/"+current); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".test-anchor"), []byte("fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(dir, "add", ".test-anchor"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "initial"); err != nil {
		t.Fatal(err)
	}
	for _, branch := range others {
		if _, err := gitOutput(dir, "branch", branch); err != nil {
			t.Fatal(err)
		}
	}
	return projectContext{root: dir, branch: current}
}

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
	if got := defaultFile(projectContext{}); got != "todo.md" {
		t.Fatalf("default file = %q", got)
	}
	if got := defaultFile(projectContext{root: "/project"}); got != filepath.Join("/project", "todo.md") {
		t.Fatalf("project file = %q", got)
	}
}

func TestBranchExists(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/x")
	unborn := t.TempDir()
	if _, err := gitOutput(unborn, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(unborn, "symbolic-ref", "HEAD", "refs/heads/fresh"); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name             string
		project          projectContext
		branch           string
		exists, verified bool
	}{
		{"existing branch", project, "feature/x", true, true},
		{"current branch", project, "main", true, true},
		{"missing branch", project, "feature/gone", false, true},
		{"empty name", project, "", false, true},
		{"unborn current branch", projectContext{root: unborn}, "fresh", true, true},
		{"outside Git", projectContext{}, "main", false, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			exists, verified := item.project.branchExists(item.branch)
			if exists != item.exists || verified != item.verified {
				t.Fatalf("branchExists(%q) = %v, %v; want %v, %v", item.branch, exists, verified, item.exists, item.verified)
			}
		})
	}
}
