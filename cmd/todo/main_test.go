package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nebarg/todo-cli/internal/buildinfo"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/project/projecttest"
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
	repo := projecttest.Repo(t, dir, "main", "feature/x")
	path := filepath.Join(dir, "todo.md")
	if err := addTask(io.Discard, path, repo, options{branch: ".", priority: "h"}, []string{"Current", "task"}); err != nil {
		t.Fatal(err)
	}
	if err := addTask(io.Discard, path, repo, options{branch: "feature/x"}, []string{"Other", "task"}); err != nil {
		t.Fatal(err)
	}
	if err := addTask(io.Discard, path, repo, options{branch: "gone"}, []string{"Lost"}); err == nil || !strings.Contains(err.Error(), `branch "gone" does not exist locally`) {
		t.Fatalf("missing branch error = %v", err)
	}
	if err := addTask(io.Discard, path, repo, options{branch: "."}, []string{"@tests", "Both"}); err == nil {
		t.Fatal("a branch task was given a category")
	}
	if err := addTask(io.Discard, path, project.Context{}, options{branch: "."}, []string{"No", "Git"}); err == nil || err.Error() != "no current Git branch" {
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

func TestRun(t *testing.T) {
	t.Chdir(t.TempDir()) // outside any Git repository
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [x] Shipped\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args        string
		out, errOut string
		status      int
	}{
		{"-f " + path + " -p h Fix the bug", "Added to " + path + ": Fix the bug\n", "", 0},
		{"-f " + path + " --clear-done", "Removed 1 done task from " + path + ":\n  Shipped\n", "", 0},
		{"-f " + path + " --clear-done", "No done tasks in " + path + "\n", "", 0},
		{"--wat", "", "unknown flag: --wat\nRun todo --help for usage\n", 2},
		{"-f " + path + " --clear-done now", "", "usage: todo [-f file] [--clear-done] [--clear-missing]\n", 2},
		{"-f " + path, "", "the dashboard needs a terminal; give a task to add, or use todo-scan --list\n", 2},
		{"-f " + path + " -e /", "", "--exclude needs a directory name or path\n", 2},
		{"-f " + path + " --clear-missing", "", "--clear-missing needs a Git repository to check branches against\n", 1},
		{"-f " + path + " -b . Fix", "", "no current Git branch\n", 1},
	} {
		t.Run(c.args, func(t *testing.T) {
			var out, errOut strings.Builder
			status := run(strings.Fields(c.args), &out, &errOut)
			if out.String() != c.out || errOut.String() != c.errOut || status != c.status {
				t.Fatalf("got %q, %q, %d\nwant %q, %q, %d", out.String(), errOut.String(), status, c.out, c.errOut, c.status)
			}
		})
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "- [ ] Fix the bug !high\n" {
		t.Fatalf("todo.md = %q, %v", data, err)
	}

	var out, errOut strings.Builder
	if status := run([]string{"--help"}, &out, &errOut); status != 0 || !strings.HasPrefix(out.String(), "Usage:") || errOut.Len() > 0 {
		t.Fatalf("--help = %d, %q, %q", status, out.String(), errOut.String())
	}
}

func TestRunVersion(t *testing.T) {
	t.Chdir(t.TempDir()) // outside any Git repository
	path := filepath.Join(t.TempDir(), "todo.md")
	for _, args := range []string{"--version", "--version Fix the bug", "-f " + path + " -p h --version Fix the bug", "--version --clear-done"} {
		t.Run(args, func(t *testing.T) {
			var out, errOut strings.Builder
			status := run(strings.Fields(args), &out, &errOut)
			if want := "todo " + buildinfo.Version() + "\n"; out.String() != want || errOut.Len() > 0 || status != 0 {
				t.Fatalf("got %q, %q, %d\nwant %q, \"\", 0", out.String(), errOut.String(), status, want)
			}
		})
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("--version wrote the task file: %v", err)
	}
}

const clearContent = "- [x] Loose done\n\n- [ ] Loose open\n\n# docs\n\n- [x] Doc done\n\n# auth\n\n- [x] Auth done\n\n- [ ] Auth open\n\n# Branches\n\n## feature/x\n\n- [x] Branch done\n"

const missingContent = "- [x] Loose done\n\n# Branches\n\n## feature/gone\n\n- [ ] Gone open\n\n- [x] Gone done\n\n## feature/live\n\n- [ ] Live open\n\n- [x] Live done\n"

func fileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func goneMissing(branch string) bool { return branch == "feature/gone" }

func TestClearDoneCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte(clearContent), 0644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := clearTasks(&out, path, true, nil); err != nil {
		t.Fatal(err)
	}
	want := "Removed 4 done tasks from " + path + ":\n  Loose done\n  Doc done\n  Auth done\n  Branch done\nRemoved empty headings: @docs, feature/x\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
	if got := fileContent(t, path); got != "- [ ] Loose open\n\n# auth\n\n- [ ] Auth open\n" {
		t.Fatalf("file = %q", got)
	}
	out.Reset()
	if err := clearTasks(&out, path, true, nil); err != nil || out.String() != "No done tasks in "+path+"\n" {
		t.Fatalf("second clear = %q, %v", out.String(), err)
	}
	out.Reset()
	if err := clearTasks(&out, filepath.Join(t.TempDir(), "missing.md"), true, nil); err != nil || !strings.HasPrefix(out.String(), "No done tasks") {
		t.Fatalf("missing file = %q, %v", out.String(), err)
	}
}

func TestClearDoneAndMissingTogether(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte(missingContent), 0644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := clearTasks(&out, path, true, goneMissing); err != nil {
		t.Fatal(err)
	}
	want := "Removed 2 done tasks and 2 tasks of missing branches from " + path + ":\n  Loose done\n  Gone open\n  Gone done\n  Live done\nBranches no longer in Git: feature/gone\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
	if got := fileContent(t, path); got != "# Branches\n\n## feature/live\n\n- [ ] Live open\n" {
		t.Fatalf("file = %q", got)
	}
}

func TestClearDoneOrMissingAlone(t *testing.T) {
	for _, item := range []struct {
		name    string
		done    bool
		missing func(string) bool
		output  string
		file    string
	}{
		{
			"done only keeps open tasks of missing branches", true, nil,
			"Removed 3 done tasks from %s:\n  Loose done\n  Gone done\n  Live done\n",
			"# Branches\n\n## feature/gone\n\n- [ ] Gone open\n\n## feature/live\n\n- [ ] Live open\n",
		},
		{
			"missing only keeps done tasks elsewhere", false, goneMissing,
			"Removed 2 tasks of missing branches from %s:\n  Gone open\n  Gone done\nBranches no longer in Git: feature/gone\n",
			"- [x] Loose done\n\n# Branches\n\n## feature/live\n\n- [ ] Live open\n\n- [x] Live done\n",
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte(missingContent), 0644); err != nil {
				t.Fatal(err)
			}
			var out strings.Builder
			if err := clearTasks(&out, path, item.done, item.missing); err != nil {
				t.Fatal(err)
			}
			if want := fmt.Sprintf(item.output, path); out.String() != want {
				t.Fatalf("output = %q, want %q", out.String(), want)
			}
			if got := fileContent(t, path); got != item.file {
				t.Fatalf("file = %q, want %q", got, item.file)
			}
		})
	}
}

func TestClearSaysWhatThereWasNothingOf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] Open\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		done    bool
		missing func(string) bool
		want    string
	}{
		{true, nil, "No done tasks in "},
		{false, goneMissing, "No branches missing from Git in "},
		{true, goneMissing, "No done tasks or branches missing from Git in "},
	} {
		var out strings.Builder
		if err := clearTasks(&out, path, item.done, item.missing); err != nil || out.String() != item.want+path+"\n" {
			t.Fatalf("clear = %q, %v; want %q", out.String(), err, item.want)
		}
	}
}

func TestClearMissingNeedsGit(t *testing.T) {
	if _, err := missingBranches(project.Context{}); err == nil {
		t.Fatal("missing branches were checked without Git")
	}
	repo := projecttest.Repo(t, t.TempDir(), "main", "feature/live")
	missing, err := missingBranches(repo)
	if err != nil || !missing("feature/gone") || missing("feature/live") || missing("") {
		t.Fatalf("missingBranches = %v", err)
	}
}

func TestClearReportPrintsControlCharactersAsSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# a\x1b]0;x\x07b\n\n- [x] Done \x1b[2Jtask\n\n# Branches\n\n## br\x07anch\n\n- [ ] Branch task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := clearTasks(&out, path, true, func(branch string) bool { return branch != "" }); err != nil {
		t.Fatal(err)
	}
	want := "Removed 1 done task and 1 task of a missing branch from " + path + ":\n  Done  [2Jtask\n  Branch task\n" +
		"Branches no longer in Git: br anch\nRemoved empty headings: @a ]0;x b\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}
