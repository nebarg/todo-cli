package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/spf13/pflag"
)

// options are the command-line flags.
type options struct {
	file, priority, category, branch string
	scan, allFiles                   bool
	clearDone, clearMissing          bool
	excludes                         []string
}

// currentBranch is the -branch value meaning the current Git branch. Git
// rejects branch names starting with a dot, so it can't name a real one.
const currentBranch = "."

const usageText = `Usage:
  todo [flags]                     open the dashboard
  todo [flags] [@category] task    add a task
  todo [flags] --scan [directory]  list TODO comments in source files
  todo [flags] --clear-done        remove done tasks
  todo [flags] --clear-missing     remove tasks of branches no longer in Git

Flags come first; everything after them is the task.

Flags:
`

// newFlags defines the flags, writing their values into o.
func newFlags(o *options) *pflag.FlagSet {
	flags := pflag.NewFlagSet("todo", pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.StringVarP(&o.priority, "priority", "p", "", "priority of a new task: `h|m|l`, or high, medium or low")
	flags.StringVarP(&o.category, "category", "c", "", "category `name` for a new task")
	flags.StringVarP(&o.branch, "branch", "b", "", "local Git branch `name` for a new task, or . for the current branch")
	flags.StringVarP(&o.file, "file", "f", "", "task file `path` (default todo.md at the repository root)")
	flags.BoolVar(&o.scan, "scan", false, "list TODO comments in source files under a directory (default here)")
	flags.BoolVar(&o.clearDone, "clear-done", false, "remove done tasks")
	flags.BoolVar(&o.clearMissing, "clear-missing", false, "remove every task of branches no longer in Git, open ones included")
	flags.StringArrayVarP(&o.excludes, "exclude", "e", nil, "skip a `dir` when scanning: a name at any depth, or a path from here; repeat for more (default node_modules and vendor)")
	flags.BoolVar(&o.allFiles, "all-files", false, "with --scan, include Markdown, hidden and ignored files")
	return flags
}

type command int

const (
	commandDashboard command = iota
	commandAdd
	commandScan
	commandClear
)

// usageError is a command line that makes no sense, reported with exit
// status 2 rather than 1.
type usageError string

func (e usageError) Error() string { return string(e) }

// chooseCommand picks what to run from the flags and the words after them,
// rejecting flags that don't apply. Without --scan or a --clear flag, any
// words are a task, so a task can start with any word.
func chooseCommand(o options, args []string) (command, error) {
	taskFlags := o.priority != "" || o.category != "" || o.branch != ""
	clear := o.clearDone || o.clearMissing
	switch {
	case o.scan && clear:
		return 0, usageError("use either --scan or --clear-done and --clear-missing")
	case o.scan:
		if len(args) > 1 || taskFlags {
			return 0, usageError("usage: todo [--all-files] [-e dir]... --scan [directory]")
		}
		return commandScan, nil
	case o.allFiles:
		return 0, usageError("--all-files is only for --scan")
	case clear:
		if len(args) > 0 || taskFlags || len(o.excludes) > 0 {
			return 0, usageError("usage: todo [-f file] [--clear-done] [--clear-missing]")
		}
		return commandClear, nil
	case len(args) > 0:
		if len(o.excludes) > 0 {
			return 0, usageError("--exclude is only for --scan and the dashboard")
		}
		return commandAdd, nil
	case taskFlags:
		return 0, usageError("-p, -c and -b need task text")
	}
	return commandDashboard, nil
}

func main() {
	var o options
	flags := newFlags(&o)
	flags.Usage = func() {
		_, _ = fmt.Fprint(os.Stdout, usageText+flags.FlagUsages())
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return
		}
		fail(usageError(err.Error() + "\nRun todo --help for usage."))
	}
	args := flags.Args()
	cmd, err := chooseCommand(o, args)
	if err != nil {
		fail(err)
	}
	project := currentProject()
	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	file := o.file
	if file == "" {
		file = defaultFile(project)
	}

	switch cmd {
	case commandScan:
		dir := cwd
		if len(args) == 1 {
			if dir, err = filepath.Abs(args[0]); err != nil {
				fail(err)
			}
		}
		exclude, err := scanExclude(cwd, dir, o.excludes)
		if err != nil {
			fail(err)
		}
		matches, err := scan.Source(dir, 0, o.allFiles, exclude)
		if err != nil {
			fail(err)
		}
		for _, match := range matches {
			fmt.Printf("%s:%d: %s\n", match.Path, match.Line, match.Text)
		}
	case commandClear:
		var missing func(string) bool
		if o.clearMissing {
			if missing, err = missingBranches(project); err != nil {
				fail(err)
			}
		}
		if err := clearTasks(os.Stdout, file, o.clearDone, missing); err != nil {
			fail(err)
		}
	case commandAdd:
		if err := addTask(file, project, o, args); err != nil {
			fail(err)
		}
	case commandDashboard:
		m, err := newModel(file, project)
		if err != nil {
			fail(err)
		}
		exclude, err := scanExclude(cwd, cwd, o.excludes)
		if err != nil {
			fail(err)
		}
		m.files = filesui.New(cwd, exclude)
		if _, err := tea.NewProgram(m).Run(); err != nil {
			fail(err)
		}
	}
}

// addTask files the task in args under the category or branch the flags
// name, and reports where it went.
func addTask(file string, project projectContext, o options, args []string) error {
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
		branch = project.currentBranch()
		if branch == "" {
			return errors.New("no current Git branch")
		}
	}
	if branch != "" && !project.hasLocalBranch(branch) {
		return fmt.Errorf("branch %q does not exist locally", branch)
	}
	p, err := store.ParsePriority(o.priority)
	if err != nil {
		return err
	}
	title := strings.Join(args, " ")
	if err := store.Add(file, title, "", p, category, branch); err != nil {
		return err
	}
	fmt.Printf("Added to %s: %s\n", file, title)
	return nil
}

// clearTasks removes every done task from file when done is set, and every
// task of a branch missing reports, with any headings left empty, and lists
// what went. A nil missing leaves branches alone.
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
	targets := pickClearTargets(tasks, missing)
	if len(targets.tasks) == 0 {
		_, err := fmt.Fprintf(out, "No %s in %s\n", nothingToClear(done, clearMissing), file)
		return err
	}
	removal, err := store.PlanRemove(file, targets.tasks)
	if err != nil {
		return err
	}
	if err := removal.Apply(); err != nil {
		return err
	}
	var report strings.Builder
	fmt.Fprintf(&report, "Removed %s from %s:\n", targets.summary(), file)
	for _, t := range slices.Backward(removal.Tasks) {
		fmt.Fprintf(&report, "  %s\n", t.Text)
	}
	if len(targets.branches) > 0 {
		fmt.Fprintf(&report, "Branches no longer in Git: %s\n", strings.Join(targets.branches, ", "))
	}
	var headings []string
	for _, category := range removal.Categories {
		headings = append(headings, "@"+category)
	}
	for _, branch := range removal.Branches {
		if !slices.Contains(targets.branches, branch) {
			headings = append(headings, branch)
		}
	}
	if len(headings) > 0 {
		fmt.Fprintf(&report, "Removed empty headings: %s\n", strings.Join(headings, ", "))
	}
	_, err = io.WriteString(out, report.String())
	return err
}

func nothingToClear(done, missing bool) string {
	switch {
	case done && missing:
		return "done tasks or branches missing from Git"
	case missing:
		return "branches missing from Git"
	}
	return "done tasks"
}

// missingBranches reports branches Git no longer has. Without Git to ask,
// it fails rather than treat every branch as present.
func missingBranches(project projectContext) (func(string) bool, error) {
	branches, _, verified := project.localBranchState()
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	if _, ok := errors.AsType[usageError](err); ok {
		os.Exit(2)
	}
	os.Exit(1)
}
