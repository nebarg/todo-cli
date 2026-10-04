package dashboard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/store"
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
	m, err := newModel(path, project.Context{}, testFiles())
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
	if got := readmeTitles(m.tasks.readme); got != "Urgent,Later,Write docs,Finished" {
		t.Fatalf("README tasks are not open then done, each in level order: %q", got)
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
	if m.focus != detailPane || !strings.Contains(details, "00 Urgent") || strings.Contains(details, "No details yet") || !strings.Contains(ansi.Strip(m.theme.RenderHints(m.footerHints())), "e open file") {
		t.Fatalf("README task details:\n%s\nhints %v", details, m.footerHints())
	}
	for _, key := range []string{"p", "c", "X", "a"} {
		pressKey(t, m, key)
		if m.status != readmeReadOnly || isOpen[*categoryPrompt](m) || isOpen[*clearConfirmation](m) || isOpen[*taskModal](m) {
			t.Fatalf("%s on a README task was not refused: %q", key, m.status)
		}
	}
	if cmd := pressKey(t, m, "e"); cmd == nil || isOpen[*taskModal](m) {
		t.Fatal("e on a README task should open the editor, not the edit form")
	}
	pressKey(t, m, "left")

	// Done, reopen, then done again, each adding the checkbox a plain item
	// lacks, with every row staying where it is.
	m.general.cursor = 2
	for _, key := range []string{"d", "j", "d", "k", "k", "k", "d"} {
		pressKey(t, m, key)
	}
	var shown []string
	for _, row := range m.rows(generalPane) {
		shown = append(shown, row.readme.Text)
	}
	if got := strings.Join(shown, ","); got != "Urgent,Later,Write docs,Finished" || m.general.cursor != 0 {
		t.Fatalf("toggles moved README rows: %q, cursor %d", got, m.general.cursor)
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
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	if m.general.open.kind != rowReadme {
		t.Fatal("README group did not open")
	}
	if err := os.WriteFile(readme, []byte("# Project\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "r")
	if m.general.open.kind == rowReadme || len(m.rows(generalPane)) != 0 {
		t.Fatalf("README group stayed open without tasks: %+v", m.rows(generalPane))
	}
}

func TestAddingIsRefusedInsideTheReadmeGroup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Todo\n\n- Loose task\n\n## Backend\n\n- Heading task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(filepath.Join(dir, "todo.md"), project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	// The README.md row is General's, where a adds a general task.
	if pressKey(t, m, "a"); !isOpen[*taskModal](m) || form(t, m).mode != modalAddGeneral {
		t.Fatalf("a on the README.md row did not add a general task: %T", m.overlay)
	}
	m.overlay = nil
	pressKey(t, m, "right")
	for _, where := range []struct {
		name string
		keys []string
	}{
		{"on a heading row", nil},
		{"on a task row", []string{"j"}},
		{"in a heading", []string{"k", "right"}},
		{"on a task's details", []string{"right"}},
	} {
		for _, key := range where.keys {
			pressKey(t, m, key)
		}
		m.status = ""
		if pressKey(t, m, "a"); m.status != readmeReadOnly || m.overlay != nil {
			t.Fatalf("a %s was not refused: status %q, overlay %T", where.name, m.status, m.overlay)
		}
	}
	if m.focus != detailPane || m.general.open.kind != rowReadmeHeading {
		t.Fatalf("test did not reach a heading task's details: focus %v, open %+v", m.focus, m.general.open)
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
	m, err := newModel(path, project.Context{}, testFiles())
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
	m, err := newModel(filepath.Join(dir, "todo.md"), project.Context{}, testFiles())
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

func TestReadmeHeadingsOpenFromTheReadmeGroup(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	original := "# Todo\n\n- Loose task\n- [x] Loose done\n\n## Now\n\n- [ ] Ship it\n- [x] Tested\n\n## Later\n\n- todo0 Urgent later\n\n# Install\n\n- not a task\n"
	if err := os.WriteFile(readme, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(filepath.Join(dir, "todo.md"), project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.rows(generalPane); len(rows) != 1 || rows[0].kind != rowReadme || rows[0].count != 5 || rows[0].completed != 2 {
		t.Fatalf("README group does not count the tasks under its headings: %+v", rows)
	}

	pressKey(t, m, "right")
	view := ansi.Strip(m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20))
	// Headings keep the README's order, though Later has the more urgent task.
	for _, want := range []string{`General › README\.md  2/5`, `\n│ ▸ Now +1/2 *│\n│ ▸ Later +0/1 *│\n│ +│\n│ ○ Loose task`, `\n│ ✓ Loose done`} {
		if !regexp.MustCompile(want).MatchString(view) {
			t.Fatalf("README group is missing %q:\n%s", want, view)
		}
	}
	if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); !strings.Contains(hints, "→ open") || !strings.Contains(hints, "← back") || strings.Contains(hints, "delete") {
		t.Fatalf("heading row hints = %q", hints)
	}
	for _, key := range []string{"backspace", "X"} {
		press(m, key)
		if m.status != readmeReadOnly || isOpen[*deleteConfirmation](m) || isOpen[*clearConfirmation](m) {
			t.Fatalf("%q on a README heading was not refused: %q", key, m.status)
		}
	}

	pressKey(t, m, "j")
	pressKey(t, m, "right")
	if m.general.open != (group{kind: rowReadmeHeading, name: "Later"}) {
		t.Fatalf("Later did not open: %+v", m.general.open)
	}
	view = ansi.Strip(m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20))
	if !strings.Contains(view, "General › README.md › Later  0/1") || !strings.Contains(view, "0 Urgent later") || strings.Contains(view, "Ship it") {
		t.Fatalf("Later heading:\n%s", view)
	}
	pressKey(t, m, "left")
	if !m.general.open.inReadme() || m.general.open.kind != rowReadme || m.general.cursor != 1 {
		t.Fatalf("← did not return to the Later row: %+v, cursor %d", m.general.open, m.general.cursor)
	}

	pressKey(t, m, "k")
	pressKey(t, m, "right")
	pressKey(t, m, "d")
	want := "# Todo\n\n- Loose task\n- [x] Loose done\n\n## Now\n\n- [x] Ship it\n- [x] Tested\n\n## Later\n\n- todo0 Urgent later\n\n# Install\n\n- not a task\n"
	if got := readFile(t, readme); got != want || m.status != "" {
		t.Fatalf("README after marking Ship it done:\n%q\nwant\n%q\nstatus %q", got, want, m.status)
	}
	if selected, ok := m.selectedReadmeTask(); !ok || selected.Text != "Ship it" {
		t.Fatalf("selection did not follow the toggled task: %+v", selected)
	}

	// A heading whose tasks go closes back to the README.md group, and 1
	// goes back to the top from a heading.
	if err := os.WriteFile(readme, []byte("# Todo\n\n- Loose task\n\n## Later\n\n- todo0 Urgent later\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "r")
	if m.general.open.kind != rowReadme || m.general.cursor != 0 {
		t.Fatalf("emptied heading did not close to its row: %+v, cursor %d", m.general.open, m.general.cursor)
	}
	pressKey(t, m, "right")
	pressKey(t, m, "1")
	if m.general.open != (group{}) || m.general.cursor != 0 {
		t.Fatalf("1 did not go back to the top: %+v, cursor %d", m.general.open, m.general.cursor)
	}

	pressKey(t, m, "right")
	pressKey(t, m, "right")
	if err := os.WriteFile(readme, []byte("# Project\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "r")
	if m.general.open != (group{}) {
		t.Fatalf("README heading stayed open without tasks: %+v", m.general.open)
	}
}

func TestReadmeTaskDetailsShowLikeATaskFilesDetails(t *testing.T) {
	dir := t.TempDir()
	readme := "## TODOs\n\n- [ ] **Slug stability.** Renaming changes its URLs.\n    - a slug fixed at creation, or\n    - editable slugs\n- [ ] No notes\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(filepath.Join(dir, "todo.md"), project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	view := ansi.Strip(m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20))
	for _, want := range []string{`General › README\.md  0/2`, `\n│ ○ Slug stability\. Renaming changes its URLs\. +⋯ │`, `\n│ ○ No notes +│`} {
		if !regexp.MustCompile(want).MatchString(view) {
			t.Fatalf("README list is missing %q:\n%s", want, view)
		}
	}
	pressKey(t, m, "right")
	details := ansi.Strip(m.renderDetailPane(60, 12))
	for _, want := range []string{"Slug stability. Renaming changes its URLs.", "- a slug fixed at creation, or", "- editable slugs"} {
		if !strings.Contains(details, want) {
			t.Fatalf("details are missing %q:\n%s", want, details)
		}
	}
}

func TestReadmeSubtasksShowUnderTheirParents(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	original := "## TODOs\n\n" +
		"- [ ] todo1 Parent\n" +
		"    - [ ] First sub\n" +
		"    - [x] Done sub\n" +
		"        - [ ] todo0 Deep sub\n" +
		"- [ ] Other\n" +
		"- [x] Finished parent\n" +
		"    - [ ] Open under finished\n"
	if err := os.WriteFile(readme, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(filepath.Join(dir, "todo.md"), project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	pressKey(t, m, "right")
	view := func() string {
		return ansi.Strip(m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20))
	}
	// Subtasks follow their parent, open ones first, one level in however
	// deep they're nested; the most urgent task, Deep sub, doesn't leave it.
	// The count leaves out subtasks.
	want := `General › README\.md  1/3 *│\n│ *│\n│ 1 Parent +│\n│   0 Deep sub +│\n│   · First sub +│\n│   ✓ Done sub +│\n│ · Other +│\n│ ✓ Finished parent +│\n│   · Open under finished +│`
	if !regexp.MustCompile(want).MatchString(view()) {
		t.Fatalf("README rows are not grouped under their parents:\n%s", view())
	}
	if status := ansi.Strip(m.panelStatus()); status != "Open  ·  README.md:3  ·  1 of 3 subtasks done" {
		t.Fatalf("parent status = %q", status)
	}
	raw := m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20)
	muted := func(text string) bool { return strings.Contains(raw, m.theme.Inline(text, m.theme.MutedStyle)) }
	if !muted("Open under finished") || muted("First sub") {
		t.Fatal("only the subtasks of a done parent should be greyed out")
	}

	// Done on the parent changes only its line, and nothing moves.
	pressKey(t, m, "d")
	if got, want := readFile(t, readme), strings.Replace(original, "- [ ] todo1 Parent", "- [x] todo1 Parent", 1); got != want {
		t.Fatalf("README after d:\n%q\nwant\n%q", got, want)
	}
	if !regexp.MustCompile(`\n│ ✓ Parent +│\n│   0 Deep sub +│\n│   · First sub +│`).MatchString(view()) || m.general.cursor != 0 {
		t.Fatalf("done parent moved:\n%s", view())
	}
	raw = m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20)
	if !muted("First sub") || !muted("Deep sub") {
		t.Fatal("subtasks of a parent just marked done are not greyed out")
	}
	if rows := m.rows(generalPane); rows[2].readme.Done {
		t.Fatal("marking the parent done marked a subtask done")
	}

	// A reload moves the parent, with every subtask, to the done tasks.
	pressKey(t, m, "r")
	if !regexp.MustCompile(`\n│ · Other +│\n│ ✓ Parent +│\n│   0 Deep sub +│\n│   · First sub +│\n│   ✓ Done sub +│\n│ ✓ Finished parent +│\n│   · Open under finished +│`).MatchString(view()) {
		t.Fatalf("reload did not move the done parent with its subtasks:\n%s", view())
	}
}
