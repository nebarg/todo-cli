// Command todo-scan browses the TODO comments in source files under a
// directory, without the task tabs of todo.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/spf13/pflag"
)

const usageText = `Usage:
  todo-scan [flags] [directory]    browse TODO comments under the directory (default here)

Flags:
`

// usageError is a command line that makes no sense, reported with exit
// status 2 rather than 1.
type usageError string

func (e usageError) Error() string { return string(e) }

// parseArgs reads the command line: the directory to scan, made absolute,
// and what to skip in it.
func parseArgs(cwd string, argv []string) (string, scan.Exclude, error) {
	var excludes []string
	flags := pflag.NewFlagSet("todo-scan", pflag.ContinueOnError)
	flags.StringArrayVarP(&excludes, "exclude", "e", nil, "skip a `dir` when scanning: a name at any depth, or a path from here; repeat for more (default node_modules and vendor)")
	flags.Usage = func() {
		_, _ = fmt.Fprint(os.Stdout, usageText+flags.FlagUsages())
	}
	if err := flags.Parse(argv); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return "", scan.Exclude{}, err
		}
		return "", scan.Exclude{}, usageError(err.Error() + "\nRun todo-scan --help for usage.")
	}
	dir := cwd
	switch args := flags.Args(); len(args) {
	case 0:
	case 1:
		dir = args[0]
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
	default:
		return "", scan.Exclude{}, usageError("usage: todo-scan [flags] [directory]")
	}
	if info, err := os.Stat(dir); err != nil {
		return "", scan.Exclude{}, err
	} else if !info.IsDir() {
		return "", scan.Exclude{}, fmt.Errorf("not a directory: %s", dir)
	}
	exclude, err := scan.ParseExclude(cwd, dir, excludes)
	return dir, exclude, err
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	dir, exclude, err := parseArgs(cwd, os.Args[1:])
	if errors.Is(err, pflag.ErrHelp) {
		return
	}
	if err != nil {
		fail(err)
	}
	if _, err := tea.NewProgram(newModel(dir, exclude)).Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	if _, ok := errors.AsType[usageError](err); ok {
		os.Exit(2)
	}
	os.Exit(1)
}
