package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCategoryArg(t *testing.T) {
	for _, item := range []struct {
		name     string
		args     []string
		category string
		rest     string
	}{
		{"leading category", []string{"@boundary", "Fix", "the", "thing"}, "@boundary", "Fix the thing"},
		{"no category", []string{"Fix", "it"}, "", "Fix it"},
		{"later at sign stays in the title", []string{"Email", "@sam"}, "", "Email @sam"},
		{"bare at sign is text", []string{"@", "later"}, "", "@ later"},
		{"category only", []string{"@boundary"}, "@boundary", ""},
		{"empty", nil, "", ""},
	} {
		t.Run(item.name, func(t *testing.T) {
			category, rest := splitCategoryArg(item.args)
			if category != item.category || strings.Join(rest, " ") != item.rest {
				t.Fatalf("splitCategoryArg(%q) = %q, %q", item.args, category, rest)
			}
		})
	}
}

func TestChooseCommand(t *testing.T) {
	for _, item := range []struct {
		argv string
		want command
		err  string
	}{
		{"", commandDashboard, ""},
		{"-e dist", commandDashboard, ""},
		{"Fix the bug", commandAdd, ""},
		{"scan the logs", commandAdd, ""},
		{"-b . -p h This is my task", commandAdd, ""},
		{"--branch=feature/x --priority high Fix", commandAdd, ""},
		{"-p h Fix it -c later", commandAdd, ""},
		{"--clear-done", commandClear, ""},
		{"-f tasks.md --clear-done", commandClear, ""},
		{"--clear-missing", commandClear, ""},
		{"--clear-done --clear-missing", commandClear, ""},
		{"--clear-done now", 0, "usage: todo [-f file] [--clear-done]"},
		{"-e dist --clear-missing", 0, "usage: todo [-f file] [--clear-done]"},
		{"-e dist Fix it", 0, "--exclude is only for the dashboard"},
		{"-b .", 0, "-p, -c and -b need task text"},
	} {
		t.Run(item.argv, func(t *testing.T) {
			var o options
			flags := newFlags(&o)
			if err := flags.Parse(strings.Fields(item.argv)); err != nil {
				t.Fatal(err)
			}
			got, err := chooseCommand(o, flags.Args())
			if item.err != "" {
				if _, ok := errors.AsType[usageError](err); !ok || !strings.HasPrefix(err.Error(), item.err) {
					t.Fatalf("error = %v, want usage error %q", err, item.err)
				}
				return
			}
			if err != nil || got != item.want {
				t.Fatalf("command = %v, %v; want %v", got, err, item.want)
			}
		})
	}
}

func TestFlagsStopAtTheTask(t *testing.T) {
	var o options
	flags := newFlags(&o)
	if err := flags.Parse([]string{"-p", "h", "Fix", "-c", "later", "--", "now"}); err != nil {
		t.Fatal(err)
	}
	if o.priority != "h" || o.category != "" || strings.Join(flags.Args(), " ") != "Fix -c later -- now" {
		t.Fatalf("priority %q, category %q, task %q", o.priority, o.category, flags.Args())
	}
	flags = newFlags(&o)
	if err := flags.Parse([]string{"--", "-p", "is", "text"}); err != nil || strings.Join(flags.Args(), " ") != "-p is text" {
		t.Fatalf("-- did not end the flags: %q, %v", flags.Args(), err)
	}
}

func TestAddTaskOnBranches(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/x")
	path := filepath.Join(dir, "todo.md")
	if err := addTask(path, project, options{branch: ".", priority: "h"}, []string{"Current", "task"}); err != nil {
		t.Fatal(err)
	}
	if err := addTask(path, project, options{branch: "feature/x"}, []string{"Other", "task"}); err != nil {
		t.Fatal(err)
	}
	if err := addTask(path, project, options{branch: "gone"}, []string{"Lost"}); err == nil || !strings.Contains(err.Error(), `branch "gone" does not exist locally`) {
		t.Fatalf("missing branch error = %v", err)
	}
	if err := addTask(path, project, options{branch: "."}, []string{"@tests", "Both"}); err == nil {
		t.Fatal("a branch task was given a category")
	}
	if err := addTask(path, projectContext{}, options{branch: "."}, []string{"No", "Git"}); err == nil || err.Error() != "no current Git branch" {
		t.Fatalf("-b . outside Git = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Branches\n\n## main\n\n- [ ] Current task !high\n\n## feature/x\n\n- [ ] Other task\n"; string(data) != want {
		t.Fatalf("todo.md = %q, want %q", data, want)
	}
}

func TestScanFlagsAreGone(t *testing.T) {
	for _, flag := range []string{"--scan", "--all-files"} {
		var o options
		if err := newFlags(&o).Parse([]string{flag}); err == nil {
			t.Errorf("%s was accepted", flag)
		}
	}
}
