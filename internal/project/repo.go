// Package project finds the Git repository a command runs in, and asks Git
// about its branches.
package project

import (
	"cmp"
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

// Repo is the Git repository a command runs in: its root and its current
// branch. The zero Repo is outside Git.
type Repo struct {
	Root   string
	Branch string
}

// Current is the Repo the working directory is in.
func Current() Repo {
	cwd, err := os.Getwd()
	if err != nil {
		return Repo{}
	}
	root, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}
	}
	branch, _ := gitOutput(root, "branch", "--show-current")
	return Repo{Root: root, Branch: branch}
}

// CurrentBranch asks Git for the current branch, which is empty outside Git
// and on a detached HEAD.
func (r Repo) CurrentBranch() string {
	if r.Root == "" {
		return ""
	}
	branch, _ := gitOutput(r.Root, "branch", "--show-current")
	return branch
}

// LocalBranchState lists local branches, sorted, and the current branch in
// one pass. verified is false when there is no Git repository to check
// against.
func (r Repo) LocalBranchState() (branches []string, current string, verified bool) {
	if r.Root == "" {
		return nil, "", false
	}
	output, err := gitOutput(r.Root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, "", false
	}
	for branch := range strings.SplitSeq(output, "\n") {
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	current = r.CurrentBranch()
	// An unborn current branch has no ref yet, but is still the active branch.
	if current != "" && !slices.Contains(branches, current) {
		branches = append(branches, current)
	}
	slices.Sort(branches)
	return branches, current, true
}

// HasLocalBranch reports whether Git has a local branch called name.
func (r Repo) HasLocalBranch(name string) bool {
	exists, _ := r.branchExists(name)
	return exists
}

// branchExists checks one branch with a single git call, which is cheaper than
// LocalBranchState. verified is false when Git could not answer.
func (r Repo) branchExists(name string) (exists, verified bool) {
	if name == "" {
		return false, true
	}
	if r.Root == "" {
		return false, false
	}
	_, err := gitOutput(r.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return false, false
	}
	// An unborn current branch has no ref yet, but is still the active branch.
	return r.CurrentBranch() == name, true
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

// taskFile is the name of the task file this app creates.
const taskFile = "todo.md"

// DefaultFile is the task file when none is given, at the repository root,
// or in the working directory outside Git: todo.md, or else a file already
// there whose name differs only in case, such as TODO.md, so a project's own
// task file is used rather than a second one made beside it.
func (r Repo) DefaultFile() string {
	dir := cmp.Or(r.Root, ".")
	return filepath.Join(dir, existingTaskFile(dir))
}

// existingTaskFile is the name to use for dir's task file. A directory that
// can't be listed gets todo.md, so reading the task file reports the problem.
func existingTaskFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return taskFile
	}
	var other string
	for _, entry := range entries {
		switch name := entry.Name(); {
		case name == taskFile:
			return taskFile
		case other == "" && !entry.IsDir() && strings.EqualFold(name, taskFile):
			other = name
		}
	}
	return cmp.Or(other, taskFile)
}
