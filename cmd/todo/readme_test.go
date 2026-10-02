package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

func pressKey(t *testing.T, m *model, key string) tea.Cmd {
	t.Helper()
	msg := tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	switch key {
	case "right":
		msg = tea.KeyPressMsg{Code: tea.KeyRight}
	case "left":
		msg = tea.KeyPressMsg{Code: tea.KeyLeft}
	}
	_, cmd := m.Update(msg)
	return cmd
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readmeTitles(tasks []store.ReadmeTask) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.Text
	}
	return strings.Join(names, ",")
}

func TestReadmeTasksOpenFromGeneralAndOnlyToggle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	readme := filepath.Join(dir, "README.md")
	tasks := "- [ ] General task\n\n# auth\n\n- [ ] Auth task\n"
	if err := os.WriteFile(path, []byte(tasks), 0644); err != nil {
		t.Fatal(err)
	}
	original := "# Project\n\n## TODOs\n\n- [x] Finished\n- Write docs\n- todo1 Later\n- todo00 Urgent\n\n## Usage\n\n- not a task\n"
	if err := os.WriteFile(readme, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}

	rows := m.rows(generalPane)
	if len(rows) != 3 || rows[0].name != "auth" || rows[1].kind != rowReadme || rows[1].count != 4 || rows[1].completed != 1 || rows[2].todo.Text != "General task" {
		t.Fatalf("README group is not listed after categories: %+v", rows)
	}
	if tabs := ansi.Strip(m.renderTabs()); !strings.Contains(tabs, "General 1/6") {
		t.Fatalf("General tab count leaves out README tasks: %q", tabs)
	}
	m.general.cursor = 1
	pressKey(t, m, "right")
	if got := readmeTitles(m.tasks.readme); got != "Urgent,Later,Finished,Write docs" {
		t.Fatalf("README tasks are not in level order: %q", got)
	}
	view := ansi.Strip(m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20))
	for _, want := range []string{`General › README\.md  1/4`, `\n│ 00 Urgent`, `\n│ 1  Later`, `\n│ ·  Write docs`, `\n│ ✓  Finished`} {
		if !regexp.MustCompile(want).MatchString(view) {
			t.Fatalf("README list is missing %q:\n%s", want, view)
		}
	}
	if status := ansi.Strip(m.panelStatus()); status != "Open  ·  README.md:8" {
		t.Fatalf("status bar = %q", status)
	}

	pressKey(t, m, "right")
	details := ansi.Strip(m.renderDetailPane(60, 12))
	if m.focus != detailPane || !strings.Contains(details, "00 Urgent") || strings.Contains(details, "No details yet") || !strings.Contains(ansi.Strip(ui.RenderHints(m.footerHints())), "e open file") {
		t.Fatalf("README task details:\n%s\nhints %v", details, m.footerHints())
	}
	for _, key := range []string{"p", "c", "X"} {
		pressKey(t, m, key)
		if m.status != readmeReadOnly || isOpen[*categoryPrompt](m) || isOpen[*clearConfirmation](m) {
			t.Fatalf("%s on a README task was not refused: %q", key, m.status)
		}
	}
	if cmd := pressKey(t, m, "e"); cmd == nil || isOpen[*taskModal](m) {
		t.Fatal("e on a README task should open the editor, not the edit form")
	}
	pressKey(t, m, "left")

	// Done, reopen, then done again, each adding the checkbox a plain item lacks.
	m.general.cursor = 2
	for _, key := range []string{"d", "k", "d", "k", "k", "d"} {
		pressKey(t, m, key)
	}
	want := "# Project\n\n## TODOs\n\n- [ ] Finished\n- [x] Write docs\n- todo1 Later\n- [x] todo00 Urgent\n\n## Usage\n\n- not a task\n"
	if got := readFile(t, readme); got != want {
		t.Fatalf("README after toggles:\n%q\nwant\n%q", got, want)
	}
	if got := readFile(t, path); got != tasks {
		t.Fatalf("todo.md changed: %q", got)
	}
	if m.status != "" {
		t.Fatalf("toggle reported %q", m.status)
	}

	pressKey(t, m, "1")
	if m.general.open.kind == rowReadme || m.general.cursor != 1 {
		t.Fatalf("1 did not return to the README row: open %v, cursor %d", m.general.open.kind == rowReadme, m.general.cursor)
	}
}

func TestReadmeGroupClosesWhenItsTasksGo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("## Todo:\n\n- Only task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	if m.general.open.kind != rowReadme {
		t.Fatal("README group did not open")
	}
	if cmd := pressKey(t, m, "a"); !isOpen[*taskModal](m) || form(t, m).target.Category != "" {
		t.Fatalf("a inside README.md should add a general task: %+v %v", m.overlay, cmd)
	}
	m.overlay = nil
	if err := os.WriteFile(readme, []byte("# Project\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "r")
	if m.general.open.kind == rowReadme || len(m.rows(generalPane)) != 0 {
		t.Fatalf("README group stayed open without tasks: %+v", m.rows(generalPane))
	}
}

func TestAddingFromTheReadmeGroupShowsTheNewTask(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("## Todo:\n\n- Only task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(filepath.Join(dir, "todo.md"), projectContext{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	pressKey(t, m, "a")
	form(t, m).title.SetValue("New general task")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	if selected, ok := m.selectedTask(); m.general.open.kind == rowReadme || !ok || selected.Text != "New general task" {
		t.Fatalf("new task not shown: README open %v, selected %+v", m.general.open.kind == rowReadme, selected)
	}
}

func TestBranchTaskStatusIgnoresTheReadmeGroupInGeneral(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("## Todo\n\n- Readme task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## main\n\n- [ ] Branch task !high\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	pressKey(t, m, "2")
	pressKey(t, m, "right")
	if status := ansi.Strip(m.panelStatus()); !strings.Contains(status, "High priority") || strings.Contains(status, readmeGroup) {
		t.Fatalf("branch task status = %q", status)
	}
}

func TestReadmeTasksWithoutLevelsKeepTheOpenBullet(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("## TODOs\n\n- Open task\n- [x] Done task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(filepath.Join(dir, "todo.md"), projectContext{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	view := ansi.Strip(m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20))
	for _, want := range []string{`\n│ ○ Open task`, `\n│ ✓ Done task`} {
		if !regexp.MustCompile(want).MatchString(view) {
			t.Fatalf("README list is missing %q:\n%s", want, view)
		}
	}
}
