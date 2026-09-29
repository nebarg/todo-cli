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
	priority := flag.String("priority", "", "high, medium, or low for a new task")
	labelsFlag := flag.String("label", "", "one label for a new task")
	onBranch := flag.Bool("branch", false, "put a new task under the current Git branch")
	branchName := flag.String("branch-name", "", "put a new task under this branch heading")
	allFiles := flag.Bool("all-files", false, "include Markdown, hidden, and ignored text in scan")
	flag.StringVar(priority, "p", "", "shorthand for -priority")
	flag.StringVar(labelsFlag, "l", "", "shorthand for -label")
	flag.StringVar(labelsFlag, "labels", "", "alias for -label")
	flag.BoolVar(onBranch, "b", false, "shorthand for -branch")
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
			branch = project.branch
			if branch == "" {
				fail(fmt.Errorf("no current Git branch; use -branch-name to choose one"))
			}
		}
		title := strings.Join(args, " ")
		var labels []string
		if strings.TrimSpace(*labelsFlag) != "" {
			labels = []string{*labelsFlag}
		}
		if err := addTaskWithOptions(file, title, *priority, labels, branch); err != nil {
			fail(err)
		}
		fmt.Printf("Added to %s: %s\n", file, title)
		return
	}
	if *priority != "" || *labelsFlag != "" || *onBranch || *branchName != "" || *allFiles {
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
