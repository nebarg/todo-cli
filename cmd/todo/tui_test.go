package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/store"
)

func TestChangingCategoryKeepsTaskSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("## General\n\n### @auth\n\n- [ ] Fix login\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = opened.(*model)
	if !m.categoryInput {
		t.Fatal("c did not open category editing")
	}
	if got := m.input.Prompt; got != "Category: " {
		t.Fatalf("category prompt = %q", got)
	}
	m.input.SetValue("backend")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if m.categoryInput || !ok || selected.Text != "Fix login" || selected.Category != "backend" {
		t.Fatalf("recategorising lost selection: %+v", m.generalRows())
	}
}

func TestCategoryInputBlocksSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startCategoryInput()
	m = opened.(*model)
	m.input.SetValue("a")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = updated.(*model)
	if got := m.input.Value(); got != "a" {
		t.Fatalf("space key changed category input to %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: " category"})
	m = updated.(*model)
	if got := m.input.Value(); got != "acategory" {
		t.Fatalf("multi-character input kept a space: %q", got)
	}
	updated, _ = m.Update(tea.PasteMsg{Content: " more words"})
	m = updated.(*model)
	if got := m.input.Value(); got != "acategorymorewords" {
		t.Fatalf("pasted input kept spaces: %q", got)
	}
}

func TestEnterEditsAndDoneOrSpaceTogglesTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] First task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.modal == nil || m.modal.title.Value() != "First task" || m.general[0].Done {
		t.Fatal("Enter did not open the selected task for editing")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(*model)
	if !m.general[0].Done {
		t.Fatal("d did not complete the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if m.general[0].Done {
		t.Fatal("Space did not reopen the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.modal == nil || m.focus != detailPane {
		t.Fatal("Enter from details did not open the edit form")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.modal == nil || !m.indexMode {
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

func TestEditorCommandUsesLineNumber(t *testing.T) {
	t.Setenv("VISUAL", "vi")
	cmd := editorProcess("/tmp/TODO.md", 12)
	if got := strings.Join(cmd.Args, " "); got != "vi +12 /tmp/TODO.md" {
		t.Fatalf("vi command = %q", got)
	}
	t.Setenv("VISUAL", "code")
	cmd = editorProcess("/tmp/TODO.md", 12)
	if got := strings.Join(cmd.Args, " "); got != "code --wait --goto /tmp/TODO.md:12" {
		t.Fatalf("code command = %q", got)
	}
}

func TestPriorityChangeKeepsMovedTaskSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Urgent\n  - Priority: High\n\n- [ ] Routine\n  - Priority: Medium\n\n- Bare\n\n- [ ] Another unprioritized\n\n# Docs\n\n- [ ] Document it\n\n# Tests\n\n- [ ] Test it\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.generalCursor = 4 // Two category rows, then the three generic tasks.
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
	if rows := m.generalRows(); rows[2].todo.Text != "Urgent" || rows[3].todo.Text != "Routine" || rows[4].todo.Text != "Bare" || rows[5].todo.Text != "Another unprioritized" || m.generalCursor != 4 {
		t.Fatalf("priority edit moved the highlighted task: cursor %d, rows %+v", m.generalCursor, rows)
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
		if rows := m.generalRows(); rows[4].todo.Text != "Bare" || rows[5].todo.Text != "Another unprioritized" || m.generalCursor != 4 {
			t.Fatalf("cycling to %q moved the row: cursor %d, rows %+v", want, m.generalCursor, rows)
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(*model)
	if got := indexTitles(m.indexTasks()); !strings.HasPrefix(got, "Urgent,Routine,Bare,Another unprioritized") {
		t.Fatalf("opening the full list unexpectedly resorted tasks: %q", got)
	}
	for range 3 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
		m = updated.(*model)
	}
	if got := indexTitles(m.indexTasks()); !strings.HasPrefix(got, "Urgent,Bare,Routine") {
		t.Fatalf("cycling the full-list sort back to priority failed: %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if rows := m.generalRows(); rows[3].todo.Text != "Routine" || rows[4].todo.Text != "Another unprioritized" || rows[5].todo.Text != "Bare" || m.generalCursor != 5 {
		t.Fatalf("completed task did not move after open tasks while staying selected: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(*model)
	if rows := m.generalRows(); rows[3].todo.Text != "Routine" || rows[4].todo.Text != "Another unprioritized" || rows[5].todo.Text != "Bare" {
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
	content := "# Branches\n\n## main\n\n- [ ] Existing high\n  - Priority: High\n\n- [ ] Moved task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.enterSelectedGroup()
	m.branchCursor = 1
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Moved task" || selected.Priority != "high" {
		t.Fatalf("branch selection moved to %+v", selected)
	}
}

func TestPriorityChangeKeepsCategoryTaskWithDetailsSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# Tests\n\n- [ ] High task\n  - Priority: High\n\n- [x] Detailed task\n  - Priority: Low\n\n  This task has details.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	m.generalCursor = 1
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
	content := "- [ ] No priority\n\n- [ ] High priority\n  - Priority: High\n\n- [ ] Medium priority\n  - Priority: Medium\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.generalRows(); rows[0].todo.Text != "High priority" || rows[1].todo.Text != "Medium priority" || rows[2].todo.Text != "No priority" {
		t.Fatalf("startup order = %+v", rows)
	}
	m.generalCursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	if rows := m.generalRows(); rows[2].todo.Text != "No priority" || m.generalCursor != 2 {
		t.Fatalf("priority edit resorted rows: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(*model)
	if rows := m.generalRows(); rows[0].todo.Text != "No priority" || rows[1].todo.Text != "High priority" || rows[2].todo.Text != "Medium priority" {
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
