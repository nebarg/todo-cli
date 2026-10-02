package main

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

type projectContext struct {
	root   string
	branch string
}

func currentProject() projectContext {
	cwd, err := os.Getwd()
	if err != nil {
		return projectContext{}
	}
	root, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return projectContext{}
	}
	branch, _ := gitOutput(root, "branch", "--show-current")
	return projectContext{root: root, branch: branch}
}

func (project projectContext) currentBranch() string {
	if project.root == "" {
		return ""
	}
	branch, _ := gitOutput(project.root, "branch", "--show-current")
	return branch
}

// localBranchState lists local branches, sorted, and the current branch in
// one pass. verified is false when there is no Git repository to check
// against.
func (project projectContext) localBranchState() (branches []string, current string, verified bool) {
	if project.root == "" {
		return nil, "", false
	}
	output, err := gitOutput(project.root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, "", false
	}
	for branch := range strings.SplitSeq(output, "\n") {
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	current = project.currentBranch()
	// An unborn current branch has no ref yet, but is still the active branch.
	if current != "" && !slices.Contains(branches, current) {
		branches = append(branches, current)
	}
	slices.Sort(branches)
	return branches, current, true
}

func (project projectContext) hasLocalBranch(name string) bool {
	exists, _ := project.branchExists(name)
	return exists
}

// branchExists checks one branch with a single git call, which is cheaper than
// localBranchState. verified is false when Git could not answer.
func (project projectContext) branchExists(name string) (exists, verified bool) {
	if name == "" {
		return false, true
	}
	if project.root == "" {
		return false, false
	}
	_, err := gitOutput(project.root, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return false, false
	}
	// An unborn current branch has no ref yet, but is still the active branch.
	return project.currentBranch() == name, true
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

func defaultFile(project projectContext) string {
	if project.root != "" {
		return filepath.Join(project.root, "todo.md")
	}
	return "todo.md"
}
