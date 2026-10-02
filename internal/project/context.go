// Package project finds the Git repository a command runs in, and asks Git
// about its branches.
package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Context is the Git repository a command runs in: its root and its current
// branch. The zero Context is outside Git.
type Context struct {
	Root   string
	Branch string
}

// Current is the Context of the working directory.
func Current() Context {
	cwd, err := os.Getwd()
	if err != nil {
		return Context{}
	}
	root, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Context{}
	}
	branch, _ := gitOutput(root, "branch", "--show-current")
	return Context{Root: root, Branch: branch}
}

// CurrentBranch asks Git for the current branch, which is empty outside Git
// and on a detached HEAD.
func (c Context) CurrentBranch() string {
	if c.Root == "" {
		return ""
	}
	branch, _ := gitOutput(c.Root, "branch", "--show-current")
	return branch
}

// LocalBranchState lists local branches, sorted, and the current branch in
// one pass. verified is false when there is no Git repository to check
// against.
func (c Context) LocalBranchState() (branches []string, current string, verified bool) {
	if c.Root == "" {
		return nil, "", false
	}
	output, err := gitOutput(c.Root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, "", false
	}
	for branch := range strings.SplitSeq(output, "\n") {
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	current = c.CurrentBranch()
	// An unborn current branch has no ref yet, but is still the active branch.
	if current != "" && !slices.Contains(branches, current) {
		branches = append(branches, current)
	}
	slices.Sort(branches)
	return branches, current, true
}

// HasLocalBranch reports whether Git has a local branch called name.
func (c Context) HasLocalBranch(name string) bool {
	exists, _ := c.branchExists(name)
	return exists
}

// branchExists checks one branch with a single git call, which is cheaper than
// LocalBranchState. verified is false when Git could not answer.
func (c Context) branchExists(name string) (exists, verified bool) {
	if name == "" {
		return false, true
	}
	if c.Root == "" {
		return false, false
	}
	_, err := gitOutput(c.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return false, false
	}
	// An unborn current branch has no ref yet, but is still the active branch.
	return c.CurrentBranch() == name, true
}

const gitTimeout = 5 * time.Second

func gitOutput(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("git %s: %s: %w", args[0], message, err)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultFile is the task file when none is given: todo.md at the repository
// root, or in the working directory outside Git.
func (c Context) DefaultFile() string {
	if c.Root != "" {
		return filepath.Join(c.Root, "todo.md")
	}
	return "todo.md"
}
