package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/editor"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/scan"
)

func scannedModel(t *testing.T) *model {
	t.Helper()
	m := newModel(t.TempDir(), scan.Exclude{}, nil)
	m.Update(filesui.ScannedMsg{Matches: []scan.Match{
		{Path: "app.go", Line: 6, Note: "fix the race", Level: "0"},
		{Path: "app.go", Line: 3, Note: "retry the request"},
		{Path: "ui.css", Line: 9, Note: "split this", Category: "boundary"},
	}})
	return m
}

func keyPress(k string) tea.KeyPressMsg {
	switch k {
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

func TestScreensFitTheTerminal(t *testing.T) {
	m := scannedModel(t)
	for _, size := range [][2]int{{120, 35}, {80, 24}, {56, 16}} {
		m.width, m.height = size[0], size[1]
		for _, page := range []string{"list", "details"} {
			if page == "details" {
				m.Update(keyPress("j"))
				m.Update(keyPress("right"))
			}
			view := m.View().Content
			if w, h := lipgloss.Width(view), lipgloss.Height(view); w != size[0] || h != size[1] {
				t.Errorf("%s at %dx%d is %dx%d", page, size[0], size[1], w, h)
			}
			lines := strings.Split(ansi.Strip(view), "\n")
			if !strings.HasPrefix(lines[0], " todo-scan ") || !strings.HasSuffix(lines[0], "3 TODOs ") {
				t.Errorf("%s header at %dx%d = %q", page, size[0], size[1], lines[0])
			}
			footer := lines[len(lines)-1]
			if !strings.HasSuffix(footer, "q quit") || strings.Contains(footer, "r rescan") != (page == "list") {
				t.Errorf("%s footer at %dx%d = %q", page, size[0], size[1], footer)
			}
			if page == "details" {
				m.Update(keyPress("left"))
				m.Update(keyPress("k"))
			}
		}
	}
	m.width, m.height = 50, 10
	if view := m.View().Content; !strings.Contains(view, "enlarge your terminal") {
		t.Fatalf("a tiny terminal = %q", view)
	}
}

func TestHeaderShortensTheDirectory(t *testing.T) {
	m := newModel("/srv/"+strings.Repeat("deep/", 20)+"project", scan.Exclude{}, nil)
	header := ansi.Strip(m.renderHeader(60))
	if ansi.StringWidth(header) != 60 || !strings.Contains(header, "…") || !strings.Contains(header, "/project") || !strings.HasSuffix(header, "scanning… ") {
		t.Fatalf("header = %q", header)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if got := displayDir(filepath.Join(home, "Code", "app")); got != filepath.Join("~", "Code", "app") {
		t.Fatalf("displayDir = %q", got)
	}
}

func TestKeys(t *testing.T) {
	m := scannedModel(t)
	for _, k := range []string{"q", "ctrl+c"} {
		if _, cmd := m.Update(keyPress(k)); cmd == nil || cmd() != tea.Quit() {
			t.Fatalf("%s did not quit", k)
		}
	}
	m.status = "old"
	_, cmd := m.Update(keyPress("r"))
	if cmd == nil || !m.files.Loading() || m.status != "" {
		t.Fatal("r did not rescan")
	}
	if err := os.Remove(m.dir); err != nil {
		t.Fatal(err)
	}
	_, statErr := os.Stat(m.dir)
	m.Update(cmd())
	if want := "Scan failed: " + statErr.Error(); m.status != want {
		t.Fatalf("status = %q, want %q", m.status, want)
	}
	if _, cmd := m.Update(editor.ClosedMsg{Err: fmt.Errorf("exit status 1")}); cmd == nil || m.status != "exit status 1" || !m.files.Loading() {
		t.Fatalf("closing the editor should rescan and report its error: %q", m.status)
	}
	m = scannedModel(t)
	m.Update(keyPress("right"))
	if !strings.Contains(ansi.Strip(m.View().Content), "Files › @boundary") {
		t.Fatalf("right did not reach the browser:\n%s", ansi.Strip(m.View().Content))
	}
}

func TestHeaderCountsLevelledTodos(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("// TODO: plain\n// todo0 urgent\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir, scan.Exclude{}, scan.LevelFilter(true, nil))
	m.Update(m.files.Scan()())
	if header := ansi.Strip(m.renderHeader(80)); !strings.HasSuffix(header, "1 levelled TODO ") {
		t.Fatalf("header = %q", header)
	}
}
