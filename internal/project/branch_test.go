package project_test

import (
	"testing"

	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/project/projecttest"
)

func TestBranchExists(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/x")
	unborn := t.TempDir()
	projecttest.Git(t, unborn, "init", "-q")
	projecttest.Git(t, unborn, "symbolic-ref", "HEAD", "refs/heads/fresh")
	for _, item := range []struct {
		name             string
		project          project.Context
		branch           string
		exists, verified bool
	}{
		{"existing branch", repo, "feature/x", true, true},
		{"current branch", repo, "main", true, true},
		{"missing branch", repo, "feature/gone", false, true},
		{"empty name", repo, "", false, true},
		{"unborn current branch", project.Context{Root: unborn}, "fresh", true, true},
		{"outside Git", project.Context{}, "main", false, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			exists, verified := item.project.BranchExists(item.branch)
			if exists != item.exists || verified != item.verified {
				t.Fatalf("branchExists(%q) = %v, %v; want %v, %v", item.branch, exists, verified, item.exists, item.verified)
			}
		})
	}
}
