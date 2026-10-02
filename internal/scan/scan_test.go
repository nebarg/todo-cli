package scan

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var defaultExclude = Exclude{Names: []string{"node_modules", "vendor"}}

func TestScanSource(t *testing.T) {
	files := map[string]string{
		"code.go":               "// @todo fix this\nconst name = \"todo scan\"\n// TODO: test it\n",
		"TODO.md":               "- [ ] TODO: stored task\n",
		"notes.md":              "Plain text TODO: not a comment\n<!-- TODO: in a comment -->\n",
		".hidden.go":            "// TODO: hidden file\n",
		"web/.eslintrc.js":      "// TODO: nested hidden file\n",
		"binary.dat":            "TODO\x00x",
		".cache/cached.go":      "// TODO: dot directory\n",
		"src/.venv/lib.go":      "// TODO: nested dot directory\n",
		"node_modules/dep.js":   "// TODO: dependency\n",
		"web/node_modules/d.js": "// TODO: nested dependency\n",
		"vendor/lib.go":         "// TODO: vendored\n",
		"dist/out.js":           "// TODO: top-level dist\n",
		"pkg/dist/out.js":       "// TODO: package dist\n",
		"pkg/gen/gen.go":        "// TODO: generated\n",
		"odd[1]/x.go":           "// TODO: glob characters\n",
		"sys.php":               "// todo@boundary Split this\n// todo00 First\n// todo1@boundary Split more\n",
	}
	cases := []struct {
		name    string
		exclude Exclude
		want    []string
	}{
		{"defaults read hidden files, but skip dependencies, dot directories and Markdown", defaultExclude,
			[]string{"sys.php:2", ".hidden.go:1", "code.go:1", "code.go:3", "dist/out.js:1", "odd[1]/x.go:1", "pkg/dist/out.js:1", "pkg/gen/gen.go:1", "sys.php:1", "sys.php:3", "web/.eslintrc.js:1"}},
		{"custom excludes replace the defaults", Exclude{Names: []string{"gen", "odd[1]"}, Paths: []string{"pkg/dist"}},
			[]string{"sys.php:2", ".hidden.go:1", "code.go:1", "code.go:3", "dist/out.js:1", "node_modules/dep.js:1", "sys.php:1", "sys.php:3", "vendor/lib.go:1", "web/.eslintrc.js:1", "web/node_modules/d.js:1"}},
	}
	for _, scanner := range scanners(t) {
		t.Run(scanner.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, files)
			if scanner.git {
				gitInit(t, dir)
			}
			scanner.use(t)
			for _, c := range cases {
				matches, err := Source(t.Context(), dir, c.exclude)
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, m := range matches {
					got = append(got, fmt.Sprintf("%s:%d", m.Path, m.Line))
				}
				if !slices.Equal(got, c.want) {
					t.Errorf("%s:\n got  %v\n want %v", c.name, got, c.want)
				}
			}
		})
	}
}

func TestScanSkipsUnreadableDirectories(t *testing.T) {
	for _, scanner := range scanners(t) {
		t.Run(scanner.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"ok/a.go": "// TODO: readable\n", "locked/b.go": "// TODO: unreadable\n"})
			locked := filepath.Join(dir, "locked")
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0755) }) // So TempDir can remove it.
			if _, err := os.ReadDir(locked); err == nil {
				t.Skip("directory permissions aren't enforced for this user")
			}
			if scanner.git {
				gitInit(t, dir)
			}
			scanner.use(t)
			matches, err := Source(t.Context(), dir, Exclude{})
			if err != nil || len(matches) != 1 || matches[0].Path != "ok/a.go" {
				t.Fatalf("matches = %+v, %v", matches, err)
			}
			if scanner.ripgrep {
				// Source falls back to the built-in scanner when ripgrep fails.
				matches, err := scanWithRipgrep(t.Context(), dir, Exclude{})
				if err != nil || len(matches) != 1 {
					t.Fatalf("ripgrep matches = %+v, %v", matches, err)
				}
			}
		})
	}
}

func TestScanReadsADirectoryItsRepositoryIgnores(t *testing.T) {
	for _, scanner := range scanners(t) {
		t.Run(scanner.name, func(t *testing.T) {
			repo := t.TempDir()
			writeFiles(t, repo, map[string]string{".gitignore": "/*\n", "ignored/a.go": "// TODO: in an ignored directory\n"})
			gitInit(t, repo)
			scanner.use(t)
			matches, err := Source(t.Context(), filepath.Join(repo, "ignored"), Exclude{})
			if err != nil || len(matches) != 1 || matches[0].Path != "a.go" {
				t.Fatalf("matches = %+v, %v", matches, err)
			}
		})
	}
}

func TestScanStopsWhenCancelled(t *testing.T) {
	for _, scanner := range scanners(t) {
		t.Run(scanner.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"a.go": "// TODO: never read\n"})
			if scanner.git {
				gitInit(t, dir)
			}
			scanner.use(t)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if matches, err := Source(ctx, dir, Exclude{}); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled scan = %+v, %v", matches, err)
			}
		})
	}
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
}

type scanner struct {
	name    string
	path    string
	git     bool
	ripgrep bool
}

// use puts only the scanner's tools on PATH, and keeps the user's Git
// configuration out of the test.
func (s scanner) use(t *testing.T) {
	t.Setenv("PATH", s.path)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull) // A global gitignore would hide test files.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// scanners runs a test with the built-in scanner, with and without Git to
// list files, and with ripgrep when it is installed.
func scanners(t *testing.T) []scanner {
	gitOnly := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(gitOnly, "git")); err != nil {
		t.Fatal(err)
	}
	list := []scanner{{name: "built-in", path: t.TempDir()}, {name: "built-in with git", path: gitOnly, git: true}}
	if rg, err := exec.LookPath("rg"); err == nil {
		list = append(list, scanner{name: "ripgrep", path: filepath.Dir(rg), ripgrep: true})
	} else {
		t.Log("ripgrep not installed; skipping its scanner")
	}
	return list
}

func TestSortedMatchesByFileAndLine(t *testing.T) {
	matches := []Match{{Path: "b.go", Line: 1}, {Path: "a.go", Line: 9}, {Path: "a.go", Line: 2}, {Path: "c.go", Line: 1}}
	want := []Match{{Path: "a.go", Line: 2}, {Path: "a.go", Line: 9}, {Path: "b.go", Line: 1}, {Path: "c.go", Line: 1}}
	if got := sortedMatches(matches); !slices.Equal(got, want) {
		t.Fatalf("sorted = %+v", got)
	}
	if got := sortedMatches(nil); len(got) != 0 {
		t.Fatalf("empty scan = %+v", got)
	}
}

func TestBuiltInScanIsDeterministic(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // Force the built-in scanner.
	dir := t.TempDir()
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("// TODO: item\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		matches, err := Source(t.Context(), dir, Exclude{})
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 40 || !slices.IsSortedFunc(matches, func(a, b Match) int { return strings.Compare(a.Path, b.Path) }) {
			t.Fatalf("scan order = %+v", matches)
		}
	}
}

func TestTodoComments(t *testing.T) {
	for _, c := range []struct {
		line                  string
		note, category, level string
		found                 bool
	}{
		{"// TODO: fix this", "fix this", "", "", true},
		{"//TODO make this smarter with postcode", "make this smarter with postcode", "", "", true},
		{"* @todo Make the 'x' configurable", "Make the 'x' configurable", "", "", true},
		{"-- TODO check int size for id", "check int size for id", "", "", true},
		{"/* TODO in future */", "in future", "", "", true},
		{"<!-- todo: tidy -->", "tidy", "", "", true},
		{"# TODO(gb): rename", "rename", "", "", true},
		{"// TODO - later", "later", "", "", true},
		{"// TODO! fix this", "fix this", "", "", true},
		{"x(); //TODO!", "", "", "", true},
		{"/** TODO document */", "document", "", "", true},
		{"$x = load(); // TODO: cache", "cache", "", "", true},
		{"// ported from the old code, TODO: remove", "remove", "", "", true},
		{"// TODO", "", "", "", true},
		{"// todo@responsivity Hide this on mobile", "Hide this on mobile", "responsivity", "", true},
		{"{{-- todo@dark-mode: input styling --}}", "input styling", "dark-mode", "", true},
		{"// todo000 solve first", "solve first", "", "000", true},
		{"// needed before merging todo0", "", "", "0", true},
		{"# todo7: later", "later", "", "7", true},
		{"// todo1@boundary split this", "split this", "boundary", "", true},
		{"// todo12 not a todo-system level", "not a todo-system level", "", "", true},
		{"// see todo11: no level, but counts before a colon", "no level, but counts before a colon", "", "", true},
		{"// see todo11 later", "", "", "", false},
		{"// see todo11, then todo0 counts", "counts", "", "0", true},
		{"// todo@types: add types", "add types", "types", "", true},
		{"// @ todo Lookup ID for this cat.", "Lookup ID for this cat.", "", "", true},
		{"// $x: var(--y) !default; // Todo in v6: remove this?", "in v6: remove this?", "", "", true},
		{"# retries = 3  # TODO tune", "tune", "", "", true},
		{"/* old(); /* TODO restore */", "restore", "", "", true},
		{"// see http://todo.example for details", "", "", "", false},
		{"// x = a//todo", "", "", "", false},
		{"// email me @ todo later", "", "", "", false},
		{"* @return todo", "", "", "", false},
		{"// maybe todo later", "", "", "", false},
		{`const name = "// TODO: not a comment"`, "", "", "", false},
		{"// todos are done", "", "", "", false},
		{"// todo2x is not a level", "", "", "", false},
	} {
		got, found := commentTodo(c.line)
		want := todoComment{note: c.note, category: c.category, level: c.level}
		if got != want || found != c.found {
			t.Errorf("commentTodo(%q) = %+v, %v; want %+v, %v", c.line, got, found, want, c.found)
		}
	}
	if m := newMatch("a.go", 1, "  // TODO  ", todoComment{}); m.Note != "// TODO" {
		t.Errorf("empty note = %q", m.Note)
	}
}

func TestLevelsSortMostUrgentFirst(t *testing.T) {
	levels := []string{"", "2", "0", "000", "1", "00", "9"}
	slices.SortFunc(levels, func(a, b string) int { return cmp.Compare(LevelRank(a), LevelRank(b)) })
	if want := []string{"000", "00", "0", "1", "2", "9", ""}; !slices.Equal(levels, want) {
		t.Fatalf("levels = %q, want %q", levels, want)
	}
	matches := sortedMatches([]Match{{Path: "a.go", Line: 1}, {Path: "z.go", Line: 9, Level: "0"}, {Path: "b.go", Line: 2, Level: "1"}})
	if len(matches) != 3 || matches[0].Path != "z.go" || matches[1].Path != "b.go" || matches[2].Path != "a.go" {
		t.Fatalf("levelled to-dos not first: %+v", matches)
	}
}
