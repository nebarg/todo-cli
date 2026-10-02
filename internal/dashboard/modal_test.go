package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/project/projecttest"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

func TestAddingWithinGroupsKeepsScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	repo := projecttest.Repo(t, dir, "main", "feature/login")
	content := "# auth\n\n- [ ] Existing\n\n# Branches\n\n## feature/login\n\n- [ ] Branch task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	m.startTaskModal(modalAddGeneral)
	if form(t, m).target.Category != "auth" {
		t.Fatal("add form did not inherit selected category")
	}
	form(t, m).title.SetValue("New auth task")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if len(m.rows(generalPane)) != 2 || !taskInCategory(m.rows(generalPane)[1].todo, "auth") || m.rows(generalPane)[1].todo.Text != "New auth task" || m.general.cursor != 1 {
		t.Fatalf("new task was not added to category: %+v", m.rows(generalPane))
	}
	m.focus = branchPane
	m.enterSelectedGroup()
	m.startTaskModal(modalAddBranch)
	if form(t, m).target.Branch != "feature/login" {
		t.Fatalf("add form chose %q instead of selected branch", form(t, m).target.Branch)
	}
	form(t, m).title.SetValue("New branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.branch.open.name != "feature/login" || len(m.rows(branchPane)) != 2 || m.rows(branchPane)[1].todo.Text != "New branch task" {
		t.Fatalf("new task was not added to branch: %+v", m.rows(branchPane))
	}
	m.startTaskModal(modalAddBranch)
	if form(t, m).target.Category != "" {
		t.Fatal("branch add form unexpectedly inherited a category")
	}
	form(t, m).title.SetValue("Another branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if len(m.rows(branchPane)) != 3 || m.rows(branchPane)[m.branch.cursor].todo.Text != "Another branch task" {
		t.Fatalf("new task was not added inside branch: %+v", m.rows(branchPane))
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(*model)
	if m.branch.open.name != "" || m.rows(branchPane)[m.branch.cursor].name != "feature/login" {
		t.Fatal("2 did not return to the branch list")
	}
}

func TestAddGeneralRootDoesNotInheritSelectedCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# test\n\n- [ ] Existing test task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.rows(generalPane); len(rows) != 1 || rows[0].kind != rowCategory {
		t.Fatalf("expected only the category row, got %+v", rows)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).target.Category != "" {
		t.Fatalf("root add inherited selected category: %+v", m.overlay)
	}
	form(t, m).title.SetValue("General task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.general.open.name != "" {
		t.Fatalf("root add opened category %q", m.general.open.name)
	}
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "General task" || selected.Category != "" {
		t.Fatalf("new general task was not selected or was categorised: %+v", m.rows(generalPane))
	}
	m.general.cursor = 0
	if _, entered := m.enterSelectedGroup(); !entered {
		t.Fatal("could not open the test category")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).target.Category != "test" {
		t.Fatalf("category add did not inherit opened category: %+v", m.overlay)
	}
	form(t, m).title.SetValue("Another test task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if m.general.open.name != "test" || !ok || selected.Text != "Another test task" || selected.Category != "test" {
		t.Fatalf("new category task was not selected or categorised: %+v", m.rows(generalPane))
	}
}

func TestAddShortcutUsesCurrentBranchAtRootAndOpenedBranch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	repo := projecttest.Repo(t, dir, "main", "feature/a")
	content := "# Branches\n\n## feature/a\n\n- [ ] Existing feature task\n\n## main\n\n- [ ] Existing main task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.branch.cursor = 0 // feature/a is selected; main is the current Git branch.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).mode != modalAddBranch || form(t, m).target.Branch != "main" {
		t.Fatalf("a did not target the current branch from the branch list: %+v", m.overlay)
	}
	form(t, m).title.SetValue("First main task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.branch.open.name != "main" {
		t.Fatalf("add did not open current branch: %q", m.branch.open.name)
	}
	m.branch.open = branchGroup("")
	m.branch.cursor = 0
	m.enterSelectedGroup()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).target.Branch != "feature/a" {
		t.Fatalf("a did not target the opened branch: %+v", m.overlay)
	}
	form(t, m).title.SetValue("Second feature task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Second feature task" || selected.Branch != "feature/a" {
		t.Fatalf("new task was not selected in feature/a: %+v", m.rows(branchPane))
	}
	m.focus = generalPane
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).mode != modalAddBranch || form(t, m).target.Branch != "main" {
		t.Fatalf("b no longer explicitly targets the current Git branch: %+v", m.overlay)
	}
}

func TestBranchAddReadsGitBranchWhenFormOpens(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "feature/old", "feature/new")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/old\n\n- [ ] Existing task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.leaveGroup()
	projecttest.Git(t, dir, "symbolic-ref", "HEAD", "refs/heads/feature/new")
	m = pressAndRun(t, m, "a")
	if !isOpen[*taskModal](m) || form(t, m).target.Branch != "feature/new" || form(t, m).scope.Value() != "feature/new" {
		t.Fatalf("branch list used a cached or selected branch: %+v", m.overlay)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	m.branch.open = branchGroup("feature/old")
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	answerGit(t, m)
	if !isOpen[*taskModal](m) || form(t, m).target.Branch != "feature/old" {
		t.Fatalf("opened branch did not override current Git branch: %+v", m.overlay)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	m.openAllTasks()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(*model)
	answerGit(t, m)
	if !isOpen[*taskModal](m) || form(t, m).target.Branch != "feature/new" {
		t.Fatalf("All Tasks inherited a hidden branch filter: %+v", m.overlay)
	}
}

func TestAddFormCreatesCategoryAndMarkdownBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(*model)
	if form(t, m).field != 1 {
		t.Fatal("tab did not focus category field")
	}
	form(t, m).resize(56, 16)
	if view := ansi.Strip(form(t, m).render(54, 12)); !strings.Contains(view, "Category") || !strings.Contains(view, "Optional category") {
		t.Fatalf("category field is not visible in the small add form: %s", view)
	}
	form(t, m).title.SetValue("New categorised task")
	updated, _ = m.Update(tea.PasteMsg{Content: "@new area"})
	m = updated.(*model)
	if form(t, m).scope.Value() != "@newarea" {
		t.Fatalf("pasted category kept spaces: %q", form(t, m).scope.Value())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if isOpen[*taskModal](m) || !ok || selected.Category != "newarea" || m.general.open.name != "newarea" {
		t.Fatalf("new category was not created and opened: %+v", m.rows(generalPane))
	}
	m.project = projecttest.Repo(t, filepath.Dir(path), "feature/new", "feature/index")
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	answerGit(t, m)
	if !isOpen[*taskModal](m) || form(t, m).mode != modalAddBranch || form(t, m).scope.Value() != "feature/new" {
		t.Fatalf("branch form did not prefill the current Git branch: %+v", m.overlay)
	}
	form(t, m).resize(56, 16)
	if view := ansi.Strip(form(t, m).render(54, 12)); !strings.Contains(view, "Branch") || !strings.Contains(view, "feature/new") {
		t.Fatalf("branch field is not visible in the small add form: %s", view)
	}
	form(t, m).title.SetValue("New branch task")
	form(t, m).scope.SetValue("not-a-branch")
	form(t, m).resetBranchCursor()
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).err != "choose an existing local Git branch" {
		t.Fatalf("unknown branch was accepted: %+v", m.overlay)
	}
	form(t, m).scope.SetValue("feature/new")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if isOpen[*taskModal](m) || !ok || selected.Branch != "feature/new" || m.branch.open.name != "feature/new" {
		t.Fatalf("new Markdown branch was not created and opened: %+v", m.rows(branchPane))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# newarea") || !strings.Contains(string(data), "# Branches\n\n## feature/new") {
		t.Fatalf("new category or branch heading missing: %s", data)
	}
	m.openAllTasks()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).mode != modalAddBranch || form(t, m).scope.Value() != "feature/new" {
		t.Fatal("All Tasks did not use the current Git branch")
	}
	form(t, m).title.SetValue("Task from All tasks")
	form(t, m).scope.SetValue("feature/index")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.all != nil || m.branch.open.name != "feature/index" {
		t.Fatalf("branch add from All tasks did not open the new section: %q", m.branch.open.name)
	}
	m.focus, m.general.open = generalPane, categoryGroup("newarea")
	m.openAllTasks()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).mode != modalAddGeneral || form(t, m).scope.Value() != "" {
		t.Fatal("a in All tasks did not open an uncategorised general add form")
	}
	form(t, m).title.SetValue("General from All tasks")
	form(t, m).scope.SetValue("docs")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); isOpen[*taskModal](m) || m.all != nil || m.general.open.name != "docs" || !ok || selected.Text != "General from All tasks" {
		t.Fatalf("general add from All tasks did not open the new task: category %q, %+v", m.general.open.name, selected)
	}
}

func TestAddFormAcceptsSymbolCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.startTaskModal(modalAddGeneral)
	form(t, m).title.SetValue("Version task")
	form(t, m).scope.SetValue("+v1")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if isOpen[*taskModal](m) || !ok || selected.Category != "+v1" || m.general.open.name != "+v1" {
		t.Fatalf("symbol category did not save from the form: %+v", m.rows(generalPane))
	}
}

func TestBranchPickerSearchAndSelection(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/auth", "fix/auth", "feature/ui", "main2")
	m, err := newModel(filepath.Join(dir, "todo.md"), repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.startTaskModal(modalAddBranch)
	form(t, m).title.SetValue("Check auth")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(*model)
	if form(t, m).field != 1 || form(t, m).scope.Value() != "main" {
		t.Fatalf("branch search was not focused with the current branch: %+v", m.overlay)
	}
	updated, _ = m.Update(tea.PasteMsg{Content: "AUTH"})
	m = updated.(*model)
	if got := form(t, m).scope.Value(); got != "AUTH" {
		t.Fatalf("search did not replace the prefilled branch: %q", got)
	}
	if got := form(t, m).matchingBranches(); len(got) != 2 || got[0] != "feature/auth" || got[1] != "fix/auth" {
		t.Fatalf("unexpected case-insensitive matches: %v", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(*model)
	if form(t, m).branchCursor != 1 {
		t.Fatalf("down did not select the second branch: %d", form(t, m).branchCursor)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(*model)
	if form(t, m).branchCursor != 0 || form(t, m).scope.Value() != "AUTH" || form(t, m).field != 1 {
		t.Fatalf("down at the end did not wrap without selecting: %+v", m.overlay)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(*model)
	if form(t, m).branchCursor != 1 || form(t, m).field != 1 {
		t.Fatalf("up at the start did not wrap: %+v", m.overlay)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if form(t, m).field != 2 || form(t, m).scope.Value() != "fix/auth" {
		t.Fatalf("enter did not accept the selected branch: %+v", m.overlay)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if isOpen[*taskModal](m) || !ok || selected.Branch != "fix/auth" {
		t.Fatalf("task was not saved under the selected branch: %+v", selected)
	}
	m.startTaskModal(modalAddBranch)
	form(t, m).scope.SetValue("main")
	form(t, m).resetBranchCursor()
	if got := form(t, m).matchingBranches(); len(got) != 2 || got[0] != "main" || got[1] != "main2" {
		t.Fatalf("exact and prefix matches were not ordered: %v", got)
	}
	form(t, m).branchCursor = 1
	if got := form(t, m).chosenBranch(); got != "main2" {
		t.Fatalf("highlighted branch lost to exact search text: %q", got)
	}
}

func TestBranchCreatedAfterStartupIsNotMissing(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main")
	m, err := newModel(filepath.Join(dir, "todo.md"), repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	projecttest.Git(t, dir, "branch", "feature/new")
	runCmd(t, m, m.startTaskModal(modalAddBranch))
	form(t, m).title.SetValue("Late branch task")
	form(t, m).scope.SetValue("new")
	form(t, m).resetBranchCursor()
	if got := form(t, m).matchingBranches(); len(got) != 1 || got[0] != "feature/new" {
		t.Fatalf("branch search used a stale branch list: %v", got)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if isOpen[*taskModal](m) {
		t.Fatalf("save failed: %s", form(t, m).err)
	}
	m.branch.open = branchGroup("")
	if rows := m.rows(branchPane); len(rows) != 1 || rows[0].name != "feature/new" || rows[0].missingGitBranch {
		t.Fatalf("new branch was marked missing: %+v", rows)
	}
}

func TestEditModalRefreshesBranchState(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/x")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/x\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.branch.open = branchGroup("feature/x")
	projecttest.Git(t, dir, "branch", "-D", "feature/x")
	// The branch was there when Git last answered, so the form opens, and
	// closes when Git answers that it has gone.
	cmd := m.startTaskModal(modalEdit)
	if !isOpen[*taskModal](m) {
		t.Fatalf("edit was blocked before Git answered: status %q", m.status)
	}
	runCmd(t, m, cmd)
	if isOpen[*taskModal](m) || m.status != missingBranchStatus {
		t.Fatalf("edit stayed open for a branch deleted after startup: status %q", m.status)
	}
	projecttest.Git(t, dir, "branch", "feature/x")
	// A blocked edit asks Git again, so the next one opens.
	runCmd(t, m, m.startTaskModal(modalEdit))
	if isOpen[*taskModal](m) || m.branchMissing("feature/x") {
		t.Fatalf("a blocked edit did not ask Git again: status %q", m.status)
	}
	m.startTaskModal(modalEdit)
	if !isOpen[*taskModal](m) {
		t.Fatalf("edit stayed blocked after the branch was restored: status %q", m.status)
	}
}

func TestGitAnswerUpdatesAnOpenBranchForm(t *testing.T) {
	m := &model{project: project.Context{Branch: "main"}, localBranches: []string{"feature/b", "main"}, width: 100, height: 30}
	m.startTaskModal(modalAddBranch)
	f := form(t, m)
	f.scope.SetValue("feature")
	f.resetBranchCursor()
	m.setBranches(branchStateMsg{branches: []string{"feature/a", "feature/b", "main"}, current: "main", verified: true})
	if got := f.chosenBranch(); got != "feature/b" {
		t.Fatalf("a new branch moved the highlight to %q", got)
	}
	m.setBranches(branchStateMsg{branches: []string{"feature/a", "main"}, current: "main", verified: true})
	if got := f.chosenBranch(); got != "" {
		t.Fatalf("the highlighted branch went, and %q was chosen in its place", got)
	}
	m.setBranches(branchStateMsg{branches: []string{"feature/a", "main"}, current: "feature/a", verified: true})
	if f.scope.Value() != "feature" {
		t.Fatalf("a new current branch replaced the branch being typed: %q", f.scope.Value())
	}

	m.overlay = nil
	m.startTaskModal(modalAddBranch)
	f = form(t, m)
	m.setBranches(branchStateMsg{branches: []string{"feature/a", "main"}, current: "feature/a", verified: true})
	if f.scope.Value() != "feature/a" || f.chosenBranch() != "feature/a" {
		t.Fatalf("the form kept the old current branch: %q, chose %q", f.scope.Value(), f.chosenBranch())
	}
}

func TestBranchPickerRejectsDeletedBranch(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/old")
	path := filepath.Join(dir, "todo.md")
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.startTaskModal(modalAddBranch)
	form(t, m).title.SetValue("Do not save")
	form(t, m).scope.SetValue("feature/old")
	projecttest.Git(t, dir, "branch", "-D", "feature/old")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).err != "branch \"feature/old\" no longer exists locally" {
		t.Fatalf("deleted branch was accepted: %+v", m.overlay)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("rejected task changed the TODO file: %v", err)
	}
}

func TestBranchPickerFitsCompactAndRegularModals(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/auth", "feature/ui", "fix/search")
	m, err := newModel(filepath.Join(dir, "todo.md"), repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.startTaskModal(modalAddBranch)
	form(t, m).scope.SetValue("")
	form(t, m).branchCursor = 0
	suggestions := form(t, m).branchSuggestions(24)
	if len(suggestions) != 2 || !strings.Contains(ansi.Strip(suggestions[0]), "feature/auth") || !strings.Contains(ansi.Strip(suggestions[1]), "feature/ui") || strings.Contains(strings.Join(suggestions, ""), "fix/search") || !strings.Contains(strings.Join(suggestions, ""), "┃") {
		t.Fatalf("picker did not show two rows with a scroll indicator: %q", suggestions)
	}
	form(t, m).scope.SetValue("feature/")
	form(t, m).resetBranchCursor()
	if suggestions = form(t, m).branchSuggestions(24); len(suggestions) != 2 || strings.Contains(strings.Join(suggestions, ""), "┃") || strings.Contains(strings.Join(suggestions, ""), "│") {
		t.Fatalf("scroll indicator shown for only two matches: %q", suggestions)
	}
	form(t, m).scope.SetValue("")
	form(t, m).branchCursor = 0
	for _, size := range [][2]int{{80, 24}, {56, 19}, {56, 16}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			form(t, m).resize(size[0], size[1])
			if size[1] == 24 && lipgloss.Height(form(t, m).details.View()) != 5 {
				t.Errorf("regular details box did not gain one line: height %d", lipgloss.Height(form(t, m).details.View()))
			}
			width, height := form(t, m).dimensions(size[0], size[1])
			view := form(t, m).render(width, height)
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
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = opened.(*model)
	if !isOpen[*taskModal](m) || form(t, m).field != 0 {
		t.Fatal("add did not open the title field")
	}
	if help := ansi.Strip(form(t, m).render(76, 20)); !strings.Contains(help, "ctrl+enter save  esc cancel  tab next field") || !strings.Contains(help, "Category") {
		t.Fatalf("add form has wrong labels or help: %s", help)
	} else {
		for line := range strings.SplitSeq(help, "\n") {
			if strings.Trim(line, " │") == "Task" {
				t.Fatal("add form repeats the task label")
			}
		}
	}
	if rendered := form(t, m).render(76, 20); !strings.Contains(rendered, ui.MutedStyle.Render("Category")) {
		t.Fatal("unfocused category label is not muted")
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m.width, m.height = size[0], size[1]
			form(t, m).resize(m.width, m.height)
			modalWidth, modalHeight := form(t, m).dimensions(m.width, m.height)
			if rendered := form(t, m).render(modalWidth, modalHeight); lipgloss.Width(rendered) != modalWidth || lipgloss.Height(rendered) != modalHeight {
				t.Errorf("modal itself overflows at %dx%d: %dx%d", size[0], size[1], lipgloss.Width(rendered), lipgloss.Height(rendered))
			}
			view := m.View().Content
			if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
				t.Errorf("modal size at %dx%d = %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
			}
			if !strings.Contains(ansi.Strip(view), "General › New task") {
				t.Error("modal heading missing")
			}
		})
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if !isOpen[*taskModal](m) || form(t, m).err == "" {
		t.Fatal("empty title should keep the form open with an error")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = opened.(*model)
	if form(t, m).title.Value() != "F" {
		t.Fatalf("title field ignored typing: %q", form(t, m).title.Value())
	}
	form(t, m).title.SetValue("First task")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	if form(t, m).field != 1 {
		t.Fatal("down arrow did not focus category field")
	}
	if help := ansi.Strip(form(t, m).render(76, 20)); !strings.Contains(help, "ctrl+enter save  esc cancel  tab next field") {
		t.Fatalf("category help changed by focus: %s", help)
	}
	if rendered := form(t, m).render(76, 20); !strings.Contains(rendered, ui.TitleStyle.Render("Category")) {
		t.Fatal("focused category label is not highlighted")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(*model)
	if form(t, m).field != 0 {
		t.Fatal("up arrow on the first details line did not focus title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	if form(t, m).field != 1 {
		t.Fatal("down arrow did not return to category field")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	if form(t, m).field != 2 {
		t.Fatal("down arrow did not focus details")
	}
	if help := ansi.Strip(form(t, m).render(76, 20)); !strings.Contains(help, "ctrl+enter save  esc cancel  tab next field") {
		t.Fatalf("details help changed by focus: %s", help)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	m = opened.(*model)
	if form(t, m).details.Value() != "X" {
		t.Fatalf("details field ignored typing: %q", form(t, m).details.Value())
	}
	form(t, m).details.SetValue("Added in modal.\n- [ ] Nested step")
	if form(t, m).details.Line() != 1 {
		t.Fatalf("details cursor should start on the final line, got %d", form(t, m).details.Line())
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(*model)
	if form(t, m).field != 2 || form(t, m).details.Line() != 0 {
		t.Fatal("up arrow should move within multiline details before returning to title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(*model)
	if form(t, m).field != 1 {
		t.Fatal("up arrow on first details line did not focus category field")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if isOpen[*taskModal](m) || len(m.tasks.general) != 1 || m.tasks.general[0].Details != "Added in modal.\n- [ ] Nested step" {
		t.Fatalf("task was not saved: %+v", m.tasks.general)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = opened.(*model)
	if !isOpen[*taskModal](m) || form(t, m).title.Value() != "First task" {
		t.Fatal("edit did not reopen the task in the modal")
	}
	form(t, m).title.SetValue("Renamed task")
	form(t, m).details.SetValue("Revised details.")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if len(m.tasks.general) != 1 || m.tasks.general[0].Text != "Renamed task" || m.tasks.general[0].Details != "Revised details." {
		t.Fatalf("edit was not saved: %+v", m.tasks.general)
	}
}

func TestTwoLineTaskInputStaysOneMarkdownTask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 100, 30
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = opened.(*model)
	if got := lipgloss.Height(form(t, m).title.View()); got != 2 {
		t.Fatalf("task input is %d lines, want 2", got)
	}
	form(t, m).title.SetValue("Fix login\nredirect")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	selected, ok := m.selectedTask()
	if !ok || selected.Text != "Fix login redirect" {
		t.Fatalf("two-line Task was not saved as one title: %+v", m.rows(generalPane))
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
				m := &model{width: size[0], height: size[1], tasks: taskSet{general: []store.Task{{Text: "Task"}}}}
				m.startTaskModal(mode)
				width, height := form(t, m).dimensions(size[0], size[1])
				lines := strings.Split(ansi.Strip(form(t, m).render(width, height)), "\n")
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

func TestOnBackgroundSurvivesNestedResets(t *testing.T) {
	seq := ansi.NewStyle().BackgroundColor(ui.ColorModal).String()
	got := ui.OnBackground(ui.KeyStyle.Render("esc")+" "+ui.MutedStyle.Render("cancel"), ui.ColorModal)
	if !strings.HasPrefix(got, seq) || strings.Count(got, "\x1b[m"+seq) != 2 {
		t.Fatalf("background not restored after resets: %q", got)
	}
	if got := ui.OnBackground("a\x1b[49mb", ui.ColorModal); got != seq+"a"+seq+"b" {
		t.Fatalf("default background not replaced: %q", got)
	}
}

func TestTaskModalFieldsAndFocus(t *testing.T) {
	m, err := newModel(filepath.Join(t.TempDir(), "todo.md"), project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 24
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = opened.(*model)
	focusedLine := func() string {
		for line := range strings.SplitSeq(ansi.Strip(form(t, m).render(76, 20)), "\n") {
			if strings.Contains(line, "┃") {
				return line
			}
		}
		return ""
	}
	if line := focusedLine(); !strings.Contains(line, "What needs doing?") {
		t.Fatalf("focus bar is not beside the task field: %q", line)
	}
	for line := range strings.SplitSeq(form(t, m).render(76, 20), "\n") {
		if strings.Contains(line, "\x1b[m ") {
			t.Fatalf("modal line lets the terminal background through after a reset: %q", line)
		}
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = opened.(*model)
	if line := focusedLine(); !strings.Contains(line, "Category") {
		t.Fatalf("focus bar did not follow the category field: %q", line)
	}
	if got, want := form(t, m).scope.Width(), 76-6; got != want {
		t.Fatalf("category field width = %d, want the full %d", got, want)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	plain := ansi.Strip(form(t, m).render(76, 20))
	lines := strings.Split(plain, "\n")
	if form(t, m).err == "" || !strings.Contains(lines[1], "General › New task") || !strings.Contains(lines[len(lines)-2], form(t, m).err) || strings.Contains(plain, "ctrl+enter") {
		t.Fatalf("error should replace the key hints and keep the heading: %s", plain)
	}
}

func TestBranchFieldHintsDescribePicker(t *testing.T) {
	m, err := newModel(filepath.Join(t.TempDir(), "todo.md"), project.Context{Branch: "main"}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 24
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = opened.(*model)
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = opened.(*model)
	if plain := ansi.Strip(form(t, m).render(76, 20)); !strings.Contains(plain, "Branches › New task") || !strings.Contains(plain, "↑↓ choose  tab accept") {
		t.Fatalf("branch form heading or picker hints missing: %s", plain)
	}
}

func TestEditFormMovesTaskToAnotherCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# auth\n\n- [ ] Login task\n\n# docs\n\n- [ ] Write guide\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 24
	m.general.open = categoryGroup("auth")
	m.startTaskModal(modalEdit)
	if form(t, m).branchScope() || form(t, m).scope.Value() != "auth" || !strings.Contains(ansi.Strip(form(t, m).render(76, 20)), "Category") {
		t.Fatalf("edit form did not offer the task's category: %q", form(t, m).scope.Value())
	}
	form(t, m).scope.SetValue("@docs")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if isOpen[*taskModal](m) {
		t.Fatalf("save failed: %s", form(t, m).err)
	}
	if selected, ok := m.selectedTask(); m.general.open.name != "docs" || !ok || selected.Text != "Login task" || selected.Category != "docs" {
		t.Fatalf("view did not follow the moved task: category %q, selected %+v", m.general.open.name, selected)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "# auth") {
		t.Fatalf("moving the last task left its heading behind: %s", data)
	}
	m.startTaskModal(modalEdit)
	form(t, m).scope.SetValue("")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); isOpen[*taskModal](m) || m.general.open.name != "" || !ok || selected.Text != "Login task" || selected.Category != "" {
		t.Fatalf("clearing the category did not move the task to General: %+v", selected)
	}
}

func TestEditFormMovesTaskToAnotherBranch(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/a", "feature/b")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/a\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 24
	m.focus, m.branch.open = branchPane, branchGroup("feature/a")
	m.startTaskModal(modalEdit)
	if !form(t, m).branchScope() || form(t, m).scope.Value() != "feature/a" || len(form(t, m).branches) != 3 {
		t.Fatalf("edit form did not offer the branch picker: %q %v", form(t, m).scope.Value(), form(t, m).branches)
	}
	form(t, m).focusField(scopeField)
	for _, r := range "b" {
		updated, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(*model)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if isOpen[*taskModal](m) {
		t.Fatalf("save failed: %s", form(t, m).err)
	}
	if selected, ok := m.selectedTask(); m.branch.open.name != "feature/b" || !ok || selected.Branch != "feature/b" {
		t.Fatalf("view did not follow the task to its new branch: %q %+v", m.branch.open.name, selected)
	}
}

func TestEditFormKeepsBranchWithoutGit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/a\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus, m.branch.open = branchPane, branchGroup("feature/a")
	m.startTaskModal(modalEdit)
	form(t, m).title.SetValue("Renamed branch task")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if isOpen[*taskModal](m) {
		t.Fatalf("editing a branch task without Git failed: %s", form(t, m).err)
	}
	if selected, ok := m.selectedTask(); !ok || selected.Text != "Renamed branch task" || selected.Branch != "feature/a" {
		t.Fatalf("edit changed the task's branch: %+v", selected)
	}
}
