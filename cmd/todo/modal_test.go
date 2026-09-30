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
	"github.com/nebarg/todo-cli/internal/store"
)

func TestAddingWithinGroupsKeepsScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	project := testGitProject(t, dir, "main", "feature/login")
	content := "# auth\n\n- [ ] Existing\n\n# Branches\n\n## feature/login\n\n- [ ] Branch task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	opened, _ := m.startTaskModal(modalAddGeneral)
	m = opened.(*model)
	if m.modal.addCategory != "auth" {
		t.Fatal("add form did not inherit selected category")
	}
	m.modal.title.SetValue("New auth task")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if len(m.generalRows()) != 2 || !taskInCategory(m.generalRows()[1].todo, "auth") || m.generalRows()[1].todo.Text != "New auth task" || m.generalCursor != 1 {
		t.Fatalf("new task was not added to category: %+v", m.generalRows())
	}
	m.focus = branchPane
	m.enterSelectedGroup()
	opened, _ = m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	if m.modal.addBranch != "feature/login" {
		t.Fatalf("add form chose %q instead of selected branch", m.modal.addBranch)
	}
	m.modal.title.SetValue("New branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.branchFilter != "feature/login" || len(m.branchRows()) != 2 || m.branchRows()[1].todo.Text != "New branch task" {
		t.Fatalf("new task was not added to branch: %+v", m.branchRows())
	}
	opened, _ = m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	if m.modal.addCategory != "" {
		t.Fatal("branch add form unexpectedly inherited a category")
	}
	m.modal.title.SetValue("Another branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if len(m.branchRows()) != 3 || m.branchRows()[m.branchCursor].todo.Text != "Another branch task" {
		t.Fatalf("new task was not added inside branch: %+v", m.branchRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updated.(*model)
	if m.branchFilter != "" || m.branchRows()[m.branchCursor].name != "feature/login" {
		t.Fatal("v did not return to the branch list")
	}
}

func TestAddGeneralRootDoesNotInheritSelectedCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# test\n\n- [ ] Existing test task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.generalRows(); len(rows) != 1 || rows[0].kind != rowCategory {
		t.Fatalf("expected only the category row, got %+v", rows)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addCategory != "" {
		t.Fatalf("root add inherited selected category: %+v", m.modal)
	}
	m.modal.title.SetValue("General task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.generalCategory != "" {
		t.Fatalf("root add opened category %q", m.generalCategory)
	}
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "General task" || selected.Category != "" {
		t.Fatalf("new general task was not selected or was categorised: %+v", m.generalRows())
	}
	m.generalCursor = 0
	if !m.enterSelectedGroup() {
		t.Fatal("could not open the test category")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addCategory != "test" {
		t.Fatalf("category add did not inherit opened category: %+v", m.modal)
	}
	m.modal.title.SetValue("Another test task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if m.generalCategory != "test" || !ok || selected.Text != "Another test task" || selected.Category != "test" {
		t.Fatalf("new category task was not selected or categorised: %+v", m.generalRows())
	}
}

func TestAddShortcutUsesCurrentBranchAtRootAndOpenedBranch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	project := testGitProject(t, dir, "main", "feature/a")
	content := "# Branches\n\n## feature/a\n\n- [ ] Existing feature task\n\n## main\n\n- [ ] Existing main task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.branchCursor = 0 // feature/a is selected; main is the current Git branch.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.mode != modalAddBranch || m.modal.addBranch != "main" {
		t.Fatalf("a did not target the current branch from the branch list: %+v", m.modal)
	}
	m.modal.title.SetValue("First main task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.branchFilter != "main" {
		t.Fatalf("add did not open current branch: %q", m.branchFilter)
	}
	m.branchFilter = ""
	m.branchCursor = 0
	m.enterSelectedGroup()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addBranch != "feature/a" {
		t.Fatalf("a did not target the opened branch: %+v", m.modal)
	}
	m.modal.title.SetValue("Second feature task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Second feature task" || selected.Branch != "feature/a" {
		t.Fatalf("new task was not selected in feature/a: %+v", m.branchRows())
	}
	m.focus = generalPane
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(*model)
	if m.modal == nil || m.modal.mode != modalAddBranch || m.modal.addBranch != "main" {
		t.Fatalf("b no longer explicitly targets the current Git branch: %+v", m.modal)
	}
}

func TestBranchAddReadsGitBranchWhenFormOpens(t *testing.T) {
	repo := t.TempDir()
	project := testGitProject(t, repo, "feature/old", "feature/new")
	path := filepath.Join(repo, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/old\n\n- [ ] Existing task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	if _, err := gitOutput(repo, "symbolic-ref", "HEAD", "refs/heads/feature/new"); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addBranch != "feature/new" || m.modal.scope.Value() != "feature/new" {
		t.Fatalf("branch list used a cached or selected branch: %+v", m.modal)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	m.branchFilter = "feature/old"
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addBranch != "feature/old" {
		t.Fatalf("opened branch did not override current Git branch: %+v", m.modal)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	m.indexMode = true
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addBranch != "feature/new" {
		t.Fatalf("All Tasks inherited a hidden branch filter: %+v", m.modal)
	}
}

func TestAddFormCreatesCategoryAndMarkdownBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(*model)
	if m.modal.field != 1 {
		t.Fatal("tab did not focus category field")
	}
	m.modal.resize(56, 16)
	if view := ansi.Strip(m.modal.render(54, 12)); !strings.Contains(view, "Category") || !strings.Contains(view, "Optional category") {
		t.Fatalf("category field is not visible in the small add form: %s", view)
	}
	m.modal.title.SetValue("New categorised task")
	updated, _ = m.Update(tea.PasteMsg{Content: "@new area"})
	m = updated.(*model)
	if m.modal.scope.Value() != "@newarea" {
		t.Fatalf("pasted category kept spaces: %q", m.modal.scope.Value())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if m.modal != nil || !ok || selected.Category != "newarea" || m.generalCategory != "newarea" {
		t.Fatalf("new category was not created and opened: %+v", m.generalRows())
	}
	m.project = testGitProject(t, filepath.Dir(path), "feature/new", "feature/index")
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.mode != modalAddBranch || m.modal.scope.Value() != "feature/new" {
		t.Fatalf("branch form did not prefill the current Git branch: %+v", m.modal)
	}
	m.modal.resize(56, 16)
	if view := ansi.Strip(m.modal.render(54, 12)); !strings.Contains(view, "Branch") || !strings.Contains(view, "feature/new") {
		t.Fatalf("branch field is not visible in the small add form: %s", view)
	}
	m.modal.title.SetValue("New branch task")
	m.modal.scope.SetValue("not-a-branch")
	m.modal.resetBranchCursor()
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.modal == nil || m.modal.err != "choose an existing local Git branch" {
		t.Fatalf("unknown branch was accepted: %+v", m.modal)
	}
	m.modal.scope.SetValue("feature/new")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if m.modal != nil || !ok || selected.Branch != "feature/new" || m.branchFilter != "feature/new" {
		t.Fatalf("new Markdown branch was not created and opened: %+v", m.branchRows())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# newarea") || !strings.Contains(string(data), "# Branches\n\n## feature/new") {
		t.Fatalf("new category or branch heading missing: %s", data)
	}
	m.indexMode = true
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(*model)
	if m.modal == nil || m.modal.mode != modalAddBranch || m.modal.scope.Value() != "feature/new" {
		t.Fatal("All Tasks did not use the current Git branch")
	}
	m.modal.title.SetValue("Task from All tasks")
	m.modal.scope.SetValue("feature/index")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.indexMode || m.branchFilter != "feature/index" {
		t.Fatalf("branch add from All tasks did not open the new section: %q", m.branchFilter)
	}
}

func TestAddFormAcceptsSymbolCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startTaskModal(modalAddGeneral)
	m = opened.(*model)
	m.modal.title.SetValue("Version task")
	m.modal.scope.SetValue("+v1")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if m.modal != nil || !ok || selected.Category != "+v1" || m.generalCategory != "+v1" {
		t.Fatalf("symbol category did not save from the form: %+v", m.generalRows())
	}
}

func TestBranchPickerSearchAndSelection(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/auth", "fix/auth", "feature/ui", "main2")
	m, err := newModel(filepath.Join(dir, "todo.md"), project)
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	m.modal.title.SetValue("Check auth")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(*model)
	if m.modal.field != 1 || m.modal.scope.Value() != "main" {
		t.Fatalf("branch search was not focused with the current branch: %+v", m.modal)
	}
	updated, _ = m.Update(tea.PasteMsg{Content: "AUTH"})
	m = updated.(*model)
	if got := m.modal.scope.Value(); got != "AUTH" {
		t.Fatalf("search did not replace the prefilled branch: %q", got)
	}
	if got := m.modal.matchingBranches(); len(got) != 2 || got[0] != "feature/auth" || got[1] != "fix/auth" {
		t.Fatalf("unexpected case-insensitive matches: %v", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(*model)
	if m.modal.branchCursor != 1 {
		t.Fatalf("down did not select the second branch: %d", m.modal.branchCursor)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(*model)
	if m.modal.branchCursor != 0 || m.modal.scope.Value() != "AUTH" || m.modal.field != 1 {
		t.Fatalf("down at the end did not wrap without selecting: %+v", m.modal)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(*model)
	if m.modal.branchCursor != 1 || m.modal.field != 1 {
		t.Fatalf("up at the start did not wrap: %+v", m.modal)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.modal.field != 2 || m.modal.scope.Value() != "fix/auth" {
		t.Fatalf("enter did not accept the selected branch: %+v", m.modal)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if m.modal != nil || !ok || selected.Branch != "fix/auth" {
		t.Fatalf("task was not saved under the selected branch: %+v", selected)
	}
	opened, _ = m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	m.modal.scope.SetValue("main")
	m.modal.resetBranchCursor()
	if got := m.modal.matchingBranches(); len(got) != 2 || got[0] != "main" || got[1] != "main2" {
		t.Fatalf("exact and prefix matches were not ordered: %v", got)
	}
	m.modal.branchCursor = 1
	if got := m.modal.chosenBranch(); got != "main2" {
		t.Fatalf("highlighted branch lost to exact search text: %q", got)
	}
}

func TestBranchCreatedAfterStartupIsNotMissing(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main")
	m, err := newModel(filepath.Join(dir, "todo.md"), project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(dir, "branch", "feature/new"); err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	m.modal.title.SetValue("Late branch task")
	m.modal.scope.SetValue("new")
	m.modal.resetBranchCursor()
	if got := m.modal.matchingBranches(); len(got) != 1 || got[0] != "feature/new" {
		t.Fatalf("branch search used a stale branch list: %v", got)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.modal != nil {
		t.Fatalf("save failed: %s", m.modal.err)
	}
	m.branchFilter = ""
	if rows := m.branchRows(); len(rows) != 1 || rows[0].name != "feature/new" || rows[0].missingGitBranch {
		t.Fatalf("new branch was marked missing: %+v", rows)
	}
}

func TestEditModalRefreshesBranchState(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/x")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/x\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.branchFilter = "feature/x"
	if _, err := gitOutput(dir, "branch", "-D", "feature/x"); err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startTaskModal(modalEdit)
	m = opened.(*model)
	if m.modal != nil || m.status != missingBranchStatus {
		t.Fatalf("edit opened for a branch deleted after startup: status %q", m.status)
	}
	if _, err := gitOutput(dir, "branch", "feature/x"); err != nil {
		t.Fatal(err)
	}
	opened, _ = m.startTaskModal(modalEdit)
	m = opened.(*model)
	if m.modal == nil || m.branchMissing("feature/x") {
		t.Fatalf("edit stayed blocked after the branch was restored: status %q", m.status)
	}
}

func TestBranchPickerRejectsDeletedBranch(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/old")
	path := filepath.Join(dir, "todo.md")
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	m.modal.title.SetValue("Do not save")
	m.modal.scope.SetValue("feature/old")
	if _, err := gitOutput(dir, "branch", "-D", "feature/old"); err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.modal == nil || m.modal.err != "branch \"feature/old\" no longer exists locally" {
		t.Fatalf("deleted branch was accepted: %+v", m.modal)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("rejected task changed the TODO file: %v", err)
	}
}

func TestBranchPickerFitsCompactAndRegularModals(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/auth", "feature/ui", "fix/search")
	m, err := newModel(filepath.Join(dir, "todo.md"), project)
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	m.modal.scope.SetValue("")
	m.modal.branchCursor = 0
	suggestions := m.modal.branchSuggestions(24)
	if len(suggestions) != 2 || !strings.Contains(ansi.Strip(suggestions[0]), "feature/auth") || !strings.Contains(ansi.Strip(suggestions[1]), "feature/ui") || strings.Contains(strings.Join(suggestions, ""), "fix/search") || !strings.Contains(strings.Join(suggestions, ""), "┃") {
		t.Fatalf("picker did not show two rows with a scroll indicator: %q", suggestions)
	}
	m.modal.scope.SetValue("feature/")
	m.modal.resetBranchCursor()
	if suggestions = m.modal.branchSuggestions(24); len(suggestions) != 2 || strings.Contains(strings.Join(suggestions, ""), "┃") || strings.Contains(strings.Join(suggestions, ""), "│") {
		t.Fatalf("scroll indicator shown for only two matches: %q", suggestions)
	}
	m.modal.scope.SetValue("")
	m.modal.branchCursor = 0
	for _, size := range [][2]int{{80, 24}, {56, 19}, {56, 16}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m.modal.resize(size[0], size[1])
			if size[1] == 24 && lipgloss.Height(m.modal.details.View()) != 5 {
				t.Errorf("regular details box did not gain one line: height %d", lipgloss.Height(m.modal.details.View()))
			}
			width, height := m.modal.dimensions(size[0], size[1])
			view := m.modal.render(width, height)
			if got := lipgloss.Width(view); got != width {
				t.Errorf("picker width at %dx%d = %d, want %d", size[0], size[1], got, width)
			}
			if got := lipgloss.Height(view); got != height {
				t.Errorf("picker height at %dx%d = %d, want %d", size[0], size[1], got, height)
			}
			plain := ansi.Strip(view)
			if !strings.Contains(plain, "Branch") || !strings.Contains(plain, branchIcon+" feature/auth") || !strings.Contains(plain, branchIcon+" feature/ui") || !strings.Contains(plain, "Details") {
				t.Errorf("picker content missing at %dx%d: %s", size[0], size[1], plain)
			}
		})
	}
}

func TestTaskModalAddsAndEditsDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = opened.(*model)
	if m.modal == nil || m.modal.field != 0 {
		t.Fatal("add did not open the title field")
	}
	if help := ansi.Strip(m.modal.render(76, 20)); !strings.Contains(help, "↑/↓/tab navigate · ctrl+enter save · esc cancel") || !strings.Contains(help, "Category") {
		t.Fatalf("add form has wrong labels or help: %s", help)
	} else {
		for line := range strings.SplitSeq(help, "\n") {
			if strings.Trim(line, " │") == "Task" {
				t.Fatal("add form repeats the task label")
			}
		}
	}
	if rendered := m.modal.render(76, 20); !strings.Contains(rendered, mutedStyle.Render("Category")) {
		t.Fatal("unfocused category label is not muted")
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m.width, m.height = size[0], size[1]
			m.modal.resize(m.width, m.height)
			modalWidth, modalHeight := m.modal.dimensions(m.width, m.height)
			if rendered := m.modal.render(modalWidth, modalHeight); lipgloss.Width(rendered) != modalWidth || lipgloss.Height(rendered) != modalHeight {
				t.Errorf("modal itself overflows at %dx%d: %dx%d", size[0], size[1], lipgloss.Width(rendered), lipgloss.Height(rendered))
			}
			view := m.View().Content
			if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
				t.Errorf("modal size at %dx%d = %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
			}
			if !strings.Contains(ansi.Strip(view), "Add general task") {
				t.Error("modal heading missing")
			}
		})
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if m.modal == nil || m.modal.err == "" {
		t.Fatal("empty title should keep the form open with an error")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = opened.(*model)
	if m.modal.title.Value() != "F" {
		t.Fatalf("title field ignored typing: %q", m.modal.title.Value())
	}
	m.modal.title.SetValue("First task")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	if m.modal.field != 1 {
		t.Fatal("down arrow did not focus category field")
	}
	if help := ansi.Strip(m.modal.render(76, 20)); !strings.Contains(help, "↑/↓/tab navigate · ctrl+enter save · esc cancel") {
		t.Fatalf("category help changed by focus: %s", help)
	}
	if rendered := m.modal.render(76, 20); !strings.Contains(rendered, titleStyle.Render("Category")) {
		t.Fatal("focused category label is not highlighted")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(*model)
	if m.modal.field != 0 {
		t.Fatal("up arrow on the first details line did not focus title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	if m.modal.field != 1 {
		t.Fatal("down arrow did not return to category field")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	if m.modal.field != 2 {
		t.Fatal("down arrow did not focus details")
	}
	if help := ansi.Strip(m.modal.render(76, 20)); !strings.Contains(help, "↑/↓/tab navigate · ctrl+enter save · esc cancel") {
		t.Fatalf("details help changed by focus: %s", help)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	m = opened.(*model)
	if m.modal.details.Value() != "X" {
		t.Fatalf("details field ignored typing: %q", m.modal.details.Value())
	}
	m.modal.details.SetValue("Added in modal.\n- [ ] Nested step")
	if m.modal.details.Line() != 1 {
		t.Fatalf("details cursor should start on the final line, got %d", m.modal.details.Line())
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(*model)
	if m.modal.field != 2 || m.modal.details.Line() != 0 {
		t.Fatal("up arrow should move within multiline details before returning to title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(*model)
	if m.modal.field != 1 {
		t.Fatal("up arrow on first details line did not focus category field")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if m.modal != nil || len(m.general) != 1 || m.general[0].Details != "Added in modal.\n- [ ] Nested step" {
		t.Fatalf("task was not saved: %+v", m.general)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = opened.(*model)
	if m.modal == nil || m.modal.title.Value() != "First task" {
		t.Fatal("edit did not reopen the task in the modal")
	}
	m.modal.title.SetValue("Renamed task")
	m.modal.details.SetValue("Revised details.")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if len(m.general) != 1 || m.general[0].Text != "Renamed task" || m.general[0].Details != "Revised details." {
		t.Fatalf("edit was not saved: %+v", m.general)
	}
}

func TestTwoLineTaskInputStaysOneMarkdownTask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 100, 30
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = opened.(*model)
	if got := lipgloss.Height(m.modal.title.View()); got != 2 {
		t.Fatalf("task input is %d lines, want 2", got)
	}
	m.modal.title.SetValue("Fix login\nredirect")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Fix login redirect" {
		t.Fatalf("two-line Task was not saved as one title: %+v", m.generalRows())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "- [ ] Fix login redirect\n" {
		t.Fatalf("two-line Task changed the Markdown structure: %q", data)
	}
}

func TestModalDetailsFillRemainingHeight(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {80, 24}, {80, 18}, {80, 17}, {60, 16}, {56, 19}, {56, 16}} {
		for _, mode := range []modalMode{modalAddGeneral, modalAddBranch, modalEdit} {
			t.Run(fmt.Sprintf("%dx%d mode %d", size[0], size[1], mode), func(t *testing.T) {
				m := &model{width: size[0], height: size[1], general: []store.Task{{Text: "Task"}}}
				opened, _ := m.startTaskModal(mode)
				m = opened.(*model)
				width, height := m.modal.dimensions(size[0], size[1])
				lines := strings.Split(ansi.Strip(m.modal.render(width, height)), "\n")
				if len(lines) != height {
					t.Fatalf("modal is %d lines, want %d", len(lines), height)
				}
				if help := lines[len(lines)-2]; !strings.Contains(help, "ctrl+enter save") {
					t.Fatalf("help is not on the last content row, so details left a gap or overflowed: %q", help)
				}
			})
		}
	}
}
