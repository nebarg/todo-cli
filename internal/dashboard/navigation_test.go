package dashboard

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/project/projecttest"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

func TestBranchListMarksMissingGitBranches(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/live", "feature/gone")
	path := filepath.Join(dir, "todo.md")
	content := "# Branches\n\n## feature/gone\n\n- [ ] Keep this task\n\n## feature/live\n\n- [ ] Active task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	if rows := m.rows(branchPane); len(rows) != 2 || rows[0].missingGitBranch || rows[1].missingGitBranch {
		t.Fatalf("existing branches were marked missing: %+v", rows)
	}
	projecttest.Git(t, dir, "branch", "-D", "feature/gone")
	runCmd(t, m, m.checkBranches())
	rows := m.rows(branchPane)
	if len(rows) != 2 || !rows[0].missingGitBranch || rows[1].missingGitBranch || rows[0].count != 1 {
		t.Fatalf("deleted branch was not identified while keeping its tasks: %+v", rows)
	}
	view := ansi.Strip(m.renderNavigationPane(rows, 0, branchPane, 60, 20))
	if !regexp.MustCompile(`⚠ feature/gone not in Git +0/1`).MatchString(view) || strings.Contains(view, "⚠ feature/live") || !regexp.MustCompile(`▸ feature/live +0/1`).MatchString(view) {
		t.Fatalf("branch list warning is unclear: %s", view)
	}
	long := navigationRow{kind: rowBranch, name: "feature/a-very-long-branch-name-that-needs-truncating", count: 4, completed: 1, missingGitBranch: true}
	if got := ansi.Strip(renderGroupRow(m.theme, long, 28, true, false)); !strings.HasPrefix(got, "⚠ feature/") || !strings.HasSuffix(got, "… not in Git  1/4") || ansi.StringWidth(got) != 28 {
		t.Fatalf("long branch hid its missing marker: %q", got)
	}
	projecttest.Git(t, dir, "branch", "feature/gone")
	runCmd(t, m, m.checkBranches())
	if m.rows(branchPane)[0].missingGitBranch {
		t.Fatal("restored Git branch still marked missing")
	}
}

func TestMissingBranchTasksCanBeEdited(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/planned\n\n- [ ] Keep this task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	if view := ansi.Strip(m.View().Content); strings.Contains(view, missingBranchStatus) {
		t.Fatalf("status bar warning shown before opening the branch: %s", view)
	}
	m = press(m, "enter")
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(lines) != m.height || !strings.Contains(lines[len(lines)-3], "Open  ·  No priority  ·  "+missingBranchStatus) {
		t.Fatalf("status bar does not warn about the branch beside the task's status: %q", lines)
	}
	if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); !strings.Contains(hints, "d done  e edit  p priority") {
		t.Fatalf("footer lacks the task keys: %q", hints)
	}
	m = press(press(m, "d"), "p")
	if m.status != "" {
		t.Fatalf("editing a task was refused: %q", m.status)
	}
	if got, want := readFile(t, path), "# Branches\n\n## feature/planned\n\n- [x] Keep this task !high\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
	m = press(m, "e")
	if !isOpen[*taskModal](m) {
		t.Fatalf("edit form did not open: status %q", m.status)
	}
	m = press(press(m, "esc"), "right")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, missingBranchStatus) || !strings.Contains(ansi.Strip(m.theme.RenderHints(m.footerHints())), "e edit") {
		t.Fatalf("task details lost the warning or the task keys: %s", view)
	}
}

func TestBranchesOutsideGitAreNotMarkedMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/x\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.rows(branchPane); len(rows) != 1 || rows[0].missingGitBranch {
		t.Fatalf("branch without a Git repository was marked missing: %+v", rows)
	}
}

func TestCategoriesAndBranchesDrillDown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Uncategorised\n\n# auth\n\n- [ ] Login task\n\n# tests\n\n- [ ] Another test\n\n# Branches\n\n## feature/login\n\n- [ ] Branch login\n\n## fix/api\n\n- [ ] Branch API\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{Branch: "fix/api"}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	general := m.rows(generalPane)
	if len(general) != 3 || general[0].kind != rowCategory || general[0].name != "auth" || general[0].count != 1 || general[1].kind != rowCategory || general[1].name != "tests" || general[2].todo.Text != "Uncategorised" {
		t.Fatalf("general rows = %+v", general)
	}
	list := ansi.Strip(m.renderNavigationPane(general, 0, generalPane, 50, 18))
	if strings.Contains(list, "[ ]") || !strings.Contains(list, "▸ auth") || strings.Contains(list, "Login task") || strings.Contains(list, "Branch login") {
		t.Fatalf("root General list showed categorised or branch tasks: %s", list)
	}
	if _, ok := m.selectedTask(); ok {
		t.Fatal("category row selected a task")
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.general.open.name != "auth" || !slices.Equal(m.breadcrumb(generalPane), []string{"General", "@auth"}) || len(m.rows(generalPane)) != 1 || m.rows(generalPane)[0].todo.Text != "Login task" {
		t.Fatalf("category did not filter general tasks: %+v", m.rows(generalPane))
	}
	if opened := ansi.Strip(m.renderNavigationPane(m.rows(generalPane), 0, generalPane, 50, 12)); strings.Contains(opened, "  Login task") || !strings.Contains(opened, "Login task") {
		t.Fatalf("category contents were indented: %s", opened)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if m.focus != detailPane {
		t.Fatal("right arrow on a filtered task did not focus details")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.focus != generalPane || m.general.open.name != "auth" {
		t.Fatal("left arrow from details skipped the filtered task list")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.general.open.name != "" || m.general.cursor != 0 {
		t.Fatal("left arrow did not return to General categories")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(*model)
	if m.branch.open.name != "fix/api" || len(m.rows(branchPane)) != 1 || m.rows(branchPane)[0].todo.Text != "Branch API" {
		t.Fatalf("current branch did not open on startup: %+v", m.rows(branchPane))
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	branches := m.rows(branchPane)
	if m.branch.open.name != "" || len(branches) != 2 || branches[m.branch.cursor].name != "fix/api" {
		t.Fatalf("left arrow did not return to the current branch row: cursor %d rows %+v", m.branch.cursor, branches)
	}
	m.branch.open = branchGroup("feature/login")
	m.branch.cursor = 0
	branchList := ansi.Strip(m.renderNavigationPane(m.rows(branchPane), 0, branchPane, 50, 12))
	if strings.Contains(branchList, "[ ]") || strings.Contains(branchList, "@auth") || !strings.Contains(branchList, "Branch login") {
		t.Fatalf("branch tasks were not shown directly: %s", branchList)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.branch.open.name != "" || m.branch.cursor != 1 {
		t.Fatal("left arrow did not return to branches")
	}
}

func TestArrowOpensDetailsAndEscapeReturnsToList(t *testing.T) {
	m := &model{tasks: taskSet{general: []store.Task{{Text: "First task"}}}, theme: ui.NewTheme(true), width: 100, height: 30}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if m.focus != detailPane || m.detailFrom != generalPane {
		t.Fatalf("right arrow page = %v from %v", m.focus, m.detailFrom)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	if m.focus != generalPane {
		t.Fatal("Escape did not return to task list")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	m = updated.(*model)
	if m.focus != sourcePane {
		t.Fatal("3 did not switch to file TODOs")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if m.focus != sourcePane {
		t.Fatal("right arrow opened details without a selected file TODO")
	}
	if m.files.Details() {
		t.Fatal("right arrow opened file details without a selected file TODO")
	}
	updated, _ = m.Update(filesui.ScannedMsg{})
	m = updated.(*model)
	footer := ansi.Strip(m.renderFooter(100))
	if strings.Contains(footer, "e open file") || strings.Contains(footer, "Found 0") {
		t.Fatalf("empty file TODO footer = %q", footer)
	}
	m.status = "No current Git branch"
	footer = ansi.Strip(m.renderFooter(60))
	if !strings.Contains(footer, "No current") {
		t.Fatalf("small footer lost error: %q", footer)
	}
}

func TestCompletedTasksFollowOpenTasksInEachScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [x] General done !high\n\n- [ ] General open !low\n\n# auth\n\n- [x] Auth done !high\n\n- [ ] Auth open !low\n\n# Branches\n\n## feature/x\n\n- [x] Branch done !high\n\n- [ ] Branch open first !low\n\n- [ ] Branch open second !low\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		sort sortOrder
		want string
	}{
		{"priority", "General open,Auth open,Branch open first,Branch open second,General done,Auth done,Branch done"},
		{"branch", "Branch open first,Branch open second,Branch done,Auth open,Auth done,General open,General done"},
		{"category", "Auth open,Auth done,Branch open first,Branch open second,Branch done,General open,General done"},
	} {
		t.Run(string(item.sort), func(t *testing.T) {
			view := allTasksView{sort: item.sort}
			if got := indexTitles(view.sorted(m.tasks.all)); got != item.want {
				t.Errorf("%s order = %q, want %q", item.sort, got, item.want)
			}
		})
	}
	if rows := m.rows(generalPane); rows[1].todo.Text != "General open" || rows[2].todo.Text != "General done" {
		t.Fatalf("general tasks are out of order: %+v", rows)
	}
	m.general.open = categoryGroup("auth")
	if rows := m.rows(generalPane); rows[0].todo.Text != "Auth open" || rows[1].todo.Text != "Auth done" {
		t.Fatalf("category tasks are out of order: %+v", rows)
	}
	m.focus = branchPane
	m.branch.open = branchGroup("feature/x")
	if rows := m.rows(branchPane); rows[0].todo.Text != "Branch open first" || rows[1].todo.Text != "Branch open second" || rows[2].todo.Text != "Branch done" {
		t.Fatalf("branch tasks are out of order: %+v", rows)
	}
	m.branch.cursor = 0
	m.toggleSelected()
	if selected, ok := m.selectedTask(); !ok || selected.Text != "Branch open first" || !selected.Done || m.branch.cursor != 2 {
		t.Fatalf("completed branch task did not stay selected at the end: %+v", m.rows(branchPane))
	}
}

func TestReloadRechecksBranches(t *testing.T) {
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/x")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/x\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir) // reload reads the Git context from the working directory.
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	projecttest.Git(t, dir, "branch", "-D", "feature/x")
	if cmd := pressKey(t, m, "enter"); cmd != nil {
		t.Fatal("opening a branch asked Git again")
	}
	if strings.Contains(ansi.Strip(m.panelStatus()), missingBranchStatus) {
		t.Fatal("the branch was shown as not in Git before Git was asked")
	}
	m = pressAndRun(t, m, "r")
	if !strings.Contains(ansi.Strip(m.panelStatus()), missingBranchStatus) {
		t.Fatal("r did not re-check the branch")
	}
	projecttest.Git(t, dir, "branch", "feature/x")
	m = pressAndRun(t, m, "r")
	if strings.Contains(ansi.Strip(m.panelStatus()), missingBranchStatus) {
		t.Fatal("restored branch is still shown as not in Git")
	}
}

func TestPressingATabAgainReturnsToItsTopLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# auth\n\n- [ ] Login task\n\n# Branches\n\n## feature/login\n\n- [ ] Branch login\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m = press(m, "enter")
	m = press(m, "2")
	m = press(m, "enter")
	m = press(m, "1")
	if m.focus != generalPane || m.general.open.name != "auth" {
		t.Fatalf("switching to General left @auth: focus %v, category %q", m.focus, m.general.open.name)
	}
	m = press(m, "1")
	if m.general.open.name != "" || m.branch.open.name != "feature/login" {
		t.Fatalf("1 again: category %q, branch %q", m.general.open.name, m.branch.open.name)
	}
	m = press(m, "1")
	if m.focus != generalPane || m.general.open.name != "" {
		t.Fatal("1 at the top of General changed the view")
	}
	m = press(m, "2")
	if m.focus != branchPane || m.branch.open.name != "feature/login" {
		t.Fatalf("switching to Branches left the branch: %q", m.branch.open.name)
	}
	m = press(m, "2")
	if m.branch.open.name != "" {
		t.Fatal("2 again did not return to the branch list")
	}
}

func TestTwoTogglesBetweenBranchListAndCurrentBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] General task\n\n# Branches\n\n## feature/other\n\n- [ ] Other task\n\n## main\n\n- [ ] Main task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{Branch: "main"}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if m.focus != generalPane || m.branch.open.name != "main" {
		t.Fatalf("startup: focus %v, branch %q", m.focus, m.branch.open.name)
	}
	steps := []struct{ key, branch string }{
		{"2", "main"},
		{"2", ""},
		{"1", ""},
		{"2", ""},
		{"2", "main"},
		{"esc", ""},
		{"k", ""},
		{"enter", "feature/other"},
		{"1", "feature/other"},
		{"2", "feature/other"},
		{"2", ""},
	}
	for i, step := range steps {
		m = press(m, step.key)
		if m.branch.open.name != step.branch {
			t.Fatalf("step %d (%s): branch %q, want %q", i, step.key, m.branch.open.name, step.branch)
		}
	}
	if rows := m.rows(branchPane); rows[m.branch.cursor].name != "feature/other" {
		t.Fatal("leaving a branch lost its row")
	}

	m, err = newModel(path, project.Context{Branch: "fix/none"}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	if m = press(press(m, "2"), "2"); m.branch.open.name != "" || m.focus != branchPane {
		t.Fatalf("branch without tasks opened %q", m.branch.open.name)
	}
}

func TestFilesTabKeysReachTheBrowser(t *testing.T) {
	m := &model{theme: ui.NewTheme(true), width: 100, height: 20}
	m.files.Update(filesui.ScannedMsg{Matches: []scan.Match{
		{Path: "a.go", Line: 1, Note: "urgent", Level: "0"},
		{Path: "b.css", Line: 2, Note: "hide", Category: "Boundary"},
	}})
	files := func() string { return ansi.Strip(m.View().Content) }
	m = press(m, "3")
	m = press(m, "enter")
	if !strings.Contains(files(), "Files › @Boundary") {
		t.Fatalf("enter did not open the category:\n%s", files())
	}
	m = press(m, "right")
	if !m.files.Details() || !strings.Contains(files(), "Files › @Boundary › Details") {
		t.Fatalf("right did not open the file TODO's details:\n%s", files())
	}
	if footer := ansi.Strip(m.renderFooter(100)); !strings.Contains(footer, "e open file") || strings.Contains(footer, "all tasks") {
		t.Fatalf("detail footer = %q", footer)
	}
	m = press(m, "3")
	if m.files.Details() || !strings.Contains(files(), "Files › @Boundary") {
		t.Fatalf("3 did not return from details to the category:\n%s", files())
	}
	m = press(m, "3")
	if strings.Contains(files(), "@Boundary") {
		t.Fatalf("3 again did not leave the category:\n%s", files())
	}
	m = press(m, "j")
	m = press(m, "right")
	m = press(m, "tab")
	if m.focus != generalPane || m.files.Details() {
		t.Fatal("leaving the Files tab kept its details open")
	}
	m = press(m, "3")
	if !strings.Contains(files(), "urgent") || strings.Contains(files(), "Details") {
		t.Fatalf("returning to Files did not show its list:\n%s", files())
	}
}

func TestReloadFollowsASwitchedGitBranch(t *testing.T) {
	setup := func(t *testing.T) (*model, string) {
		dir := t.TempDir()
		repo := projecttest.Repo(t, dir, "main", "feature/x", "feature/empty")
		path := filepath.Join(dir, "todo.md")
		if err := os.WriteFile(path, []byte("# Branches\n\n## main\n\n- [ ] Main task\n\n## feature/x\n\n- [ ] Feature task\n"), 0644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir) // reload reads the Git context from the working directory.
		m, err := newModel(path, repo, testFiles())
		if err != nil {
			t.Fatal(err)
		}
		m.focus = branchPane
		return m, dir
	}
	checkout := func(t *testing.T, dir, branch string) {
		t.Helper()
		projecttest.Git(t, dir, "checkout", "-q", branch)
	}

	t.Run("follows from the old current branch", func(t *testing.T) {
		m, dir := setup(t)
		m = press(m, "right") // details of the old branch's task
		checkout(t, dir, "feature/x")
		m = pressAndRun(t, m, "r")
		if m.project.Branch != "feature/x" || m.branch.open.name != "feature/x" || m.focus != branchPane {
			t.Fatalf("after switching: branch %q, open %q, focus %v", m.project.Branch, m.branch.open.name, m.focus)
		}
	})
	t.Run("keeps a branch opened by hand", func(t *testing.T) {
		m, dir := setup(t)
		m = press(m, "2") // back to the branch list
		m.branch.cursor = 0
		m = press(m, "right")
		if m.branch.open.name != "feature/x" {
			t.Fatalf("opened %q", m.branch.open.name)
		}
		checkout(t, dir, "feature/empty")
		m = pressAndRun(t, m, "r")
		if m.branch.open.name != "feature/x" {
			t.Fatalf("a branch opened by hand was left: %q", m.branch.open.name)
		}
	})
	t.Run("returns to the list when the new branch has no tasks", func(t *testing.T) {
		m, dir := setup(t)
		checkout(t, dir, "feature/empty")
		m = pressAndRun(t, m, "r")
		if m.project.Branch != "feature/empty" || m.branch.open.name != "" {
			t.Fatalf("after switching to a branch without tasks: open %q", m.branch.open.name)
		}
	})
}
