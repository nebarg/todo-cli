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
)

const subtaskTasks = "- [ ] Ship login !high\n" +
	"  - [ ] Write the form\n" +
	"  - [x] Add the route\n" +
	"- [ ] Other !low\n" +
	"- [x] Old task\n" +
	"  - [ ] Left open\n\n" +
	"# auth\n\n" +
	"- [ ] Auth task\n" +
	"  - [ ] Auth sub\n"

// rowTitles is the task rows General shows, in order.
func rowTitles(m *model) string {
	var titles []string
	for _, row := range m.rows(generalPane) {
		if row.kind == rowTask {
			titles = append(titles, row.todo.Text)
		}
	}
	return strings.Join(titles, ",")
}

func TestSubtasksShowUnderTheirTask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte(subtaskTasks), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if got := rowTitles(m); got != "Ship login,Write the form,Add the route,Other,Old task,Left open" {
		t.Fatalf("subtasks are not under their tasks: %q", got)
	}
	render := func() string {
		return m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, 60, 20)
	}
	want := `▸ auth +0/1 │\n│ +│\n│ ● Ship login +│\n│   ○ Write the form +│\n│   ✓ Add the route +│\n│ ● Other +│\n│ ✓ Old task +│\n│   ○ Left open +│`
	if view := ansi.Strip(render()); !regexp.MustCompile(want).MatchString(view) {
		t.Fatalf("subtasks are not indented under their tasks:\n%s", view)
	}
	muted := func(text string) bool { return strings.Contains(render(), m.theme.Inline(text, m.theme.MutedStyle)) }
	if !muted("Left open") || muted("Write the form") {
		t.Fatal("only the subtasks of a done task should be greyed out")
	}
	// Counts leave out subtasks.
	if tabs := ansi.Strip(m.renderTabs()); !strings.Contains(tabs, "General 1/4") {
		t.Fatalf("General's count includes subtasks: %q", tabs)
	}
	m.general.cursor = 1
	if status := ansi.Strip(m.panelStatus()); status != "Open  ·  ● High priority  ·  1 of 2 subtasks done" {
		t.Fatalf("task status = %q", status)
	}

	// Done on the task leaves it, and its subtasks, where they are, greying
	// out the open one without ticking it.
	m = press(m, "d")
	wantFile := strings.Replace(subtaskTasks, "- [ ] Ship login !high", "- [x] Ship login !high", 1)
	if got := readFile(t, path); got != wantFile {
		t.Fatalf("after d on the task:\n%q\nwant\n%q", got, wantFile)
	}
	if got := rowTitles(m); got != "Ship login,Write the form,Add the route,Other,Old task,Left open" || m.general.cursor != 1 {
		t.Fatalf("done task moved: %q, cursor %d", got, m.general.cursor)
	}
	if !muted("Write the form") {
		t.Fatal("the open subtask of a task just marked done is not greyed out")
	}

	// A subtask has done, priority and edit, but keeps its task's category,
	// and no subtasks of its own.
	m.general.cursor = 2
	if status := ansi.Strip(m.panelStatus()); status != "Open  ·  No priority" {
		t.Fatalf("subtask status = %q", status)
	}
	m = press(m, "d")
	wantFile = strings.Replace(wantFile, "  - [ ] Write the form", "  - [x] Write the form", 1)
	if got := readFile(t, path); got != wantFile || m.general.cursor != 2 {
		t.Fatalf("after d on the subtask:\n%q\nwant\n%q", got, wantFile)
	}
	if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); strings.Contains(hints, "category") {
		t.Fatalf("a subtask's hints offer c: %q", hints)
	}
	m = press(m, "c")
	if m.status != subtaskStaysPut || isOpen[*categoryPrompt](m) {
		t.Fatalf("c on a subtask was not refused: %q", m.status)
	}
	m = press(m, "e")
	f := form(t, m)
	if plain := ansi.Strip(f.render(m.theme, 70, 20)); !strings.Contains(plain, "General › Ship login › Edit subtask") || strings.Contains(plain, "Category") || strings.Contains(plain, "Details") {
		t.Fatalf("subtask form:\n%s", plain)
	}
	f.title.SetValue("Write the forms")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	wantFile = strings.Replace(wantFile, "  - [x] Write the form", "  - [x] Write the forms", 1)
	if got := readFile(t, path); got != wantFile {
		t.Fatalf("after editing the subtask:\n%q\nwant\n%q", got, wantFile)
	}
	if selected, ok := m.selectedTask(); !ok || selected.Text != "Write the forms" || !selected.Subtask {
		t.Fatalf("edited subtask not selected: %+v", selected)
	}

	// Deleting a task names its subtasks, which go with it.
	m.general.cursor = 1
	m = press(m, "backspace")
	if dialog := deleteDialog(t, m); !strings.Contains(dialog, `Delete "Ship login"? Its 2 subtasks go too.`) {
		t.Fatalf("delete dialog = %q", dialog)
	}
	m = press(m, "esc")
	m.general.cursor = 2
	m = press(m, "backspace")
	if dialog := deleteDialog(t, m); !strings.Contains(dialog, `Delete "Write the forms"?`) || strings.Contains(dialog, "go too") {
		t.Fatalf("delete dialog = %q", dialog)
	}
	m = press(m, "esc")
	m.general.cursor = 3
	m = press(m, "backspace")
	if dialog := deleteDialog(t, m); !strings.Contains(dialog, `Delete "Add the route"?`) || strings.Contains(dialog, "go too") {
		t.Fatalf("delete dialog = %q", dialog)
	}
	m = press(m, "y")
	wantFile = strings.Replace(wantFile, "  - [x] Add the route\n", "", 1)
	if got := readFile(t, path); got != wantFile {
		t.Fatalf("after deleting the subtask:\n%q\nwant\n%q", got, wantFile)
	}

	// A reload moves a done task to the end with its subtasks.
	m = press(m, "r")
	if got := rowTitles(m); got != "Other,Ship login,Write the forms,Old task,Left open" {
		t.Fatalf("after reload: %q", got)
	}

	// All tasks lists subtasks under their task too.
	m = press(m, "i")
	if got := indexTitles(m.all.sorted(m.tasks.all)); got != "Other,Auth task,Auth sub,Ship login,Write the forms,Old task,Left open" {
		t.Fatalf("All tasks order = %q", got)
	}
	if view := ansi.Strip(m.all.view(m.theme, m.tasks.all, m.branchMissing, 80, 20)); !strings.Contains(view, "All tasks  2/4") || !regexp.MustCompile(`\n│   ○ Auth sub `).MatchString(view) {
		t.Fatalf("All tasks view:\n%s", view)
	}
}

func TestATaskAndASubtaskAlikeKeepTheirPlaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] x\n- [ ] Q\n- [ ] P\n  - [ ] x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	// p moves P, with its subtask, to the top of the file, above the task
	// that reads the same as the subtask.
	m.general.cursor = 2
	m = press(m, "p")
	if got, want := readFile(t, path), "- [ ] P !high\n  - [ ] x\n- [ ] x\n- [ ] Q\n"; got != want {
		t.Fatalf("after p:\n%q\nwant\n%q", got, want)
	}
	if got := rowTitles(m); got != "x,Q,P,x" || !m.rows(generalPane)[3].todo.Subtask {
		t.Fatalf("rows after p = %q", got)
	}
}

func TestAddingASubtaskFromATasksDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# auth\n\n- [ ] Ship login\n  - [x] Add the route\n- [x] Done task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m = press(m, "enter")
	pressKey(t, m, "right")
	if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); m.focus != detailPane || !strings.Contains(hints, "a add subtask") {
		t.Fatalf("details hints = %q", hints)
	}
	m = press(m, "a")
	f := form(t, m)
	if plain := ansi.Strip(f.render(m.theme, 70, 20)); !strings.Contains(plain, "General › @auth › Ship login › New subtask") || strings.Contains(plain, "Category") || strings.Contains(plain, "Details") {
		t.Fatalf("new subtask form:\n%s", plain)
	}
	f.title.SetValue("Write the form !high")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	want := "# auth\n\n- [ ] Ship login\n  - [x] Add the route\n  - [ ] Write the form !high\n- [x] Done task\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("after adding a subtask:\n%q\nwant\n%q", got, want)
	}
	// The list shows the new subtask, selected, above its done sibling.
	if selected, ok := m.selectedTask(); m.focus != generalPane || !ok || selected.Text != "Write the form" || !selected.Subtask {
		t.Fatalf("new subtask not selected in the list: focus %v, %+v", m.focus, selected)
	}
	if got := rowTitles(m); got != "Ship login,Write the form,Add the route,Done task" {
		t.Fatalf("rows = %q", got)
	}

	// From a subtask's details, a adds to the same task.
	pressKey(t, m, "right")
	m = press(m, "a")
	f = form(t, m)
	if plain := ansi.Strip(f.render(m.theme, 70, 20)); !strings.Contains(plain, "Ship login › New subtask") {
		t.Fatalf("new subtask form from a subtask:\n%s", plain)
	}
	f.title.SetValue("Test it")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	want = strings.Replace(want, "  - [ ] Write the form !high\n", "  - [ ] Write the form !high\n  - [ ] Test it\n", 1)
	if got := readFile(t, path); got != want {
		t.Fatalf("after adding a second subtask:\n%q\nwant\n%q", got, want)
	}
	if selected, ok := m.selectedTask(); !ok || selected.Text != "Test it" {
		t.Fatalf("second subtask not selected: %+v", selected)
	}
}

func TestClearingDoneTasksTakesTheirSubtasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	original := "- [x] Old task\n  - [ ] Left open\n  - [x] Done under done\n- [ ] Ship login\n  - [x] Add the route\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	// A done task's subtasks go with it, so the done one isn't counted again.
	if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); !strings.Contains(hints, "X clear 2 done") {
		t.Fatalf("hints = %q", hints)
	}
	m = press(m, "X")
	dialog := strings.Join(strings.Fields(strings.ReplaceAll(ansi.Strip(confirmation(t, m).render(m.theme)), "│", " ")), " ")
	if !strings.Contains(dialog, "Remove 1 done task and 1 done subtask from General? 2 subtasks go with their tasks, 1 of them open.") {
		t.Fatalf("clear dialog = %q", dialog)
	}
	m = press(m, "y")
	if got := readFile(t, path); got != "- [ ] Ship login\n" || m.status != "Removed 1 done task and 1 done subtask" {
		t.Fatalf("after clearing: %q, status %q", got, m.status)
	}
	m = press(m, "u")
	if got := readFile(t, path); got != original || m.status != "Restored 2 tasks" {
		t.Fatalf("after undo: %q, status %q", got, m.status)
	}
}
