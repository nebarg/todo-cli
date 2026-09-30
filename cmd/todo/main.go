package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
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
	flag.StringVar(categoryFlag, "l", "", "")
	flag.StringVar(categoryFlag, "label", "", "") // Accept older command lines.
	flag.StringVar(categoryFlag, "labels", "", "")
	flag.BoolVar(onBranch, "b", false, "shorthand for -branch")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [task text]\n\nFlags:\n", os.Args[0])
		flag.VisitAll(func(option *flag.Flag) {
			if option.Name == "l" || option.Name == "label" || option.Name == "labels" {
				return
			}
			fmt.Fprintf(flag.CommandLine.Output(), "  -%s\t%s\n", option.Name, option.Usage)
		})
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
		matches, err := scanSource(absolute, 0, *allFiles)
		if err != nil {
			fail(err)
		}
		for _, match := range matches {
			fmt.Printf("%s:%d: %s\n", match.path, match.line, match.text)
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
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "usage: todo [flags] task text")
			os.Exit(2)
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
		p, err := parsePriority(*priorityFlag)
		if err != nil {
			fail(err)
		}
		title := strings.Join(args, " ")
		var categories []string
		if *categoryFlag != "" {
			categories = []string{*categoryFlag}
		}
		if err := addTaskWithOptions(file, title, p, categories, branch); err != nil {
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
