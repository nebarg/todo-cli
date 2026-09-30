package main

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func testGitProject(t *testing.T, dir, current string, others ...string) projectContext {
	t.Helper()
	if _, err := gitOutput(dir, "init", "-q"); err != nil {
		t.Skipf("Git is unavailable: %v", err)
	}
	if _, err := gitOutput(dir, "symbolic-ref", "HEAD", "refs/heads/"+current); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".test-anchor"), []byte("fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(dir, "add", ".test-anchor"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "initial"); err != nil {
		t.Fatal(err)
	}
	for _, branch := range others {
		if _, err := gitOutput(dir, "branch", branch); err != nil {
			t.Fatal(err)
		}
	}
	return projectContext{root: dir, branch: current}
}

func TestDashboardFitsTerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Fix failing tests\n\n# Branches\n\n## feature/login\n\n- [ ] Add login check\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{root: filepath.Dir(path), branch: "feature/login"})
	if err != nil {
		t.Fatal(err)
	}
	m.source = []sourceTodo{{path: "main.go", line: 12, text: "// TODO: improve"}}
	m.sourceLoading = false
	for _, size := range [][2]int{{120, 35}, {80, 24}, {78, 16}, {60, 20}, {56, 19}} {
		m.width, m.height = size[0], size[1]
		for _, page := range []struct {
			name  string
			focus pane
			want  string
			hide  string
		}{
			{"general", generalPane, "Fix failing tests", "main.go:12"},
			{"branches", branchPane, "feature/login", "Fix failing tests"},
			{"files", sourcePane, "main.go:12", "Fix failing tests"},
			{"details", detailPane, "Status", "main.go:12"},
		} {
			m.focus = page.focus
			m.detailFrom = generalPane
			view := m.View()
			if got := lipgloss.Width(view.Content); got != size[0] {
				t.Errorf("%s width at %dx%d = %d", page.name, size[0], size[1], got)
			}
			if got := lipgloss.Height(view.Content); got != size[1] {
				t.Errorf("%s height at %dx%d = %d", page.name, size[0], size[1], got)
			}
			plain := ansi.Strip(view.Content)
			lines := strings.Split(plain, "\n")
			if len(lines) < 5 || lines[2] == "" {
				t.Errorf("%s has an unexpected gap below tabs at %dx%d", page.name, size[0], size[1])
			}
			if len(lines) < 6 || strings.Trim(lines[4], " │") != "" {
				t.Errorf("%s has no gap below its heading at %dx%d", page.name, size[0], size[1])
			}
			if !strings.Contains(plain, page.want) || strings.Contains(plain, page.hide) {
				t.Errorf("%s content at %dx%d: %s", page.name, size[0], size[1], plain)
			}
			if !strings.Contains(plain, "1 General") || !strings.Contains(plain, "3 Files") {
				t.Errorf("%s tabs missing at %dx%d", page.name, size[0], size[1])
			}
		}
	}
}

func TestInactiveTabsShareTheBarBackground(t *testing.T) {
	m := &model{focus: sourcePane}
	tabs := m.renderTabs(80)
	inactive := lipgloss.NewStyle().Foreground(colorMuted).Background(lipgloss.Color("#17253A"))
	for _, label := range []string{" 1 General ", " 2 Branches "} {
		if !strings.Contains(tabs, inactive.Render(label)) {
			t.Errorf("inactive tab %q has the wrong background: %q", label, tabs)
		}
	}
	if ansi.StringWidth(tabs) != 80 {
		t.Errorf("tab bar width = %d, want 80", ansi.StringWidth(tabs))
	}
}

func TestScanErrorVisibleInDetails(t *testing.T) {
	m := &model{focus: sourcePane, sourceError: "permission denied"}
	got := strings.Join(m.sourceDetails(60), "\n")
	if !strings.Contains(got, "permission denied") {
		t.Fatalf("scan error missing from detail pane: %s", got)
	}
}

func TestTaskDetailsShownOnDetailPage(t *testing.T) {
	if taskTitleStyle.GetForeground() != lipgloss.Color("#FFFFFF") {
		t.Fatal("task title is not styled with the white text color")
	}
	m := &model{general: []task{{text: "Fix login redirect", details: "When a session expires, return to the previous page.\n\n- Add a regression test"}}}
	got := strings.Join(m.taskDetails(60), "\n")
	for _, want := range []string{"Fix login redirect", "When a session expires", "- Add a regression test", "Status", "Category"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in task details: %s", want, got)
		}
	}
	if strings.Index(got, "Status") < strings.Index(got, "When a session expires") {
		t.Error("metadata appears before the task description")
	}
	if strings.Contains(got, "e  edit") {
		t.Error("unfocused details repeat keyboard shortcuts")
	}
	if strings.Contains(ansi.Strip(got), "Label") {
		t.Error("detail page still uses the old field name")
	}
	for _, size := range [][2]int{{120, 35}, {60, 20}} {
		m.width, m.height = size[0], size[1]
		list := ansi.Strip(m.View().Content)
		if strings.Contains(list, "When a session expires") {
			t.Errorf("description visible in %dx%d task list", size[0], size[1])
		}
		m.focus, m.detailFrom = detailPane, generalPane
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "When a session expires") {
			t.Errorf("description missing from %dx%d detail page", size[0], size[1])
		}
		m.focus = generalPane
	}
	m.focus, m.detailFrom = detailPane, generalPane
	if footer := ansi.Strip(m.renderFooter(120)); !strings.Contains(footer, "esc back") || !strings.Contains(footer, "(d)one/space · (e)dit/enter") || !strings.Contains(footer, "(p)riority") || !strings.Contains(footer, "(c)ategory") {
		t.Error("detail footer does not show navigation and edit shortcuts")
	}
	if footer := ansi.Strip(m.renderFooter(56)); !strings.Contains(footer, "(d)one/space · (e)dit/enter") || !strings.Contains(footer, "· p ·") || !strings.Contains(footer, "(c)ategory") {
		t.Errorf("narrow detail footer lost task shortcuts: %q", footer)
	}
}

func TestTaskRowsKeepTitlesAlignedAndShowDetails(t *testing.T) {
	for priority, foreground := range map[priority]string{
		"high": "240;119;119", "medium": "244;162;97", "low": "244;211;94",
	} {
		selected := renderTaskRow(task{text: "Highlighted title", priority: priority, details: "Extra context"}, 30, true)
		unselected := renderTaskRow(task{text: "Highlighted title", priority: priority, details: "Extra context"}, 30, false)
		if ansi.StringWidth(selected) != 30 || !strings.Contains(ansi.Strip(selected), "○ Highlighted title  ⋯") || !strings.Contains(selected, "38;2;"+foreground+";48;2;36;87;166mHighlighted title") {
			t.Fatalf("%s selected task is not fully highlighted and coloured: %q", priority, selected)
		}
		if ansi.Strip(unselected) != "○ Highlighted title  ⋯" || !strings.Contains(unselected, "38;2;"+foreground+"mHighlighted title") {
			t.Fatalf("%s unselected task is not coloured: %q", priority, unselected)
		}
		done := renderTaskRow(task{text: "Highlighted title", priority: priority, done: true}, 30, false)
		if ansi.Strip(done) != "✓ Highlighted title" || strings.Contains(done, "38;2;"+foreground) {
			t.Fatalf("%s done task is not muted: %q", priority, done)
		}
	}
	open := ansi.Strip(renderTaskRow(task{text: "Same title"}, 24, false))
	done := ansi.Strip(renderTaskRow(task{text: "Same title", done: true}, 24, false))
	if strings.Index(open, "Same title") != strings.Index(done, "Same title") || strings.Contains(open, "⋯") || strings.Contains(done, "⋯") {
		t.Fatalf("completion changed title alignment or added details marker: %q / %q", open, done)
	}
	truncated := ansi.Strip(renderTaskRow(task{text: strings.Repeat("x", 50), details: "More"}, 24, false))
	if ansi.StringWidth(truncated) != 24 || !strings.HasSuffix(truncated, "  ⋯") {
		t.Fatalf("long task lost its details marker: %q", truncated)
	}
}

func TestTaskCountsIncludeCategoriesAndBranches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [x] Generic done\n- [ ] Generic open\n\n# Docs\n\n- [x] Documented\n- [ ] Needs docs\n\n# Branches\n\n## main\n\n- [x] Branch done\n- [ ] Branch open\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if header := ansi.Strip(m.renderHeader(100)); !strings.HasPrefix(header, "  To Do") || !strings.Contains(header, "2/4 general · 1/2 branch") || strings.Contains(header, "main") || strings.Contains(header, "No Git repository") {
		t.Fatalf("header counts = %q", header)
	}
	general := ansi.Strip(m.renderNavigationPane(m.generalTitle(), m.generalRows(), 0, generalPane, 60, 20))
	if !strings.Contains(general, "General  2/4") || !strings.Contains(general, "@Docs  1/2 ›") {
		t.Fatalf("general counts = %s", general)
	}
	branches := ansi.Strip(m.renderNavigationPane(m.branchTitle(), m.branchRows(), 0, branchPane, 60, 20))
	if !strings.Contains(branches, "Git Branches  1/2") || !strings.Contains(branches, "main  1/2 ›") {
		t.Fatalf("branch counts = %s", branches)
	}
	m.generalLabel = "Docs"
	if label := ansi.Strip(m.renderNavigationPane(m.generalTitle(), m.generalRows(), 0, generalPane, 60, 20)); !strings.Contains(label, "General · @Docs  1/2") {
		t.Fatalf("label count = %s", label)
	}
	m.branchFilter = "main"
	if branch := ansi.Strip(m.renderNavigationPane(m.branchTitle(), m.branchRows(), 0, branchPane, 60, 20)); !strings.Contains(branch, "Git Branches · main  1/2") {
		t.Fatalf("selected branch count = %s", branch)
	}
	m.indexMode = true
	if index := ansi.Strip(m.renderIndex(100, 20)); !strings.Contains(index, "All tasks  3/6") {
		t.Fatalf("all tasks count = %s", index)
	}
}

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
	content := "- [ ] Unlabeled\n\n# auth\n\n- [ ] Login task\n\n# tests\n\n- [ ] Another test\n\n# Branches\n\n## feature/login\n\n- [ ] Branch login\n\n## fix/api\n\n- [ ] Branch API\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{branch: "fix/api"})
	if err != nil {
		t.Fatal(err)
	}
	general := m.generalRows()
	if len(general) != 3 || general[0].kind != rowLabel || general[0].name != "auth" || general[0].count != 1 || general[1].kind != rowLabel || general[1].name != "tests" || general[2].todo.text != "Unlabeled" {
		t.Fatalf("general rows = %+v", general)
	}
	list := ansi.Strip(m.renderNavigationPane(m.generalTitle(), general, 0, generalPane, 50, 18))
	if strings.Contains(list, "[ ]") || !strings.Contains(list, "@auth") || strings.Contains(list, "Login task") || strings.Contains(list, "Branch login") {
		t.Fatalf("root General list showed labeled or branch tasks: %s", list)
	}
	if _, ok := m.selectedTask(); ok {
		t.Fatal("label row selected a task")
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(*model)
	if m.generalLabel != "auth" || m.generalTitle() != "General · @auth" || len(m.generalRows()) != 1 || m.generalRows()[0].todo.text != "Login task" {
		t.Fatalf("label did not filter general tasks: %+v", m.generalRows())
	}
	if opened := ansi.Strip(m.renderNavigationPane(m.generalTitle(), m.generalRows(), 0, generalPane, 50, 12)); strings.Contains(opened, "  Login task") || !strings.Contains(opened, "Login task") {
		t.Fatalf("label contents were indented: %s", opened)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(*model)
	if m.focus != detailPane {
		t.Fatal("right arrow on a filtered task did not focus details")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.focus != generalPane || m.generalLabel != "auth" {
		t.Fatal("left arrow from details skipped the filtered task list")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(*model)
	if m.generalLabel != "" || m.generalCursor != 0 {
		t.Fatal("left arrow did not return to General labels")
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
	if m.categoryInput || !ok || selected.text != "Fix login" || taskLabel(selected) != "backend" {
		t.Fatalf("relabel lost selection: %+v", m.generalRows())
	}
}

func TestLabelInputBlocksSpaces(t *testing.T) {
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
		t.Fatalf("space key changed label input to %q", got)
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
	if m.modal.addLabel != "auth" {
		t.Fatal("add form did not inherit selected label")
	}
	m.modal.title.SetValue("New auth task")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if len(m.generalRows()) != 2 || !taskHasLabel(m.generalRows()[1].todo, "auth") || m.generalRows()[1].todo.text != "New auth task" || m.generalCursor != 1 {
		t.Fatalf("new task was not added to label: %+v", m.generalRows())
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
	if m.branchFilter != "feature/login" || len(m.branchRows()) != 2 || m.branchRows()[1].todo.text != "New branch task" {
		t.Fatalf("new task was not added to branch: %+v", m.branchRows())
	}
	opened, _ = m.startTaskModal(modalAddBranch)
	m = opened.(*model)
	if m.modal.addLabel != "" {
		t.Fatal("branch add form unexpectedly inherited a category")
	}
	m.modal.title.SetValue("Another branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if len(m.branchRows()) != 3 || m.branchRows()[m.branchCursor].todo.text != "Another branch task" {
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
	if rows := m.generalRows(); len(rows) != 1 || rows[0].kind != rowLabel {
		t.Fatalf("expected only the category row, got %+v", rows)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addLabel != "" {
		t.Fatalf("root add inherited selected category: %+v", m.modal)
	}
	m.modal.title.SetValue("General task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	if m.generalLabel != "" {
		t.Fatalf("root add opened category %q", m.generalLabel)
	}
	selected, ok := m.selectedTask()
	if !ok || selected.text != "General task" || taskLabel(selected) != "" {
		t.Fatalf("new general task was not selected or was labeled: %+v", m.generalRows())
	}
	m.generalCursor = 0
	if !m.enterSelectedGroup() {
		t.Fatal("could not open the test category")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(*model)
	if m.modal == nil || m.modal.addLabel != "test" {
		t.Fatalf("category add did not inherit opened category: %+v", m.modal)
	}
	m.modal.title.SetValue("Another test task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if m.generalLabel != "test" || !ok || selected.text != "Another test task" || taskLabel(selected) != "test" {
		t.Fatalf("new category task was not selected or labeled: %+v", m.generalRows())
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
	if !ok || selected.text != "Second feature task" || selected.branch != "feature/a" {
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
		t.Fatal("tab did not focus label field")
	}
	m.modal.resize(56, 16)
	if view := ansi.Strip(m.modal.render(54, 12)); !strings.Contains(view, "Category") || !strings.Contains(view, "Optional category") {
		t.Fatalf("category field is not visible in the small add form: %s", view)
	}
	m.modal.title.SetValue("New labeled task")
	updated, _ = m.Update(tea.PasteMsg{Content: "@new label"})
	m = updated.(*model)
	if m.modal.scope.Value() != "@newlabel" {
		t.Fatalf("pasted label kept spaces: %q", m.modal.scope.Value())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(*model)
	selected, ok := m.selectedTask()
	if m.modal != nil || !ok || taskLabel(selected) != "newlabel" || m.generalLabel != "newlabel" {
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
	if m.modal != nil || !ok || selected.branch != "feature/new" || m.branchFilter != "feature/new" {
		t.Fatalf("new Markdown branch was not created and opened: %+v", m.branchRows())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# newlabel") || !strings.Contains(string(data), "# Branches\n\n## feature/new") {
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
	if m.modal != nil || !ok || taskLabel(selected) != "+v1" || m.generalLabel != "+v1" {
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
	if m.modal != nil || !ok || selected.branch != "fix/auth" {
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
		if !strings.Contains(plain, "Branch") || !strings.Contains(plain, " feature/auth") || !strings.Contains(plain, " feature/ui") || !strings.Contains(plain, "Details") {
			t.Errorf("picker content missing at %dx%d: %s", size[0], size[1], plain)
		}
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
		for _, line := range strings.Split(help, "\n") {
			if strings.Trim(line, " │") == "Task" {
				t.Fatal("add form repeats the task label")
			}
		}
	}
	if rendered := m.modal.render(76, 20); !strings.Contains(rendered, mutedStyle.Render("Category")) {
		t.Fatal("unfocused category label is not muted")
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
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
		t.Fatal("down arrow did not focus label")
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
		t.Fatal("down arrow did not return to label")
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
		t.Fatal("up arrow on first details line did not focus label")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(*model)
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(*model)
	if m.modal != nil || len(m.general) != 1 || m.general[0].details != "Added in modal.\n- [ ] Nested step" {
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
	if len(m.general) != 1 || m.general[0].text != "Renamed task" || m.general[0].details != "Revised details." {
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
	if !ok || selected.text != "Fix login redirect" {
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
	if m.modal == nil || m.modal.title.Value() != "First task" || m.general[0].done {
		t.Fatal("Enter did not open the selected task for editing")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(*model)
	if !m.general[0].done {
		t.Fatal("d did not complete the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if m.general[0].done {
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
	if selected, ok := m.selectedTask(); !ok || !selected.done {
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

func TestIndexShowsEveryMarkdownTaskAndSorts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Plain task\n\n# zeta\n\n- [ ] Medium task\n  - Priority: Medium\n\n  More context\n  on another line.\n\n# Branches\n\n## feature/z\n\n- [ ] Low task\n  - Priority: Low\n\n## feature/a\n\n- [x] High task\n  - Priority: High\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(*model)
	if !m.indexMode || m.indexSort != "priority" {
		t.Fatalf("index opened with sort %q", m.indexSort)
	}
	if got := indexTitles(m.indexTasks()); got != "Medium task,Low task,Plain task,High task" {
		t.Fatalf("priority order = %q", got)
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
		m.width, m.height = size[0], size[1]
		view := m.View().Content
		if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
			t.Errorf("index size at %dx%d = %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
		}
		plain := ansi.Strip(view)
		for _, want := range []string{"All tasks  1/4", "TASK: DETAILS", "CATEGORY / BRANCH", "@zeta", " feature/a", "High task", "More context on another line."} {
			if !strings.Contains(plain, want) && size[0] == 120 {
				t.Errorf("index missing %q: %s", want, plain)
			}
		}
		if strings.Contains(plain, "File TODOs") {
			t.Error("source TODOs appeared in Markdown index")
		}
		if strings.Contains(plain, "PRI  ") {
			t.Error("priority column is still visible")
		}
	}
	rendered := m.renderIndex(120, 20)
	plain := ansi.Strip(rendered)
	lines := strings.Split(plain, "\n")
	if len(lines) < 4 || strings.Trim(lines[2], " │") != "" || strings.Index(lines[3], "TASK: DETAILS") > strings.Index(lines[3], "CATEGORY / BRANCH") {
		t.Fatalf("all tasks heading and columns are out of order: %s", plain)
	}
	if !strings.Contains(rendered, "38;2;244;162;97") {
		t.Error("medium priority did not color the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'B', Text: "B"})
	m = updated.(*model)
	if m.indexSort != "priority" {
		t.Fatal("old branch sort shortcut still changed the sort")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(*model)
	if got := indexTitles(m.indexTasks()); m.indexSort != "branch" || got != "High task,Low task,Medium task,Plain task" {
		t.Fatalf("branch order = %q", got)
	}
	if selected, _ := m.selectedTask(); selected.text != "Medium task" {
		t.Fatal("sort lost selected task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(*model)
	if m.indexSort != "category" || !strings.Contains(ansi.Strip(m.renderIndex(120, 20)), "CATEGORY / BRANCH") {
		t.Fatal("category sort or column heading is missing")
	}
	if got := indexTitles(m.indexTasks()); got != "Medium task,High task,Low task,Plain task" {
		t.Fatalf("category order = %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(*model)
	if m.indexSort != "priority" || indexTitles(m.indexTasks()) != "Medium task,Low task,Plain task,High task" {
		t.Fatal("sort did not cycle back to priority")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	if m.indexMode {
		t.Fatal("Escape did not return to dashboard")
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
		m.indexSort = item.sort
		if got := indexTitles(m.indexTasks()); got != item.want {
			t.Errorf("%s order = %q, want %q", item.sort, got, item.want)
		}
	}
	if rows := m.generalRows(); rows[1].todo.text != "General open" || rows[2].todo.text != "General done" {
		t.Fatalf("general tasks are out of order: %+v", rows)
	}
	m.generalLabel = "auth"
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

func TestFooterOmitsObviousMovementHints(t *testing.T) {
	m := &model{}
	for _, pane := range []pane{generalPane, branchPane, sourcePane, detailPane} {
		m.focus = pane
		if footer := ansi.Strip(m.renderFooter(120)); strings.Contains(footer, "↑/↓") || strings.Contains(footer, "move") || strings.Contains(footer, "scroll") {
			t.Errorf("movement hint remains on pane %d: %q", pane, footer)
		}
	}
	m.indexMode = true
	if footer := ansi.Strip(m.renderFooter(120)); strings.Contains(footer, "↑/↓") || strings.Contains(footer, "move") {
		t.Errorf("movement hint remains in All tasks: %q", footer)
	}
}

func indexTitles(tasks []task) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.text
	}
	return strings.Join(names, ",")
}

func TestIndexEditShowsTaskLocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Plain task\n\n# auth\n\n- [ ] Category task\n\n# Branches\n\n## feature/ui\n\n- [ ] Branch task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.indexMode = true
	for _, item := range []struct{ task, location string }{
		{"Plain task", "General task"},
		{"Category task", "Category  @auth"},
		{"Branch task", "Branch     feature/ui"},
	} {
		for i, task := range m.indexTasks() {
			if task.text == item.task {
				m.indexCursor = i
				break
			}
		}
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
		m = updated.(*model)
		if m.modal == nil {
			t.Fatalf("edit did not open for %s", item.task)
		}
		for _, size := range [][2]int{{76, 20}, {54, 12}} {
			m.modal.resize(size[0]+2, size[1]+4)
			rendered := m.modal.render(size[0], size[1])
			if !strings.Contains(ansi.Strip(rendered), item.location) || lipgloss.Width(rendered) != size[0] || lipgloss.Height(rendered) != size[1] {
				t.Errorf("edit location missing or overflowing for %s at %dx%d: %s", item.task, size[0], size[1], ansi.Strip(rendered))
			}
		}
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
		m = updated.(*model)
	}
}

func TestIndexTaskActionsAndPriorityPalette(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("## General\n\n- [ ] First\n  - Priority: High\n\n- [ ] Second\n  - Priority: Low\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.indexMode = true
	m.indexSort = "priority"
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); !ok || selected.text != "First" || !selected.done {
		t.Fatal("Space did not complete the selected indexed task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = updated.(*model)
	if m.modal == nil || m.modal.title.Value() != "First" {
		t.Fatal("Edit did not open the indexed task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); !ok || selected.text != "First" || selected.priority != "medium" || m.indexSort != "priority" {
		t.Fatal("p should change the selected task's priority without changing sort")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(*model)
	if !m.categoryInput || m.indexSort != "priority" {
		t.Fatal("c should edit the selected task's category without changing sort")
	}
	for priority, want := range map[priority]color.Color{"high": colorHigh, "medium": colorMedium, "low": colorLow} {
		if got := priorityStyle(priority).GetForeground(); got != want {
			t.Errorf("%s priority color = %v, want %v", priority, got, want)
		}
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
	if !ok || selected.text != "Bare" {
		t.Fatalf("setup selected %+v", selected)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if !ok || selected.text != "Bare" || selected.priority != "high" {
		t.Fatalf("first p selected %+v", selected)
	}
	if rows := m.generalRows(); rows[2].todo.text != "Urgent" || rows[3].todo.text != "Routine" || rows[4].todo.text != "Bare" || rows[5].todo.text != "Another unprioritized" || m.generalCursor != 4 {
		t.Fatalf("priority edit moved the highlighted task: cursor %d, rows %+v", m.generalCursor, rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	selected, ok = m.selectedTask()
	if !ok || selected.text != "Bare" || selected.priority != "medium" {
		t.Fatalf("second p selected %+v", selected)
	}
	for _, want := range []priority{priorityLow, priorityNone, priorityHigh} {
		updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		m = updated.(*model)
		selected, ok = m.selectedTask()
		if !ok || selected.text != "Bare" || selected.priority != want {
			t.Fatalf("cycling to %q selected %+v", want, selected)
		}
		if rows := m.generalRows(); rows[4].todo.text != "Bare" || rows[5].todo.text != "Another unprioritized" || m.generalCursor != 4 {
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
	if rows := m.generalRows(); rows[3].todo.text != "Routine" || rows[4].todo.text != "Another unprioritized" || rows[5].todo.text != "Bare" || m.generalCursor != 5 {
		t.Fatalf("completed task did not move after open tasks while staying selected: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(*model)
	if rows := m.generalRows(); rows[3].todo.text != "Routine" || rows[4].todo.text != "Another unprioritized" || rows[5].todo.text != "Bare" {
		t.Fatalf("reload did not leave completed tasks last: %+v", rows)
	}
	items, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.text == "Routine" && item.priority != "medium" {
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
	if !ok || selected.text != "Moved task" || selected.priority != "high" {
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
	for _, want := range []priority{priorityNone, priorityHigh, priorityMedium, priorityLow} {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		m = updated.(*model)
		selected, ok := m.selectedTask()
		if !ok || selected.text != "Detailed task" || selected.priority != want || selected.details != "This task has details." {
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
	if rows := m.generalRows(); rows[0].todo.text != "High priority" || rows[1].todo.text != "Medium priority" || rows[2].todo.text != "No priority" {
		t.Fatalf("startup order = %+v", rows)
	}
	m.generalCursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	if rows := m.generalRows(); rows[2].todo.text != "No priority" || m.generalCursor != 2 {
		t.Fatalf("priority edit resorted rows: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(*model)
	if rows := m.generalRows(); rows[0].todo.text != "No priority" || rows[1].todo.text != "High priority" || rows[2].todo.text != "Medium priority" {
		t.Fatalf("reload order = %+v", rows)
	}
}
