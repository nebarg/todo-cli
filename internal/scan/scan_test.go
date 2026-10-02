package scan

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

var defaultExclude = Exclude{Names: []string{"node_modules", "vendor"}}

func TestScanSource(t *testing.T) {
	files := map[string]string{
		"code.go":               "// @todo fix this\nconst name = \"todo scan\"\n// TODO: test it\n",
		"TODO.md":               "- [ ] TODO: stored task\n",
		"notes.md":              "Plain text TODO: not a comment\n<!-- TODO: in a comment -->\n",
		".hidden.go":            "// TODO: hidden file\n",
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
		name     string
		allFiles bool
		exclude  Exclude
		want     []string
	}{
		{"defaults skip dependencies, dot directories and Markdown", false, defaultExclude,
			[]string{"sys.php:2", "code.go:1", "code.go:3", "dist/out.js:1", "odd[1]/x.go:1", "pkg/dist/out.js:1", "pkg/gen/gen.go:1", "sys.php:1", "sys.php:3"}},
		{"all files adds hidden files and Markdown comments, but still only comments", true, defaultExclude,
			[]string{"sys.php:2", ".hidden.go:1", "code.go:1", "code.go:3", "dist/out.js:1", "notes.md:2", "odd[1]/x.go:1", "pkg/dist/out.js:1", "pkg/gen/gen.go:1", "sys.php:1", "sys.php:3"}},
		{"custom excludes replace the defaults", false, Exclude{Names: []string{"gen", "odd[1]"}, Paths: []string{"pkg/dist"}},
			[]string{"sys.php:2", "code.go:1", "code.go:3", "dist/out.js:1", "node_modules/dep.js:1", "sys.php:1", "sys.php:3", "vendor/lib.go:1", "web/node_modules/d.js:1"}},
	}
	for _, scanner := range scanners(t) {
		t.Run(scanner.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if scanner.git {
				if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %v %s", err, out)
				}
			}
			t.Setenv("PATH", scanner.path)
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull) // A global gitignore would hide test files.
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			for _, c := range cases {
				matches, err := Source(dir, 0, c.allFiles, c.exclude)
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

type scanner struct {
	name string
	path string
	git  bool
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
	list := []scanner{{"built-in", t.TempDir(), false}, {"built-in with git", gitOnly, true}}
	if rg, err := exec.LookPath("rg"); err == nil {
		list = append(list, scanner{"ripgrep", filepath.Dir(rg), false})
	} else {
		t.Log("ripgrep not installed; skipping its scanner")
	}
	return list
}

func TestSortedMatchesOrderBeforeLimit(t *testing.T) {
	matches := []Match{{Path: "b.go", Line: 1}, {Path: "a.go", Line: 9}, {Path: "a.go", Line: 2}, {Path: "c.go", Line: 1}}
	got := sortedMatches(matches, 3)
	if len(got) != 3 || got[0] != (Match{Path: "a.go", Line: 2}) || got[1].Line != 9 || got[2].Path != "b.go" {
		t.Fatalf("matches were limited before sorting: %+v", got)
	}
	if got := sortedMatches(nil, 3); len(got) != 0 {
		t.Fatalf("empty scan = %+v", got)
	}
}

func TestBuiltInScanLimitIsDeterministic(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // Force the built-in scanner.
	dir := t.TempDir()
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("// TODO: item\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		matches, err := Source(dir, 3, false, Exclude{})
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 3 || matches[0].Path != "f00.go" || matches[2].Path != "f02.go" {
			t.Fatalf("limited scan picked arbitrary files: %+v", matches)
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
	matches := sortedMatches([]Match{{Path: "a.go", Line: 1}, {Path: "z.go", Line: 9, Level: "0"}, {Path: "b.go", Line: 2, Level: "1"}}, 2)
	if len(matches) != 2 || matches[0].Path != "z.go" || matches[1].Path != "b.go" {
		t.Fatalf("limit dropped an urgent to-do: %+v", matches)
	}
}
