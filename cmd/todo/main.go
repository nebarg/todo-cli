// Command todo keeps tasks in a Markdown file, todo.md at the repository root
// by default, or an existing TODO.md. Without arguments it opens a terminal
// dashboard of the general tasks, the tasks of each Git branch, and the TODO
// comments in source files. Given task text, it adds a task; --clear-done and
// --clear-missing remove tasks.
package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/nebarg/todo-cli/internal/buildinfo"
	"github.com/nebarg/todo-cli/internal/dashboard"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
	"github.com/spf13/pflag"
)

// options are the command-line flags.
type options struct {
	file, priority, category, branch string
	clearDone, clearMissing          bool
	version                          bool
	excludes                         []string
}

// currentBranch is the -branch value meaning the current Git branch. Git
// rejects branch names starting with a dot, so it can't name a real one.
const currentBranch = "."

const usageText = `Usage:
  todo [flags]                   open the dashboard
  todo [flags] [@category] task  add a task
  todo [flags] --clear-done      remove done tasks
  todo [flags] --clear-missing   remove tasks of branches not in Git

Flags come first; everything after them is the task.

Flags:
`

// newFlags defines the flags, writing their values into o.
func newFlags(o *options) *pflag.FlagSet {
	flags := pflag.NewFlagSet("todo", pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.StringVarP(&o.priority, "priority", "p", "", "priority of a new task: `h|m|l`, or high, medium or low")
	flags.StringVarP(&o.category, "category", "c", "", "category `name` for a new task; quote a name with spaces")
	flags.StringVarP(&o.branch, "branch", "b", "", "Git branch `name` for a new task, or . for the current branch")
	flags.StringVarP(&o.file, "file", "f", "", "task file `path` (default todo.md at the repository root, or an existing TODO.md)")
	flags.BoolVar(&o.clearDone, "clear-done", false, "remove done tasks")
	flags.BoolVar(&o.clearMissing, "clear-missing", false, "remove every task, open ones included, of branches Git doesn't have locally, including ones not created yet")
	flags.BoolVar(&o.version, "version", false, "print the version")
	flags.StringArrayVarP(&o.excludes, "exclude", "e", nil, "skip a `dir` in the Files tab: a name at any depth, or a path from here; repeat for more (default node_modules and vendor)")
	return flags
}

type command int

const (
	commandDashboard command = iota
	commandAdd
	commandClear
)

// usageError is a command line that makes no sense, reported with exit
// status 2 rather than 1.
type usageError string

func (e usageError) Error() string { return string(e) }

// chooseCommand picks what to run from the flags and the words after them,
// rejecting flags that don't apply. Without a --clear flag, any
// words are a task, so a task can start with any word.
func chooseCommand(o options, args []string) (command, error) {
	taskFlags := o.priority != "" || o.category != "" || o.branch != ""
	switch {
	case o.clearDone || o.clearMissing:
		if len(args) > 0 || taskFlags || len(o.excludes) > 0 {
			return 0, usageError("usage: todo [-f file] [--clear-done] [--clear-missing]")
		}
		return commandClear, nil
	case len(args) > 0:
		if len(o.excludes) > 0 {
			return 0, usageError("--exclude is only for the dashboard")
		}
		return commandAdd, nil
	case taskFlags:
		return 0, usageError("-p, -c and -b need task text")
	}
	return commandDashboard, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs todo with the arguments after the program name, and returns its
// exit status: 2 for a usage error and 1 for any other.
func run(args []string, stdout, stderr io.Writer) int {
	err := runCommand(args, stdout)
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintln(stderr, err)
	if _, ok := errors.AsType[usageError](err); ok {
		return 2
	}
	return 1
}

// runCommand does what the command line asks, reporting on out.
func runCommand(argv []string, out io.Writer) error {
	var o options
	flags := newFlags(&o)
	flags.Usage = func() {
		_, _ = fmt.Fprint(out, usageText+flags.FlagUsages())
	}
	if err := flags.Parse(argv); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return usageError(err.Error() + "\nRun todo --help for usage")
	}
	if o.version {
		_, err := fmt.Fprintf(out, "todo %s\n", buildinfo.Version())
		return err
	}
	args := flags.Args()
	cmd, err := chooseCommand(o, args)
	if err != nil {
		return err
	}
	repo := project.Current()
	file := cmp.Or(o.file, repo.DefaultFile())
	switch cmd {
	case commandClear:
		var missing func(string) bool
		if o.clearMissing {
			if missing, err = missingBranches(repo); err != nil {
				return err
			}
		}
		return clearTasks(out, file, o.clearDone, missing)
	case commandAdd:
		return addTask(out, file, repo, o, args)
	}
	return runDashboard(out, file, repo, o.excludes)
}

// runDashboard opens the dashboard on out, which must be a terminal, with the
// Files tab scanning the working directory.
func runDashboard(out io.Writer, file string, repo project.Repo, excludes []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	exclude, err := scan.ParseExclude(cwd, cwd, excludes)
	if err != nil {
		return usageError(err.Error())
	}
	m, err := dashboard.New(file, repo, filesui.New(cwd, exclude, nil))
	if err != nil {
		return err
	}
	err = ui.Run(m, out)
	if errors.Is(err, ui.ErrNoTerminal) {
		return usageError("the dashboard needs a terminal; give a task to add, or use todo-scan --list")
	}
	return err
}

// addTask files the task in args under the category or branch the flags
// name, and reports it on out, naming a branch Git doesn't have as unknown
// so a mistyped name stands out.
func addTask(out io.Writer, file string, repo project.Repo, o options, args []string) error {
	category, args := splitCategoryArg(args)
	if len(args) == 0 {
		return usageError("usage: todo [flags] [@category] task")
	}
	if category != "" && o.category != "" {
		return usageError("use either @category or --category")
	}
	if category == "" {
		category = o.category
	}
	branch := o.branch
	if branch == currentBranch {
		branch = repo.CurrentBranch()
		if branch == "" {
			return errors.New("no current Git branch")
		}
	}
	p, err := store.ParsePriority(o.priority)
	if err != nil {
		return err
	}
	title := strings.Join(args, " ")
	if err := store.Add(file, title, "", p, store.Section{Category: category, Branch: branch}); err != nil {
		return err
	}
	if branch != "" && !repo.HasLocalBranch(branch) {
		_, err = fmt.Fprintf(out, "Added task on unknown branch %s: %s\n", ui.CleanDisplay(branch), title)
		return err
	}
	_, err = fmt.Fprintf(out, "Added task: %s\n", title)
	return err
}

// clearTasks removes every done task from file when done is set, and every
// task of a branch missing reports, with any headings left empty, and lists
// what went, with the control characters of the file's text as spaces so a
// terminal doesn't act on them. A nil missing leaves branches alone.
func clearTasks(out io.Writer, file string, done bool, missing func(string) bool) error {
	tasks, err := store.Load(file)
	if err != nil {
		return err
	}
	clearMissing := missing != nil
	if !clearMissing {
		missing = func(string) bool { return false }
	}
	if !done {
		tasks = slices.DeleteFunc(tasks, func(t store.Task) bool { return !missing(t.Branch) })
	}
	targets := store.PickClearTargets(tasks, missing)
	if len(targets.Tasks) == 0 {
		_, err := fmt.Fprintf(out, "No %s\n", nothingToClear(done, clearMissing))
		return err
	}
	removal, err := store.PlanRemove(file, targets.Tasks)
	if err != nil {
		return err
	}
	if err := removal.Apply(); err != nil {
		return err
	}
	var report strings.Builder
	fmt.Fprintf(&report, "Removed %s:\n", ui.CleanDisplay(ui.ClearSummary(targets)))
	for _, t := range slices.Backward(removal.Tasks) {
		fmt.Fprintf(&report, "  %s\n", ui.CleanDisplay(t.Text))
		for _, subtask := range tasks {
			if t.Holds(subtask) {
				fmt.Fprintf(&report, "    %s\n", ui.CleanDisplay(subtask.Text))
			}
		}
	}
	var headings []string
	for _, category := range removal.Categories {
		headings = append(headings, "@"+category)
	}
	for _, branch := range removal.Branches {
		if !slices.Contains(targets.Branches, branch) {
			headings = append(headings, branch)
		}
	}
	if len(headings) > 0 {
		fmt.Fprintf(&report, "Removed empty headings: %s\n", ui.CleanDisplay(strings.Join(headings, ", ")))
	}
	_, err = io.WriteString(out, report.String())
	return err
}

func nothingToClear(done, missing bool) string {
	switch {
	case done && missing:
		return "done tasks or unknown branches"
	case missing:
		return "unknown branches"
	}
	return "done tasks"
}

// missingBranches reports branches Git doesn't have locally. Without Git to
// ask, it fails rather than treat every branch as present.
func missingBranches(repo project.Repo) (func(string) bool, error) {
	branches, _, verified := repo.LocalBranchState()
	if !verified {
		return nil, errors.New("--clear-missing needs a Git repository to check branches against")
	}
	return func(branch string) bool {
		return branch != "" && !slices.Contains(branches, branch)
	}, nil
}

// splitCategoryArg takes a leading @category argument off the task text. Only
// the first argument counts, so an @ later in the text stays part of the title.
func splitCategoryArg(args []string) (string, []string) {
	if len(args) > 0 && len(args[0]) > 1 && strings.HasPrefix(args[0], "@") {
		return args[0], args[1:]
	}
	return "", args
}
