package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestCommandRunsTheEditorInTheShell runs stand-in editors, in a directory
// with a space in its name, and checks the arguments each one receives.
func TestCommandRunsTheEditorInTheShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows runs the editor without sh")
	}
	dir := filepath.Join(t.TempDir(), "My Editor")
	received := filepath.Join(t.TempDir(), "args")
	for _, name := range []string{"vim", "code", "nano"} {
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + received + "'\n"
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	file := "/tmp/My Notes/TODO.md"
	cases := []struct {
		visual, editor string
		want           []string
	}{
		{`"` + dir + `/vim" -n`, "", []string{"-n", "+12", file}},
		{strings.ReplaceAll(dir, " ", `\ `) + "/code", "", []string{"--wait", "--goto", file + ":12"}},
		{"", "'" + dir + "/nano' --view", []string{"--view", file}},
		{"   ", "'" + dir + "/vim'", []string{"+12", file}},
	}
	for _, c := range cases {
		t.Setenv("VISUAL", c.visual)
		t.Setenv("EDITOR", c.editor)
		if err := os.Remove(received); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if out, err := Command(file, 12).CombinedOutput(); err != nil {
			t.Fatalf("VISUAL=%q EDITOR=%q: %v %s", c.visual, c.editor, err, out)
		}
		data, err := os.ReadFile(received)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !slices.Equal(got, c.want) {
			t.Fatalf("VISUAL=%q EDITOR=%q: editor received %q, want %q", c.visual, c.editor, got, c.want)
		}
	}
}

func TestCommandFallsBackToVi(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows runs the editor without sh")
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	want := []string{"sh", "-c", `vi "$@"`, "vi", "+12", "/tmp/TODO.md"}
	if got := Command("/tmp/TODO.md", 12).Args; !slices.Equal(got, want) {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestProgramIsTheFirstShellWord(t *testing.T) {
	for command, want := range map[string]string{
		"vi":                                 "vi",
		"code --wait":                        "code",
		`"/opt/My Editor/nvim" -u NONE`:      "/opt/My Editor/nvim",
		`'/opt/My Editor/code' --new-window`: "/opt/My Editor/code",
		`/opt/My\ Editor/vim -n`:             "/opt/My Editor/vim",
		`"a\"b" c`:                           `a"b`,
		`'a\b' c`:                            `a\b`,
		"vim\t-n":                            "vim",
	} {
		if got := program(command); got != want {
			t.Errorf("program(%q) = %q, want %q", command, got, want)
		}
	}
}
