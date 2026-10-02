// Command todo-scan browses the TODO comments in source files under a
// directory, without the task tabs of todo. With --list or --check it prints
// them instead, for scripts and CI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/ui"
	"github.com/spf13/pflag"
)

const usageText = `Usage:
  todo-scan [flags] [directory]    browse TODO comments under the directory (default here)

--list prints the TODOs, and --check counts them, exiting 1 if there are
any. Give both for the list and the exit status. Errors exit 2.

Flags:
`

// Exit statuses follow grep, so CI can tell TODOs from a broken check.
const (
	exitClean = 0
	exitFound = 1
	exitError = 2
)

type options struct {
	dir     string
	given   string // the directory as the command line gave it, which starts each --list path
	exclude scan.Exclude
	list    bool
	check   bool
	levels  bool
	level   []string
}

// parseArgs reads the command line: the directory to scan, as given and made
// absolute, what to skip in it, and whether to print rather than browse.
// --help prints the usage on out.
func parseArgs(cwd string, argv []string, out io.Writer) (options, error) {
	var o options
	var excludes []string
	flags := pflag.NewFlagSet("todo-scan", pflag.ContinueOnError)
	flags.StringArrayVarP(&excludes, "exclude", "e", nil, "skip a `dir` when scanning: a name at any depth, or a path from here; repeat for more (default node_modules and vendor)")
	flags.BoolVar(&o.list, "list", false, "print the TODOs as path:line: text, most urgent first")
	flags.BoolVar(&o.check, "check", false, "print how many TODOs there are, and exit 1 if there are any")
	flags.BoolVar(&o.levels, "levels", false, "only todo-system's levelled TODOs, todo0 to todo9")
	flags.StringSliceVar(&o.level, "level", nil, "only TODOs at this `level`, such as 0 for todo0, 00 for todo00, or 0+ for any number of zeros; repeat for more")
	flags.Usage = func() {
		_, _ = fmt.Fprint(out, usageText+flags.FlagUsages())
	}
	if err := flags.Parse(argv); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return o, err
		}
		return o, fmt.Errorf("%w\nRun todo-scan --help for usage", err)
	}
	if flags.Changed("level") && len(o.level) == 0 {
		return o, errors.New("--level needs a level, such as 0 or 1")
	}
	for _, level := range o.level {
		if !scan.ValidLevel(level) {
			return o, fmt.Errorf("--level %q is not a todo-system level: use one digit, such as 1, only zeros, such as 00, or 0+ for any zeros", level)
		}
	}
	o.dir = cwd
	switch args := flags.Args(); len(args) {
	case 0:
	case 1:
		o.dir, o.given = args[0], args[0]
		if !filepath.IsAbs(o.dir) {
			o.dir = filepath.Join(cwd, o.dir)
		}
	default:
		return o, errors.New("usage: todo-scan [flags] [directory]")
	}
	if info, err := os.Stat(o.dir); err != nil {
		return o, err
	} else if !info.IsDir() {
		return o, fmt.Errorf("not a directory: %s", o.dir)
	}
	var err error
	o.exclude, err = scan.ParseExclude(cwd, o.dir, excludes)
	return o, err
}

// report prints the TODOs for --list and their count for --check, and
// returns the exit status. With both, the count goes to errOut, so out stays
// a list a script can read.
func report(ctx context.Context, out, errOut io.Writer, o options) int {
	matches, err := scan.Source(ctx, o.dir, o.exclude)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return exitError
	}
	if keep := scan.LevelFilter(o.levels, o.level); keep != nil {
		matches = slices.DeleteFunc(matches, func(match scan.Match) bool { return !keep(match) })
	}
	if o.list {
		for _, match := range matches {
			_, _ = fmt.Fprintf(out, "%s:%d: %s\n", path.Join(filepath.ToSlash(o.given), match.Path), match.Line, match.Text)
		}
	}
	if !o.check {
		return exitClean
	}
	one, many := "TODO", "TODOs"
	if o.levels || len(o.level) > 0 {
		one, many = "levelled TODO", "levelled TODOs"
	}
	countOut := out
	if o.list {
		countOut = errOut
	}
	_, _ = fmt.Fprintln(countOut, ui.Plural(len(matches), one, many))
	if len(matches) > 0 {
		return exitFound
	}
	return exitClean
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run runs todo-scan with the arguments after the program name, and returns
// its exit status.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, err)
	}
	o, err := parseArgs(cwd, args, stdout)
	if errors.Is(err, pflag.ErrHelp) {
		return exitClean
	}
	if err != nil {
		return fail(stderr, err)
	}
	if o.list || o.check {
		return report(ctx, stdout, stderr, o)
	}
	err = ui.Run(newModel(o.dir, o.exclude, scan.LevelFilter(o.levels, o.level)), stdout)
	if errors.Is(err, ui.ErrNoTerminal) {
		err = errors.New("the browser needs a terminal; use --list or --check")
	}
	if err != nil {
		return fail(stderr, err)
	}
	return exitClean
}

// fail reports err and returns the exit status for an error.
func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return exitError
}
