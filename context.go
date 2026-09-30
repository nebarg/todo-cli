package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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

func (project projectContext) localBranches() []string {
	branches, _ := project.localBranchesChecked()
	return branches
}

func (project projectContext) localBranchesChecked() ([]string, bool) {
	if project.root == "" {
		if project.branch == "" {
			return nil, true
		}
		return []string{project.branch}, true
	}
	output, err := gitOutput(project.root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, false
	}
	var branches []string
	for branch := range strings.SplitSeq(output, "\n") {
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	// An unborn current branch has no ref yet, but is still the active branch.
	if current := project.currentBranch(); current != "" && !slices.Contains(branches, current) {
		branches = append(branches, current)
	}
	sort.Strings(branches)
	return branches, true
}

func (project projectContext) hasLocalBranch(name string) bool {
	return slices.Contains(project.localBranches(), name)
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func defaultFile(project projectContext) string {
	if project.root != "" {
		return filepath.Join(project.root, "todo.md")
	}
	return "todo.md"
}
