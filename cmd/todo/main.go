package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
)

func main() {
	fileFlag := flag.String("file", "", "markdown file (default: repository root/todo.md)")
	priorityFlag := flag.String("priority", "", "high, medium, or low for a new task")
	categoryFlag := flag.String("category", "", "one category for a new task")
	onBranch := flag.Bool("branch", false, "put a new task under the current Git branch")
	branchName := flag.String("branch-name", "", "put a new task under this existing local Git branch")
	allFiles := flag.Bool("all-files", false, "include Markdown, hidden, and ignored text in scan")
	flag.StringVar(priorityFlag, "p", "", "shorthand for -priority")
	flag.StringVar(categoryFlag, "c", "", "shorthand for -category")
	flag.BoolVar(onBranch, "b", false, "shorthand for -branch")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [@category] [task text]\n       %s [-file path] clear-done\n\nFlags:\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	project := currentProject()
	file := *fileFlag
	if file == "" {
		file = defaultFile(project)
	}
	args := flag.Args()

	if len(args) > 0 && args[0] == "scan" {
		if len(args) > 2 {
			fmt.Fprintln(os.Stderr, "usage: todo scan [directory]")
			os.Exit(2)
		}
		dir := project.root
		if dir == "" {
			dir = "."
		}
		if len(args) == 2 {
			dir = args[1]
		}
		absolute, err := filepath.Abs(dir)
		if err != nil {
			fail(err)
		}
		matches, err := scan.Source(absolute, 0, *allFiles)
		if err != nil {
			fail(err)
		}
		for _, match := range matches {
			fmt.Printf("%s:%d: %s\n", match.Path, match.Line, match.Text)
		}
		return
	}

	if len(args) > 0 && args[0] == "clear-done" {
		if len(args) > 1 || *priorityFlag != "" || *categoryFlag != "" || *onBranch || *branchName != "" || *allFiles {
			fmt.Fprintln(os.Stderr, "usage: todo [-file path] clear-done")
			os.Exit(2)
		}
		if err := clearDone(os.Stdout, file, missingBranches(project)); err != nil {
			fail(err)
		}
		return
	}

	if len(args) > 0 {
		if *allFiles {
			fmt.Fprintln(os.Stderr, "-all-files is only for scan")
			os.Exit(2)
		}
		if args[0] == "add" {
			args = args[1:]
		}
		var category string
		category, args = splitCategoryArg(args)
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "usage: todo [flags] [@category] task text")
			os.Exit(2)
		}
		if category != "" && *categoryFlag != "" {
			fail(fmt.Errorf("use either @category or -category"))
		}
		if category == "" {
			category = *categoryFlag
		}
		if *onBranch && *branchName != "" {
			fail(fmt.Errorf("use either -branch or -branch-name"))
		}
		branch := *branchName
		if *onBranch {
			branch = project.currentBranch()
			if branch == "" {
				fail(fmt.Errorf("no current Git branch"))
			}
		}
		if branch != "" && !project.hasLocalBranch(branch) {
			fail(fmt.Errorf("branch %q does not exist locally", branch))
		}
		p, err := store.ParsePriority(*priorityFlag)
		if err != nil {
			fail(err)
		}
		title := strings.Join(args, " ")
		if err := store.Add(file, title, "", p, category, branch); err != nil {
			fail(err)
		}
		fmt.Printf("Added to %s: %s\n", file, title)
		return
	}
	if *priorityFlag != "" || *categoryFlag != "" || *onBranch || *branchName != "" || *allFiles {
		fmt.Fprintln(os.Stderr, "task flags need task text")
		os.Exit(2)
	}
	m, err := newModel(file, project)
	if err != nil {
		fail(err)
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fail(err)
	}
}

// clearDone removes every done task from file, and every task of a branch
// missing from Git, with any headings left empty, and lists what went.
func clearDone(out io.Writer, file string, missing func(string) bool) error {
	tasks, err := store.Load(file)
	if err != nil {
		return err
	}
	targets := pickClearTargets(tasks, missing)
	if len(targets.tasks) == 0 {
		_, err := fmt.Fprintf(out, "No done tasks in %s\n", file)
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
		fmt.Fprintf(&report, "Deleted branches missing from Git: %s\n", strings.Join(targets.branches, ", "))
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

// missingBranches reports branches Git no longer has; without Git to ask,
// nothing counts as missing.
func missingBranches(project projectContext) func(string) bool {
	branches, _, verified := project.localBranchState()
	return func(branch string) bool {
		return verified && branch != "" && !slices.Contains(branches, branch)
	}
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
	os.Exit(1)
}
