package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
