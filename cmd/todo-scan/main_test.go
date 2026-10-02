package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nebarg/todo-cli/internal/buildinfo"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/spf13/pflag"
)

func TestParseArgs(t *testing.T) {
	cwd := t.TempDir()
	sub := filepath.Join(cwd, "web")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "notes.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	defaults := scan.Exclude{Names: []string{"node_modules", "vendor"}}
	for _, c := range []struct {
		argv string
		want options
	}{
		{"", options{dir: cwd, exclude: defaults}},
		{"web", options{dir: sub, given: "web", exclude: defaults}},
		{"./web/", options{dir: sub, given: "./web/", exclude: defaults}},
		{".", options{dir: cwd, given: ".", exclude: defaults}},
		{"-e dist --exclude web/gen web", options{dir: sub, given: "web", exclude: scan.Exclude{Names: []string{"dist"}, Paths: []string{"gen"}}}},
		{sub, options{dir: sub, given: sub, exclude: defaults}},
		{"--list", options{dir: cwd, exclude: defaults, list: true}},
		{"--check web", options{dir: sub, given: "web", exclude: defaults, check: true}},
		{"--list --check --levels", options{dir: cwd, exclude: defaults, list: true, check: true, levels: true}},
		{"--list --level 0 --level 1", options{dir: cwd, exclude: defaults, list: true, level: []string{"0", "1"}}},
		{"--check --level 00,9", options{dir: cwd, exclude: defaults, check: true, level: []string{"00", "9"}}},
		{"--list --level 0+,1", options{dir: cwd, exclude: defaults, list: true, level: []string{"0+", "1"}}},
		{"--levels web", options{dir: sub, given: "web", exclude: defaults, levels: true}},
		{"--level 1", options{dir: cwd, exclude: defaults, level: []string{"1"}}},
		{"--version --level x missing", options{level: []string{"x"}, version: true}},
	} {
		t.Run(c.argv, func(t *testing.T) {
			got, err := parseArgs(cwd, strings.Fields(c.argv), io.Discard)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Fatalf("parseArgs = %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
	for _, argv := range []string{"a b", "--wat", "missing", "notes.txt", "--check=soon", "--list --level 12", "--list --level x", "--list --level=", "--list --level 1+", "--list --level 0*", "--list --level +"} {
		if _, err := parseArgs(cwd, strings.Fields(argv), io.Discard); err == nil {
			t.Errorf("parseArgs(%q) was accepted", argv)
		}
	}
	var usage strings.Builder
	if _, err := parseArgs(cwd, []string{"--help"}, &usage); !errors.Is(err, pflag.ErrHelp) || !strings.HasPrefix(usage.String(), "Usage:") {
		t.Fatalf("--help = %v, printing %q", err, usage.String())
	}
}

func TestReport(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.go":       "// TODO: tidy\nx := 1 // todo0 fix first\ny := 2 // todo00 fix before that\n",
		"b.go":       "// todo@ui split\n",
		"clean.go":   "package clean\n",
		"string.go":  `const s = "// TODO: not a comment"` + "\n",
		"web/app.js": "/* todo1 later */\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(o options) (string, string, int) {
		var out, errOut strings.Builder
		status := report(t.Context(), &out, &errOut, o)
		return out.String(), errOut.String(), status
	}
	all := "a.go:3: y := 2 // todo00 fix before that\na.go:2: x := 1 // todo0 fix first\nweb/app.js:1: /* todo1 later */\na.go:1: // TODO: tidy\nb.go:1: // todo@ui split\n"
	levelled := "a.go:3: y := 2 // todo00 fix before that\na.go:2: x := 1 // todo0 fix first\nweb/app.js:1: /* todo1 later */\n"
	for _, c := range []struct {
		name        string
		o           options
		out, errOut string
		status      int
	}{
		{"list", options{dir: dir, list: true}, all, "", exitClean},
		{"check", options{dir: dir, check: true}, "5 TODOs\n", "", exitFound},
		{"list and check", options{dir: dir, list: true, check: true}, all, "5 TODOs\n", exitFound},
		{"list levels", options{dir: dir, list: true, levels: true}, levelled, "", exitClean},
		{"check levels", options{dir: dir, check: true, levels: true}, "3 levelled TODOs\n", "", exitFound},
		{"list and check levels", options{dir: dir, list: true, check: true, levels: true}, levelled, "3 levelled TODOs\n", exitFound},
		{"list one level", options{dir: dir, list: true, level: []string{"0"}}, "a.go:2: x := 1 // todo0 fix first\n", "", exitClean},
		{"list and check two levels", options{dir: dir, list: true, check: true, level: []string{"00", "1"}}, "a.go:3: y := 2 // todo00 fix before that\nweb/app.js:1: /* todo1 later */\n", "2 levelled TODOs\n", exitFound},
		{"list any zeros", options{dir: dir, list: true, level: []string{"0+"}}, "a.go:3: y := 2 // todo00 fix before that\na.go:2: x := 1 // todo0 fix first\n", "", exitClean},
		{"check any zeros and todo1", options{dir: dir, check: true, level: []string{"0+", "1"}}, "3 levelled TODOs\n", "", exitFound},
		{"check a level nobody used", options{dir: dir, check: true, level: []string{"5"}}, "0 levelled TODOs\n", "", exitClean},
		{"check with exclusions", options{dir: dir, check: true, exclude: scan.Exclude{Names: []string{"web"}}}, "4 TODOs\n", "", exitFound},
		{"list from a directory given as a relative path", options{dir: dir, given: "../proj/", list: true, levels: true}, "../proj/a.go:3: y := 2 // todo00 fix before that\n../proj/a.go:2: x := 1 // todo0 fix first\n../proj/web/app.js:1: /* todo1 later */\n", "", exitClean},
		{"list from a directory given as an absolute path", options{dir: dir, given: dir, list: true, level: []string{"1"}}, filepath.ToSlash(dir) + "/web/app.js:1: /* todo1 later */\n", "", exitClean},
		{"list from the current directory given as .", options{dir: dir, given: ".", list: true}, all, "", exitClean},
		{"check from a directory given", options{dir: dir, given: "../proj", check: true}, "5 TODOs\n", "", exitFound},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, errOut, status := run(c.o)
			if out != c.out || errOut != c.errOut || status != c.status {
				t.Fatalf("got %q, %q, %d\nwant %q, %q, %d", out, errOut, status, c.out, c.errOut, c.status)
			}
		})
	}

	clean := t.TempDir()
	if err := os.WriteFile(filepath.Join(clean, "a.go"), []byte("// TODO: one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, errOut, status := run(options{dir: clean, check: true, levels: true}); out != "0 levelled TODOs\n" || errOut != "" || status != exitClean {
		t.Fatalf("levels check without levels = %q, %q, %d", out, errOut, status)
	}
	if err := os.Remove(filepath.Join(clean, "a.go")); err != nil {
		t.Fatal(err)
	}
	if out, errOut, status := run(options{dir: clean, list: true, check: true}); out != "" || errOut != "0 TODOs\n" || status != exitClean {
		t.Fatalf("clean check = %q, %q, %d", out, errOut, status)
	}
	if _, errOut, status := run(options{dir: filepath.Join(clean, "gone"), check: true}); errOut == "" || status != exitError {
		t.Fatalf("a failed scan = %q, %d", errOut, status)
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"a.go": "// todo0 first\n", "sub/b.go": "// TODO: later\n"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	version := "todo-scan " + buildinfo.Version() + "\n"
	for _, c := range []struct {
		args        string
		out, errOut string
		status      int
	}{
		{"--version", version, "", exitClean},
		{"--version missing", version, "", exitClean},
		{"--version --list --level 12", version, "", exitClean},
		{"--list", "a.go:1: // todo0 first\nsub/b.go:1: // TODO: later\n", "", exitClean},
		{"--list sub", "sub/b.go:1: // TODO: later\n", "", exitClean},
		{"--list --check --levels", "a.go:1: // todo0 first\n", "1 levelled TODO\n", exitFound},
		{"--check -e sub", "1 TODO\n", "", exitFound},
		{"--wat", "", "unknown flag: --wat\nRun todo-scan --help for usage\n", exitError},
		{"--list missing", "", "stat " + filepath.Join(dir, "missing") + ": no such file or directory\n", exitError},
		{"", "", "the browser needs a terminal; use --list or --check\n", exitError},
	} {
		t.Run(c.args, func(t *testing.T) {
			var out, errOut strings.Builder
			status := run(t.Context(), strings.Fields(c.args), &out, &errOut)
			if out.String() != c.out || errOut.String() != c.errOut || status != c.status {
				t.Fatalf("got %q, %q, %d\nwant %q, %q, %d", out.String(), errOut.String(), status, c.out, c.errOut, c.status)
			}
		})
	}

	var out, errOut strings.Builder
	if status := run(t.Context(), []string{"--help"}, &out, &errOut); status != exitClean || !strings.HasPrefix(out.String(), "Usage:") || errOut.Len() > 0 {
		t.Fatalf("--help = %d, %q, %q", status, out.String(), errOut.String())
	}
}
