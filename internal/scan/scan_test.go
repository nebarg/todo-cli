package scan

import (
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
	}
	cases := []struct {
		name     string
		allFiles bool
		exclude  Exclude
		want     []string
	}{
		{"defaults skip dependencies, dot directories and Markdown", false, defaultExclude,
			[]string{"code.go:1", "code.go:3", "dist/out.js:1", "odd[1]/x.go:1", "pkg/dist/out.js:1", "pkg/gen/gen.go:1"}},
		{"all files still skips excluded and dot directories", true, defaultExclude,
			[]string{".hidden.go:1", "TODO.md:1", "code.go:1", "code.go:2", "code.go:3", "dist/out.js:1", "odd[1]/x.go:1", "pkg/dist/out.js:1", "pkg/gen/gen.go:1"}},
		{"custom excludes replace the defaults", false, Exclude{Names: []string{"gen", "odd[1]"}, Paths: []string{"pkg/dist"}},
			[]string{"code.go:1", "code.go:3", "dist/out.js:1", "node_modules/dep.js:1", "vendor/lib.go:1", "web/node_modules/d.js:1"}},
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
