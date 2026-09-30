package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestBranchListMarksMissingGitBranches(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/live", "feature/gone")
	path := filepath.Join(dir, "todo.md")
	content := "# Branches\n\n## feature/gone\n\n- [ ] Keep this task\n\n## feature/live\n\n- [ ] Active task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	if rows := m.branchRows(); len(rows) != 2 || rows[0].missingGitBranch || rows[1].missingGitBranch {
		t.Fatalf("existing branches were marked missing: %+v", rows)
	}
	if _, err := gitOutput(dir, "branch", "-D", "feature/gone"); err != nil {
		t.Fatal(err)
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	rows := m.branchRows()
	if len(rows) != 2 || !rows[0].missingGitBranch || rows[1].missingGitBranch || rows[0].count != 1 {
		t.Fatalf("deleted branch was not identified while keeping its tasks: %+v", rows)
	}
	view := ansi.Strip(m.renderNavigationPane(m.branchTitle(), rows, 0, branchPane, 60, 20))
	if !strings.Contains(view, "⚠ feature/gone  0/1 › missing") || strings.Contains(view, "⚠ feature/live") || !strings.Contains(view, "feature/live  0/1 ›") {
		t.Fatalf("branch list warning is unclear: %s", view)
	}
	long := navigationRow{kind: rowBranch, name: "feature/a-very-long-branch-name-that-needs-truncating", count: 4, completed: 1, missingGitBranch: true}
	if got := ansi.Strip(renderMissingBranchRow(long, 28, true)); !strings.HasPrefix(got, "⚠ feature/") || !strings.Contains(got, "1/4 › missing") || ansi.StringWidth(got) != 28 {
		t.Fatalf("long branch hid its missing marker: %q", got)
	}
	if _, err := gitOutput(dir, "branch", "feature/gone"); err != nil {
		t.Fatal(err)
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	if m.branchRows()[0].missingGitBranch {
		t.Fatal("restored Git branch still marked missing")
	}
}

func TestMissingBranchTasksAreReadOnly(t *testing.T) {
	dir := t.TempDir()
	project := testGitProject(t, dir, "main", "feature/gone")
	path := filepath.Join(dir, "todo.md")
	content := "# Branches\n\n## feature/gone\n\n- [ ] Keep this task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(dir, "branch", "-D", "feature/gone"); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project)
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	if view := ansi.Strip(m.View().Content); strings.Contains(view, "Branch no longer exists") {
		t.Fatalf("status bar shown before opening the branch: %s", view)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(lines) != m.height || !strings.Contains(lines[len(lines)-3], "⚠ Branch no longer exists · tasks cannot be edited") {
		t.Fatalf("status bar missing from bottom of panel: %q", lines)
	}
	for _, key := range []tea.KeyPressMsg{{Code: 'd', Text: "d"}, {Code: 'p', Text: "p"}, {Code: 'e', Text: "e"}, {Code: tea.KeyEnter}} {
		updated, _ = m.Update(key)
		m = updated.(*model)
		if m.modal != nil || m.status != missingBranchStatus {
			t.Fatalf("%q was not blocked: modal=%v status=%q", key.String(), m.modal != nil, m.status)
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Branch no longer exists") {
		t.Fatalf("status bar missing from task details: %s", view)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("task on missing branch was modified:\n%s", data)
	}
}

func TestBranchesOutsideGitAreNotMarkedMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("# Branches\n\n## feature/x\n\n- [ ] Branch task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	if rows := m.branchRows(); len(rows) != 1 || rows[0].missingGitBranch {
		t.Fatalf("branch without a Git repository was marked missing: %+v", rows)
	}
}

func TestCategoriesAndBranchesDrillDown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Uncategorised\n\n# auth\n\n- [ ] Login task\n\n# tests\n\n- [ ] Another test\n\n# Branches\n\n## feature/login\n\n- [ ] Branch login\n\n## fix/api\n\n- [ ] Branch API\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{branch: "fix/api"})
	if err != nil {
		t.Fatal(err)
	}
	general := m.generalRows()
	if len(general) != 3 || general[0].kind != rowCategory || general[0].name != "auth" || general[0].count != 1 || general[1].kind != rowCategory || general[1].name != "tests" || general[2].todo.text != "Uncategorised" {
		t.Fatalf("general rows = %+v", general)
	}
	list := ansi.Strip(m.renderNavigationPane(m.generalTitle(), general, 0, generalPane, 50, 18))
	if strings.Contains(list, "[ ]") || !strings.Contains(list, "@auth") || strings.Contains(list, "Login task") || strings.Contains(list, "Branch login") {
		t.Fatalf("root General list showed categorised or branch tasks: %s", list)
	}
	if _, ok := m.selectedTask(); ok {
		t.Fatal("category row selected a task")
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.generalCategory != "auth" || m.generalTitle() != "General · @auth" || len(m.generalRows()) != 1 || m.generalRows()[0].todo.text != "Login task" {
		t.Fatalf("category did not filter general tasks: %+v", m.generalRows())
	}
	if opened := ansi.Strip(m.renderNavigationPane(m.generalTitle(), m.generalRows(), 0, generalPane, 50, 12)); strings.Contains(opened, "  Login task") || !strings.Contains(opened, "Login task") {
		t.Fatalf("category contents were indented: %s", opened)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if m.focus != detailPane {
		t.Fatal("right arrow on a filtered task did not focus details")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.focus != generalPane || m.generalCategory != "auth" {
		t.Fatal("left arrow from details skipped the filtered task list")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.generalCategory != "" || m.generalCursor != 0 {
		t.Fatal("left arrow did not return to General categories")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(*model)
	branches := m.branchRows()
	if len(branches) != 2 || branches[m.branchCursor].name != "fix/api" {
		t.Fatalf("current branch was not preselected: cursor %d rows %+v", m.branchCursor, branches)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if m.branchFilter != "fix/api" || len(m.branchRows()) != 1 || m.branchRows()[0].todo.text != "Branch API" {
		t.Fatalf("branch did not open: %+v", m.branchRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.branchFilter != "" || m.branchCursor != 1 {
		t.Fatal("left arrow did not return to selected branch")
	}
	m.branchFilter = "feature/login"
	m.branchCursor = 0
	branchList := ansi.Strip(m.renderNavigationPane(m.branchTitle(), m.branchRows(), 0, branchPane, 50, 12))
	if strings.Contains(branchList, "[ ]") || strings.Contains(branchList, "@auth") || !strings.Contains(branchList, "Branch login") {
		t.Fatalf("branch tasks were not shown directly: %s", branchList)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.branchFilter != "" || m.branchCursor != 1 {
		t.Fatal("left arrow did not return to branches")
	}
}

func TestArrowOpensDetailsAndEscapeReturnsToList(t *testing.T) {
	m := &model{general: []task{{text: "First task"}}, width: 100, height: 30}
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
	updated, _ = m.Update(sourceScanMsg{matches: nil})
	m = updated.(*model)
	footer := ansi.Strip(m.renderFooter(100))
	if !strings.Contains(footer, "(e)dit/enter file") || strings.Contains(footer, "Found 0") {
		t.Fatalf("scan changed the file TODO footer: %q", footer)
	}
	m.status = "No current Git branch"
	footer = ansi.Strip(m.renderFooter(60))
	if !strings.Contains(footer, "No current") {
		t.Fatalf("small footer lost error: %q", footer)
	}
}

func TestCompletedTasksFollowOpenTasksInEachScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [x] General done\n  - Priority: High\n\n- [ ] General open\n  - Priority: Low\n\n# auth\n\n- [x] Auth done\n  - Priority: High\n\n- [ ] Auth open\n  - Priority: Low\n\n# Branches\n\n## feature/x\n\n- [x] Branch done\n  - Priority: High\n\n- [ ] Branch open first\n  - Priority: Low\n\n- [ ] Branch open second\n  - Priority: Low\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
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
			m.indexSort = item.sort
			if got := indexTitles(m.indexTasks()); got != item.want {
				t.Errorf("%s order = %q, want %q", item.sort, got, item.want)
			}
		})
	}
	if rows := m.generalRows(); rows[1].todo.text != "General open" || rows[2].todo.text != "General done" {
		t.Fatalf("general tasks are out of order: %+v", rows)
	}
	m.generalCategory = "auth"
	if rows := m.generalRows(); rows[0].todo.text != "Auth open" || rows[1].todo.text != "Auth done" {
		t.Fatalf("category tasks are out of order: %+v", rows)
	}
	m.focus = branchPane
	m.branchFilter = "feature/x"
	if rows := m.branchRows(); rows[0].todo.text != "Branch open first" || rows[1].todo.text != "Branch open second" || rows[2].todo.text != "Branch done" {
		t.Fatalf("branch tasks are out of order: %+v", rows)
	}
	m.branchCursor = 0
	m.toggleSelected()
	if selected, ok := m.selectedTask(); !ok || selected.text != "Branch open first" || !selected.done || m.branchCursor != 2 {
		t.Fatalf("completed branch task did not stay selected at the end: %+v", m.branchRows())
	}
}
