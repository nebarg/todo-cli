package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// testFiles is a Files tab that tests fill with filesui.ScannedMsg rather
// than by scanning.
func testFiles() filesui.Model { return filesui.New(".", scan.Exclude{}, nil) }

// runCmd runs cmd as the program would: m gets each message it leads to,
// and the commands m returns run in turn. A batch's commands run at once.
func runCmd(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		_, next := m.Update(msg)
		runCmd(t, m, next)
		return
	}
	msgs := make([]tea.Msg, len(batch))
	var wg sync.WaitGroup
	for i, cmd := range batch {
		if cmd != nil {
			wg.Go(func() { msgs[i] = cmd() })
		}
	}
	wg.Wait()
	for _, msg := range msgs {
		runCmd(t, m, func() tea.Msg { return msg })
	}
}

// pressAndRun presses key and runs the commands it starts, such as asking
// Git in the background.
func pressAndRun(t *testing.T, m *model, key string) *model {
	t.Helper()
	runCmd(t, m, pressKey(t, m, key))
	return m
}

// answerGit gives m Git's answer to the background check that a branch task
// form starts, without waiting on the form's cursor blink that runCmd would
// also run.
func answerGit(t *testing.T, m *model) {
	t.Helper()
	runCmd(t, m, m.checkBranches())
}

// isOpen reports whether the overlay open over m is a T.
func isOpen[T overlay](m *model) bool {
	_, ok := m.overlay.(T)
	return ok
}

// opened is the overlay open over m, failing the test unless it is a T.
func opened[T overlay](t *testing.T, m *model) T {
	t.Helper()
	o, ok := m.overlay.(T)
	if !ok {
		t.Fatalf("overlay = %#v, want a %T", m.overlay, o)
	}
	return o
}

func form(t *testing.T, m *model) *taskModal                 { return opened[*taskModal](t, m) }
func confirmation(t *testing.T, m *model) *clearConfirmation { return opened[*clearConfirmation](t, m) }
func prompt(t *testing.T, m *model) *categoryPrompt          { return opened[*categoryPrompt](t, m) }

func TestChangingCategoryKeepsTaskSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("## General\n\n### @auth\n\n- [ ] Fix login\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = opened.(*model)
	if !isOpen[*categoryPrompt](m) {
		t.Fatal("c did not open category editing")
	}
	if got := prompt(t, m).input.Prompt; got != "Category: " {
		t.Fatalf("category prompt = %q", got)
	}
	prompt(t, m).input.SetValue("backend")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if isOpen[*categoryPrompt](m) || !ok || selected.Text != "Fix login" || selected.Category != "backend" {
		t.Fatalf("recategorising lost selection: %+v", m.rows(generalPane))
	}
}

func TestCategoryPromptShowsAFailedSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.startCategoryInput()
	prompt(t, m).input.SetValue("docs")
	if err := os.WriteFile(path, []byte("- [ ] Changed elsewhere\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m = press(m, "enter")
	if footer := ansi.Strip(m.renderFooter(100)); !isOpen[*categoryPrompt](m) || !strings.Contains(footer, "Task changed on disk; press r to reload") {
		t.Fatalf("failed save: prompt open %v, footer %q", isOpen[*categoryPrompt](m), footer)
	}
	if footer := ansi.Strip(press(m, "x").renderFooter(100)); strings.Contains(footer, "Task changed") || !strings.Contains(footer, "cancel") {
		t.Fatalf("typing kept the error: %q", footer)
	}
}

func TestCategoryInputTakesSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.startCategoryInput()
	prompt(t, m).input.SetValue("release")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = updated.(*model)
	updated, _ = m.Update(tea.PasteMsg{Content: "notes\nfor  v2"})
	m = updated.(*model)
	if got := prompt(t, m).input.Value(); got != "release notes for  v2" {
		t.Fatalf("category input = %q", got)
	}
	m = press(m, "enter")
	if isOpen[*categoryPrompt](m) {
		t.Fatalf("spaced category was not saved: %q", prompt(t, m).err)
	}
	if got, want := readFile(t, path), "# release notes for v2\n\n- [ ] Task\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
	if row, ok := m.selectedNavigationRow(); !ok || row.todo.Category != "release notes for v2" {
		t.Fatalf("the moved task is not selected: %+v", row)
	}
}

func TestEditFormSavesASpacedCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("# release notes\n\n- [ ] Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m = press(m, "enter")
	if m.general.open.name != "release notes" {
		t.Fatalf("enter did not open the spaced category: %+v", m.general.open)
	}
	m = press(m, "e")
	f := form(t, m)
	if got := f.scope.Value(); got != "release notes" {
		t.Fatalf("edit form category = %q", got)
	}
	f.focusField(scopeField)
	for _, key := range []string{" ", "v", "2"} {
		updated, _ := m.Update(tea.KeyPressMsg{Code: []rune(key)[0], Text: key})
		m = updated.(*model)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if isOpen[*taskModal](m) {
		t.Fatalf("edit was not saved: %q", form(t, m).err)
	}
	if got, want := readFile(t, path), "# release notes v2\n\n- [ ] Task\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestEnterEditsAndDoneOrSpaceTogglesTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] First task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).title.Value() != "First task" || m.tasks.general[0].Done {
		t.Fatal("Enter did not open the selected task for editing")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(*model)
	if !m.tasks.general[0].Done {
		t.Fatal("d did not complete the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if m.tasks.general[0].Done {
		t.Fatal("Space did not reopen the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || m.focus != detailPane {
		t.Fatal("Enter from details did not open the edit form")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || m.all == nil {
		t.Fatal("Enter from All tasks did not open the edit form")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); !ok || !selected.Done {
		t.Fatal("d did not complete the selected task in All tasks")
	}
}

func TestPriorityChangeKeepsMovedTaskSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Urgent !high\n\n- [ ] Routine !medium\n\n- Bare\n\n- [ ] Another unprioritized\n\n# Docs\n\n- [ ] Document it\n\n# Tests\n\n- [ ] Test it\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.general.cursor = 4 // Two category rows, then the three generic tasks.
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Bare" {
		t.Fatalf("setup selected %+v", selected)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if !ok || selected.Text != "Bare" || selected.Priority != "high" {
		t.Fatalf("first p selected %+v", selected)
	}
	if rows := m.rows(generalPane); rows[2].todo.Text != "Urgent" || rows[3].todo.Text != "Routine" || rows[4].todo.Text != "Bare" || rows[5].todo.Text != "Another unprioritized" || m.general.cursor != 4 {
		t.Fatalf("priority edit moved the highlighted task: cursor %d, rows %+v", m.general.cursor, rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if !ok || selected.Text != "Bare" || selected.Priority != "medium" {
		t.Fatalf("second p selected %+v", selected)
	}
	for _, want := range []store.Priority{store.PriorityLow, store.PriorityNone, store.PriorityHigh} {
		updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		m = updated.(*model)
		selected, ok = m.selectedTask()
		if !ok || selected.Text != "Bare" || selected.Priority != want {
			t.Fatalf("cycling to %q selected %+v", want, selected)
		}
		if rows := m.rows(generalPane); rows[4].todo.Text != "Bare" || rows[5].todo.Text != "Another unprioritized" || m.general.cursor != 4 {
			t.Fatalf("cycling to %q moved the row: cursor %d, rows %+v", want, m.general.cursor, rows)
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(*model)
	if got := indexTitles(m.all.sorted(m.tasks.all)); !strings.HasPrefix(got, "Urgent,Routine,Bare,Another unprioritized") {
		t.Fatalf("opening the full list unexpectedly resorted tasks: %q", got)
	}
	for range 3 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
		m = updated.(*model)
	}
	if got := indexTitles(m.all.sorted(m.tasks.all)); !strings.HasPrefix(got, "Urgent,Routine,Bare,Another unprioritized") {
		t.Fatalf("cycling the full-list sort back to priority moved tasks before a reload: %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if rows := m.rows(generalPane); rows[3].todo.Text != "Routine" || rows[4].todo.Text != "Bare" || !rows[4].todo.Done || rows[5].todo.Text != "Another unprioritized" || m.general.cursor != 4 {
		t.Fatalf("completed task did not stay in place and selected: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(*model)
	if rows := m.rows(generalPane); rows[3].todo.Text != "Routine" || rows[4].todo.Text != "Another unprioritized" || rows[5].todo.Text != "Bare" {
		t.Fatalf("reload did not leave completed tasks last: %+v", rows)
	}
	items, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Text == "Routine" && item.Priority != "medium" {
			t.Fatalf("p changed the wrong task: %+v", items)
		}
	}
}

func TestPriorityChangeKeepsMovedBranchTaskSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# Branches\n\n## main\n\n- [ ] Existing high !high\n\n- [ ] Moved task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{Branch: "main"}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.enterSelectedGroup()
	m.branch.cursor = 1
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Moved task" || selected.Priority != "high" {
		t.Fatalf("branch selection moved to %+v", selected)
	}
}

func TestPriorityChangeKeepsCategoryTaskWithDetailsSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# Tests\n\n- [ ] High task !high\n\n- [x] Detailed task !low\n\n  This task has details.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	m.general.cursor = 1
	for _, want := range []store.Priority{store.PriorityNone, store.PriorityHigh, store.PriorityMedium, store.PriorityLow} {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		m = updated.(*model)
		selected, ok := m.selectedTask()
		if !ok || selected.Text != "Detailed task" || selected.Priority != want || selected.Details != "This task has details." {
			t.Fatalf("cycling to %q selected %+v", want, selected)
		}
	}
}

func TestPrioritySortHappensOnLoadAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] No priority\n\n- [ ] High priority !high\n\n- [ ] Medium priority !medium\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.rows(generalPane); rows[0].todo.Text != "High priority" || rows[1].todo.Text != "Medium priority" || rows[2].todo.Text != "No priority" {
		t.Fatalf("startup order = %+v", rows)
	}
	m.general.cursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	if rows := m.rows(generalPane); rows[2].todo.Text != "No priority" || m.general.cursor != 2 {
		t.Fatalf("priority edit resorted rows: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(*model)
	if rows := m.rows(generalPane); rows[0].todo.Text != "No priority" || rows[1].todo.Text != "High priority" || rows[2].todo.Text != "Medium priority" {
		t.Fatalf("reload order = %+v", rows)
	}
}

func TestChangedTaskErrorLeavesUIHintsToCaller(t *testing.T) {
	if strings.Contains(store.ErrTaskChanged.Error(), "press") {
		t.Fatalf("store error mentions a UI key: %q", store.ErrTaskChanged)
	}
	if got := errorStatus(fmt.Errorf("save: %w", store.ErrTaskChanged)); !strings.Contains(got, "press r to reload") {
		t.Fatalf("TUI status lost the reload hint: %q", got)
	}
}

func TestOnlyCOpensCategoryInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, index := range []bool{false, true} {
		m, err := newModel(path, project.Context{}, testFiles())
		if err != nil {
			t.Fatal(err)
		}
		if index {
			m.openAllTasks()
		}
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		if isOpen[*categoryPrompt](updated.(*model)) {
			t.Fatalf("l opened the category input (index=%v)", index)
		}
		updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
		if !isOpen[*categoryPrompt](updated.(*model)) {
			t.Fatalf("c did not open the category input (index=%v)", index)
		}
	}
	if help := ansi.Strip(renderHelp(ui.NewTheme(true))); strings.Contains(help, "c l") {
		t.Fatalf("help still lists l: %s", help)
	}
}

// taskOrder is each row's task in the focused list as text:line, marked ✓
// when done and with its priority when it has one.
func taskOrder(m *model) []string {
	var order []string
	for _, row := range m.rows(m.focus) {
		task := fmt.Sprintf("%s:%d", row.todo.Text, row.todo.Line)
		if row.todo.Done {
			task += "✓"
		}
		if row.todo.Priority != store.PriorityNone {
			task += "!" + string(row.todo.Priority)
		}
		order = append(order, task)
	}
	return order
}

func TestWritesKeepIdenticalTasksInPlace(t *testing.T) {
	const content = "- [ ] scan\n\n- [ ] list\n\n- [ ] scan\n\n- [ ] need\n"
	type step struct {
		key    string
		file   string
		rows   []string
		cursor int
	}
	for _, item := range []struct {
		name   string
		cursor int
		steps  []step
	}{
		{"done and reopened", 0, []step{
			{"d", "- [x] scan\n\n- [ ] list\n\n- [ ] scan\n\n- [ ] need\n", []string{"scan:0✓", "list:2", "scan:4", "need:6"}, 0},
			{"d", content, []string{"scan:0", "list:2", "scan:4", "need:6"}, 0},
		}},
		{"priority", 2, []step{
			{"p", "- [ ] scan !high\n\n- [ ] scan\n\n- [ ] list\n\n- [ ] need\n", []string{"scan:2", "list:4", "scan:0!high", "need:6"}, 2},
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			m, err := newModel(path, project.Context{}, testFiles())
			if err != nil {
				t.Fatal(err)
			}
			m.general.cursor = item.cursor
			for _, s := range item.steps {
				m = press(m, s.key)
				if got := fileContent(t, path); got != s.file {
					t.Fatalf("%s wrote %q, want %q", s.key, got, s.file)
				}
				if got := taskOrder(m); !slices.Equal(got, s.rows) || m.general.cursor != s.cursor {
					t.Fatalf("after %s rows = %v with the cursor on %d, want %v on %d", s.key, got, m.general.cursor, s.rows, s.cursor)
				}
			}
		})
	}
}

func TestNewTasksGoAboveDoneTasksUntilReload(t *testing.T) {
	titles := func(m *model) string {
		var names []string
		for _, row := range m.rows(generalPane) {
			names = append(names, row.todo.Text)
		}
		return strings.Join(names, ",")
	}
	for _, item := range []struct{ name, content, title, want string }{
		{"after the open tasks", "# auth\n\n- [ ] One\n- [ ] Two\n- [x] Done\n", "New", "One,Two,New,Done"},
		{"before the done tasks", "# auth\n\n- [x] Done\n", "New", "New,Done"},
		{"with a priority", "# auth\n\n- [ ] One\n", "New !high", "One,New"},
	} {
		t.Run(item.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte(item.content), 0644); err != nil {
				t.Fatal(err)
			}
			m, err := newModel(path, project.Context{}, testFiles())
			if err != nil {
				t.Fatal(err)
			}
			m = press(m, "enter")
			m = press(m, "a")
			form(t, m).title.SetValue(item.title)
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
			if got := titles(m); got != item.want {
				t.Fatalf("rows after adding = %q, want %q", got, item.want)
			}
			if selected, ok := m.selectedTask(); !ok || selected.Text != "New" {
				t.Fatalf("new task not selected: %+v", selected)
			}
		})
	}
}

func TestUpAndDownWrapPastTheEndsOfAList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] One\n\n  Details.\n- [ ] Two\n- [ ] Three\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key    string
		cursor int
	}{
		{"k", 2}, // up from the top wraps to the end
		{"j", 0}, // and down from the end to the top
		{"j", 1},
		{"k", 0},
	} {
		if m = press(m, step.key); m.general.cursor != step.cursor {
			t.Fatalf("after %s the cursor is on %d, want %d", step.key, m.general.cursor, step.cursor)
		}
	}

	m = press(m, "i")
	if m = press(m, "k"); m.all.cursor != 2 {
		t.Fatalf("All tasks did not wrap: cursor %d", m.all.cursor)
	}
	m = press(m, "esc")

	// A task's details scroll, and don't wrap.
	m.general.cursor = 0
	pressKey(t, m, "right")
	if m = press(m, "k"); m.focus != detailPane || m.detailScroll != 0 || m.general.cursor != 0 {
		t.Fatalf("up on a task's details moved: focus %v, scroll %d, cursor %d", m.focus, m.detailScroll, m.general.cursor)
	}
}
