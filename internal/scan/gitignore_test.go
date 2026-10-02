package scan

import (
	"bytes"
	"cmp"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestIgnoreRulesMatchGit lists the files of a repository with tricky
// ignore rules, and compares them with the untracked files Git lists, with
// core.ignoreCase set in the repository, the user's configuration or both.
func TestIgnoreRulesMatchGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	// A blank setting leaves core.ignoreCase out of that configuration.
	cases := []struct{ name, user, repo string }{
		{"case-sensitive", "", "false"},
		{"ignoring case", "", "true"},
		{"ignoring case for the user", "yes", ""},
		{"the repository's setting wins", "true", "false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { testIgnoreRulesMatchGit(t, c.user, c.repo) })
	}
}

func testIgnoreRulesMatchGit(t *testing.T, userIgnoreCase, repoIgnoreCase string) {
	config := t.TempDir()
	gitconfig := "[user]\n\tname = Test\n[core]\n\texcludesFile = " + filepath.Join(config, "ignore") + " ; a comment\n"
	if userIgnoreCase != "" {
		gitconfig += "\tignoreCase = " + userIgnoreCase + "\n"
	}
	writeFiles(t, config, map[string]string{"gitconfig": gitconfig, "ignore": "global-*.go\n"})
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(config, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	gitInit(t, dir)
	// git init sets core.ignoreCase on a file system that ignores case, so
	// it is set before it is removed.
	git(t, dir, "config", "core.ignoreCase", cmp.Or(repoIgnoreCase, "false"))
	if repoIgnoreCase == "" {
		git(t, dir, "config", "--unset", "core.ignoreCase")
	}
	files := map[string]string{
		".git/info/exclude": "from-info.go\n*.tmp\n",
		".gitignore": strings.Join([]string{
			"# a comment", `\#literal.go`, `\!bang.go`, "*.log", "!keep.log", "build/", "!build/rescued.go",
			"/top.go", "docs/*.go", "**/gen/", "a/**/z.go", "logs/**", "!logs/kept.go", "[ab]c.go", "[!x]y.go",
			"q?.go", "dironly/", "trailing.go   ", `space\ .go`, `tail\ `, "sub/**/deep.go", "!global-keep.go", "!wanted.tmp",
			"*.UPPER", "!KEEP.upper", "Mixed/", "/Caps.go", "lower.go", "[A-C]x.go", "Up/**/*.Go",
		}, "\n") + "\n",
		"src/.gitignore":  "*.go\n!keep.go\n/only-here.txt\n",
		"crlf/.gitignore": "*.go\r\n",
	}
	for _, name := range []string{
		"#literal.go", "!bang.go", "a.log", "keep.log", "build/x.go", "build/rescued.go", "top.go", "sub/top.go",
		"docs/a.go", "docs/deep/b.go", "gen/g.go", "src/gen/g.go", "generator.go", "a/z.go", "a/b/z.go",
		"a/b/c/z.go", "b/z.go", "logs/x.go", "logs/kept.go", "ac.go", "bc.go", "cc.go", "ay.go", "xy.go",
		"q1.go", "q12.go", "dironly", "x/dironly/d.go", "trailing.go", "space .go", "tail ", "sub/deep.go",
		"sub/x/deep.go", "src/main.go", "src/keep.go", "src/only-here.txt", "src/x/only-here.txt", "crlf/c.go",
		"from-info.go", "other.tmp", "wanted.tmp", "global-1.go", "global-keep.go",
		"x.upper", "keep.upper", "mixed/m.go", "caps.go", "sub/caps.go", "LOWER.go", "bx.go", "GLOBAL-2.go", "up/a/b.go",
	} {
		files[name] = "x\n"
	}
	writeFiles(t, dir, files)

	var got []string
	if err := walkFiles(t.Context(), dir, Exclude{}, func(path string) { got = append(got, filepath.ToSlash(path)) }); err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	var want []string
	for path := range bytes.SplitSeq(git(t, dir, "ls-files", "--others", "--exclude-standard", "-z"), []byte{0}) {
		if len(path) > 0 {
			want = append(want, string(path))
		}
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("listed files differ from Git's:\n got  %q\n want %q", got, want)
	}
	// Every rule above should have had something to do.
	if len(got) > len(files)/2 {
		t.Fatalf("only %d of %d files ignored: %q", len(files)-len(got), len(files), got)
	}
}

func git(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func TestGlobalIgnoreFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	write := func(name, content string) {
		t.Helper()
		writeFiles(t, home, map[string]string{name: content})
	}
	check := func(want string) {
		t.Helper()
		if got := globalIgnoreFile(); got != want {
			t.Fatalf("global gitignore = %q, want %q", got, want)
		}
	}

	check(filepath.Join(home, ".config", "git", "ignore"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	check(filepath.Join(home, "xdg", "git", "ignore"))

	write("xdg/git/config", "[core]\n\texcludesFile = /from/xdg\n")
	check("/from/xdg")
	write(".gitconfig", "[CORE]\n  ExcludesFile=~/dotfiles/ignore # a comment\n[remote \"origin\"]\n\texcludesfile = /wrong\n")
	check(filepath.Join(home, "dotfiles", "ignore"))
	write(".gitconfig", "[core]\n\texcludesfile = /first\n\texcludesfile = \"/with space\" ; a comment\n")
	check("/with space")

	write("other", "[core]\n\texcludesfile = /from/env\n")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "other"))
	check("/from/env")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	check(filepath.Join(home, "xdg", "git", "ignore"))
}
