// Package projecttest has helpers for tests that need a Git repository,
// in the manner of net/http/httptest.
package projecttest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebarg/todo-cli/internal/project"
)

// Repo makes dir a Git repository on the branch current, with one commit and
// a local branch for each of others, and returns it as a project.Repo. It
// skips the test when Git is unavailable.
func Repo(tb testing.TB, dir, current string, others ...string) project.Repo {
	tb.Helper()
	if _, err := git(dir, "init", "-q"); err != nil {
		tb.Skipf("Git is unavailable: %v", err)
	}
	Git(tb, dir, "symbolic-ref", "HEAD", "refs/heads/"+current)
	if err := os.WriteFile(filepath.Join(dir, ".test-anchor"), []byte("fixture\n"), 0644); err != nil {
		tb.Fatal(err)
	}
	Git(tb, dir, "add", ".test-anchor")
	Git(tb, dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "initial")
	for _, branch := range others {
		Git(tb, dir, "branch", branch)
	}
	return project.Repo{Root: dir, Branch: current}
}

// Git runs git -C dir with args, failing the test if Git does, and returns
// its output without surrounding whitespace.
func Git(tb testing.TB, dir string, args ...string) string {
	tb.Helper()
	out, err := git(dir, args...)
	if err != nil {
		tb.Fatal(err)
	}
	return out
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
