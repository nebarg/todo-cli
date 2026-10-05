package scan

import (
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
		"long.go":               strings.Repeat("x := 1\n", 2000) + "// TODO: past the binary check\n",
	}
	cases := []struct {
		name    string
		exclude Exclude
		want    []string
	}{
		{"defaults read hidden files, but skip dependencies, dot directories and Markdown", defaultExclude,
			[]string{"sys.php:2", ".hidden.go:1", "code.go:1", "code.go:3", "dist/out.js:1", "long.go:2001", "odd[1]/x.go:1", "pkg/dist/out.js:1", "pkg/gen/gen.go:1", "sys.php:1", "sys.php:3", "web/.eslintrc.js:1"}},
		{"custom excludes replace the defaults", Exclude{Names: []string{"gen", "odd[1]"}, Paths: []string{"pkg/dist"}},
			[]string{"sys.php:2", ".hidden.go:1", "code.go:1", "code.go:3", "dist/out.js:1", "long.go:2001", "node_modules/dep.js:1", "sys.php:1", "sys.php:3", "vendor/lib.go:1", "web/.eslintrc.js:1", "web/node_modules/d.js:1"}},
	}
	for _, setup := range setups {
		t.Run(setup.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, files)
			if setup.repo {
				gitInit(t, dir)
			}
			keepGitConfigOut(t)
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
	for _, setup := range setups {
		t.Run(setup.name, func(t *testing.T) {
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
			if setup.repo {
				gitInit(t, dir)
			}
			keepGitConfigOut(t)
			matches, err := Source(t.Context(), dir, Exclude{})
			if err != nil || len(matches) != 1 || matches[0].Path != "ok/a.go" {
				t.Fatalf("matches = %+v, %v", matches, err)
			}
			if matches, err := Source(t.Context(), locked, Exclude{}); err == nil {
				t.Fatalf("scanning an unreadable directory = %+v, no error", matches)
			}
		})
	}
}

func TestScanReadsADirectoryItsRepositoryIgnores(t *testing.T) {
	repo := t.TempDir()
	writeFiles(t, repo, map[string]string{".gitignore": "/*\n", "ignored/a.go": "// TODO: in an ignored directory\n"})
	gitInit(t, repo)
	keepGitConfigOut(t)
	matches, err := Source(t.Context(), filepath.Join(repo, "ignored"), Exclude{})
	if err != nil || len(matches) != 1 || matches[0].Path != "a.go" {
		t.Fatalf("matches = %+v, %v", matches, err)
	}
}

func TestScanFollowsEachRepositorysGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"loose.go":           "// TODO: outside any repository\n",
		"app/.gitignore":     "/build/\n",
		"app/main.go":        "// TODO: in a repository\n",
		"app/build/out.go":   "// TODO: ignored by app\n",
		"app/lib/.gitignore": "*.gen.go\n",
		"app/lib/lib.go":     "// TODO: in a repository inside it\n",
		"app/lib/x.gen.go":   "// TODO: ignored by lib\n",
		"app/.cache/c.go":    "// TODO: in a dot directory\n",
	})
	gitInit(t, filepath.Join(dir, "app"))
	gitInit(t, filepath.Join(dir, "app", "lib"))
	keepGitConfigOut(t)
	for _, scan := range []struct{ dir, prefix string }{{dir, "app/"}, {filepath.Join(dir, "app"), ""}} {
		matches, err := Source(t.Context(), scan.dir, Exclude{})
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range matches {
			got = append(got, m.Path)
		}
		want := []string{scan.prefix + "lib/lib.go", scan.prefix + "main.go"}
		if scan.prefix != "" {
			want = append(want, "loose.go")
		}
		if !slices.Equal(got, want) {
			t.Errorf("scanning %s = %v, want %v", scan.dir, got, want)
		}
	}
}

func TestScanChecksOnlyTheFirstBytesForBinary(t *testing.T) {
	todo := "// TODO: found\n"
	for _, setup := range setups {
		t.Run(setup.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"nul-first.go":        "\x00" + todo,
				"nul-in-check.go":     todo + strings.Repeat("\n", binaryCheckLen-len(todo)-1) + "\x00",
				"nul-past-check.go":   todo + strings.Repeat("\n", binaryCheckLen-len(todo)) + "\x00 later",
				"short.go":            todo,
				"empty.go":            "",
				"no-final-newline.go": "// TODO: unfinished",
			})
			if setup.repo {
				gitInit(t, dir)
			}
			keepGitConfigOut(t)
			matches, err := Source(t.Context(), dir, Exclude{})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, m := range matches {
				got = append(got, fmt.Sprintf("%s:%d", m.Path, m.Line))
			}
			if want := []string{"no-final-newline.go:1", "nul-past-check.go:1", "short.go:1"}; !slices.Equal(got, want) {
				t.Fatalf("matches = %v, want %v", got, want)
			}
		})
	}
}

func TestScanSkipsUnreadableFiles(t *testing.T) {
	for _, setup := range setups {
		t.Run(setup.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"ok.go": "// TODO: readable\n", "locked.go": "// TODO: unreadable\n"})
			locked := filepath.Join(dir, "locked.go")
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := os.ReadFile(locked); err == nil {
				t.Skip("file permissions aren't enforced for this user")
			}
			if setup.repo {
				gitInit(t, dir)
			}
			keepGitConfigOut(t)
			matches, err := Source(t.Context(), dir, Exclude{})
			if err != nil || len(matches) != 1 || matches[0].Path != "ok.go" {
				t.Fatalf("matches = %+v, %v", matches, err)
			}
		})
	}
}

func TestScanStopsWhenCancelled(t *testing.T) {
	for _, setup := range setups {
		t.Run(setup.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"a.go": "// TODO: never read\n"})
			if setup.repo {
				gitInit(t, dir)
			}
			keepGitConfigOut(t)
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

// setups run a scan test on a directory outside a repository, and on one
// made a repository.
var setups = []struct {
	name string
	repo bool
}{{"outside a repository", false}, {"in a repository", true}}

// keepGitConfigOut stops the user's global gitignore from hiding test files.
func keepGitConfigOut(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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

func TestScanSkipsFilesOverTheSizeLimit(t *testing.T) {
	dir := t.TempDir()
	for name, size := range map[string]int{"limit.go": maxFileSize, "over.go": maxFileSize + 1} {
		todo := "// TODO: " + name + "\n"
		content := todo + strings.Repeat("\n", size-len(todo))
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	matches, err := Source(t.Context(), dir, Exclude{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Path != "limit.go" {
		t.Fatalf("matches = %+v, want only limit.go's", matches)
	}
}

func TestScanIsDeterministic(t *testing.T) {
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

func TestLevelledMatchesSortFirst(t *testing.T) {
	matches := sortedMatches([]Match{{Path: "a.go", Line: 1}, {Path: "z.go", Line: 9, Level: "0"}, {Path: "b.go", Line: 2, Level: "1"}})
	if len(matches) != 3 || matches[0].Path != "z.go" || matches[1].Path != "b.go" || matches[2].Path != "a.go" {
		t.Fatalf("levelled to-dos not first: %+v", matches)
	}
}
