package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
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
		return project.branch
	}
	branch, _ := gitOutput(project.root, "branch", "--show-current")
	return branch
}

// localBranchState lists local branches and the current branch in one pass.
// verified is false when there is no Git repository to check against.
func (project projectContext) localBranchState() (branches []string, current string, verified bool) {
	if project.root == "" {
		if project.branch == "" {
			return nil, "", false
		}
		return []string{project.branch}, project.branch, false
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
	sort.Strings(branches)
	return branches, current, true
}

func (project projectContext) hasLocalBranch(name string) bool {
	branches, _, _ := project.localBranchState()
	return slices.Contains(branches, name)
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
			return "", fmt.Errorf("git %s: %s", args[0], message)
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
