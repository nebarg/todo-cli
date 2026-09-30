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
	m := model{focus: sourcePane}
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
	m := model{focus: sourcePane, sourceError: "permission denied"}
	got := strings.Join(m.sourceDetails(60), "\n")
	if !strings.Contains(got, "permission denied") {
		t.Fatalf("scan error missing from detail pane: %s", got)
	}
}

func TestTaskDetailsShownOnDetailPage(t *testing.T) {
	if taskTitleStyle.GetForeground() != lipgloss.Color("#FFFFFF") {
		t.Fatal("task title is not styled with the white text color")
	}
	m := model{general: []task{{text: "Fix login redirect", details: "When a session expires, return to the previous page.\n\n- Add a regression test"}}}
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
	for priority, foreground := range map[string]string{
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
	m = updated.(model)
	if m.generalLabel != "auth" || m.generalTitle() != "General · @auth" || len(m.generalRows()) != 1 || m.generalRows()[0].todo.text != "Login task" {
		t.Fatalf("label did not filter general tasks: %+v", m.generalRows())
	}
	if opened := ansi.Strip(m.renderNavigationPane(m.generalTitle(), m.generalRows(), 0, generalPane, 50, 12)); strings.Contains(opened, "  Login task") || !strings.Contains(opened, "Login task") {
		t.Fatalf("label contents were indented: %s", opened)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(model)
	if m.focus != detailPane {
		t.Fatal("right arrow on a filtered task did not focus details")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(model)
	if m.focus != generalPane || m.generalLabel != "auth" {
		t.Fatal("left arrow from details skipped the filtered task list")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(model)
	if m.generalLabel != "" || m.generalCursor != 0 {
		t.Fatal("left arrow did not return to General labels")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(model)
	branches := m.branchRows()
	if len(branches) != 2 || branches[m.branchCursor].name != "fix/api" {
		t.Fatalf("current branch was not preselected: cursor %d rows %+v", m.branchCursor, branches)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(model)
	if m.branchFilter != "fix/api" || len(m.branchRows()) != 1 || m.branchRows()[0].todo.text != "Branch API" {
		t.Fatalf("branch did not open: %+v", m.branchRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(model)
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
	m = updated.(model)
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
	m = opened.(model)
	if m.inputMode != "category" {
		t.Fatal("c did not open category editing")
	}
	if got := m.input.Prompt; got != "Category: " {
		t.Fatalf("category prompt = %q", got)
	}
	m.input.SetValue("backend")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)
	selected, ok := m.selectedTask()
	if m.inputMode != "" || !ok || selected.text != "Fix login" || taskLabel(selected) != "backend" {
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
	opened, _ := m.startInput("category")
	m = opened.(model)
	m.input.SetValue("a")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = updated.(model)
	if got := m.input.Value(); got != "a" {
		t.Fatalf("space key changed label input to %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: " category"})
	m = updated.(model)
	if got := m.input.Value(); got != "acategory" {
		t.Fatalf("multi-character input kept a space: %q", got)
	}
	updated, _ = m.Update(tea.PasteMsg{Content: " more words"})
	m = updated.(model)
	if got := m.input.Value(); got != "acategorymorewords" {
		t.Fatalf("pasted input kept spaces: %q", got)
	}
}

func TestAddingWithinGroupsKeepsScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# auth\n\n- [ ] Existing\n\n# Branches\n\n## feature/login\n\n- [ ] Branch task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	opened, _ := m.startTaskModal("add-general")
	m = opened.(model)
	if m.modal.addLabel != "auth" {
		t.Fatal("add form did not inherit selected label")
	}
	m.modal.title.SetValue("New auth task")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if len(m.generalRows()) != 2 || !taskHasLabel(m.generalRows()[1].todo, "auth") || m.generalRows()[1].todo.text != "New auth task" || m.generalCursor != 1 {
		t.Fatalf("new task was not added to label: %+v", m.generalRows())
	}
	m.focus = branchPane
	opened, _ = m.startTaskModal("add-branch")
	m = opened.(model)
	if m.modal.addBranch != "feature/login" {
		t.Fatalf("add form chose %q instead of selected branch", m.modal.addBranch)
	}
	m.modal.title.SetValue("New branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if m.branchFilter != "feature/login" || len(m.branchRows()) != 2 || m.branchRows()[1].todo.text != "New branch task" {
		t.Fatalf("new task was not added to branch: %+v", m.branchRows())
	}
	opened, _ = m.startTaskModal("add-branch")
	m = opened.(model)
	if m.modal.addLabel != "" {
		t.Fatal("branch add form unexpectedly inherited a category")
	}
	m.modal.title.SetValue("Another branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if len(m.branchRows()) != 3 || m.branchRows()[m.branchCursor].todo.text != "Another branch task" {
		t.Fatalf("new task was not added inside branch: %+v", m.branchRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updated.(model)
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
	m = updated.(model)
	if m.modal == nil || m.modal.addLabel != "" {
		t.Fatalf("root add inherited selected category: %+v", m.modal)
	}
	m.modal.title.SetValue("General task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
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
	m = updated.(model)
	if m.modal == nil || m.modal.addLabel != "test" {
		t.Fatalf("category add did not inherit opened category: %+v", m.modal)
	}
	m.modal.title.SetValue("Another test task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	selected, ok = m.selectedTask()
	if m.generalLabel != "test" || !ok || selected.text != "Another test task" || taskLabel(selected) != "test" {
		t.Fatalf("new category task was not selected or labeled: %+v", m.generalRows())
	}
}

func TestAddShortcutUsesSelectedBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "# Branches\n\n## feature/a\n\n- [ ] Existing feature task\n\n## main\n\n- [ ] Existing main task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	m.branchCursor = 0 // feature/a is selected; main is the current Git branch.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(model)
	if m.modal == nil || m.modal.mode != "add-branch" || m.modal.addBranch != "feature/a" {
		t.Fatalf("a did not target selected branch: %+v", m.modal)
	}
	m.modal.title.SetValue("First feature task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if m.branchFilter != "feature/a" {
		t.Fatalf("add did not open selected branch: %q", m.branchFilter)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(model)
	if m.modal == nil || m.modal.addBranch != "feature/a" {
		t.Fatalf("a did not target open branch: %+v", m.modal)
	}
	m.modal.title.SetValue("Second feature task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	selected, ok := m.selectedTask()
	if !ok || selected.text != "Second feature task" || selected.branch != "feature/a" {
		t.Fatalf("new task was not selected in feature/a: %+v", m.branchRows())
	}
	m.focus = generalPane
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(model)
	if m.modal == nil || m.modal.mode != "add-branch" || m.modal.addBranch != "main" {
		t.Fatalf("b no longer explicitly targets the current Git branch: %+v", m.modal)
	}
}

func TestAddFormCreatesCategoryAndMarkdownBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(model)
	if m.modal.field != 1 {
		t.Fatal("tab did not focus label field")
	}
	m.modal.resize(56, 16)
	if view := ansi.Strip(m.modal.render(54, 12)); !strings.Contains(view, "Category") || !strings.Contains(view, "Optional category") {
		t.Fatalf("category field is not visible in the small add form: %s", view)
	}
	m.modal.title.SetValue("New labeled task")
	updated, _ = m.Update(tea.PasteMsg{Content: "@new label"})
	m = updated.(model)
	if m.modal.scope.Value() != "@newlabel" {
		t.Fatalf("pasted label kept spaces: %q", m.modal.scope.Value())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	selected, ok := m.selectedTask()
	if m.modal != nil || !ok || taskLabel(selected) != "newlabel" || m.generalLabel != "newlabel" {
		t.Fatalf("new category was not created and opened: %+v", m.generalRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = updated.(model)
	if m.modal == nil || m.modal.mode != "add-branch" || m.modal.scope.Value() != "" {
		t.Fatalf("branch form did not open without a Git branch: %+v", m.modal)
	}
	m.modal.resize(56, 16)
	if view := ansi.Strip(m.modal.render(54, 12)); !strings.Contains(view, "Branch") || !strings.Contains(view, "Branch name") {
		t.Fatalf("branch field is not visible in the small add form: %s", view)
	}
	m.modal.title.SetValue("New branch task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if m.modal == nil || m.modal.err != "enter a branch name" {
		t.Fatalf("empty branch name was accepted: %+v", m.modal)
	}
	m.modal.scope.SetValue("feature/new")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
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
	m = updated.(model)
	if m.modal == nil || m.modal.mode != "add-branch" || m.modal.scope.Value() != "feature/new" {
		t.Fatalf("b did not open branch add from All tasks: %+v", m.modal)
	}
	m.modal.title.SetValue("Task from All tasks")
	m.modal.scope.SetValue("feature/index")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if m.indexMode || m.branchFilter != "feature/index" {
		t.Fatalf("branch add from All tasks did not open the new section: %q", m.branchFilter)
	}
}

func TestTaskModalAddsAndEditsDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = opened.(model)
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
	m = opened.(model)
	if m.modal == nil || m.modal.err == "" {
		t.Fatal("empty title should keep the form open with an error")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = opened.(model)
	if m.modal.title.Value() != "F" {
		t.Fatalf("title field ignored typing: %q", m.modal.title.Value())
	}
	m.modal.title.SetValue("First task")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(model)
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
	m = opened.(model)
	if m.modal.field != 0 {
		t.Fatal("up arrow on the first details line did not focus title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(model)
	if m.modal.field != 1 {
		t.Fatal("down arrow did not return to label")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(model)
	if m.modal.field != 2 {
		t.Fatal("down arrow did not focus details")
	}
	if help := ansi.Strip(m.modal.render(76, 20)); !strings.Contains(help, "↑/↓/tab navigate · ctrl+enter save · esc cancel") {
		t.Fatalf("details help changed by focus: %s", help)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	m = opened.(model)
	if m.modal.details.Value() != "X" {
		t.Fatalf("details field ignored typing: %q", m.modal.details.Value())
	}
	m.modal.details.SetValue("Added in modal.\n- [ ] Nested step")
	if m.modal.details.Line() != 1 {
		t.Fatalf("details cursor should start on the final line, got %d", m.modal.details.Line())
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(model)
	if m.modal.field != 2 || m.modal.details.Line() != 0 {
		t.Fatal("up arrow should move within multiline details before returning to title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(model)
	if m.modal.field != 1 {
		t.Fatal("up arrow on first details line did not focus label")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(model)
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(model)
	if m.modal != nil || len(m.general) != 1 || m.general[0].details != "Added in modal.\n- [ ] Nested step" {
		t.Fatalf("task was not saved: %+v", m.general)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = opened.(model)
	if m.modal == nil || m.modal.title.Value() != "First task" {
		t.Fatal("edit did not reopen the task in the modal")
	}
	m.modal.title.SetValue("Renamed task")
	m.modal.details.SetValue("Revised details.")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(model)
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
	m = opened.(model)
	if got := lipgloss.Height(m.modal.title.View()); got != 2 {
		t.Fatalf("task input is %d lines, want 2", got)
	}
	m.modal.title.SetValue("Fix login\nredirect")
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = opened.(model)
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
	m := model{general: []task{{text: "First task"}}, width: 100, height: 30}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(model)
	if m.focus != detailPane || m.detailFrom != generalPane {
		t.Fatalf("right arrow page = %v from %v", m.focus, m.detailFrom)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(model)
	if m.focus != generalPane {
		t.Fatal("Escape did not return to task list")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	m = updated.(model)
	if m.focus != sourcePane {
		t.Fatal("3 did not switch to file TODOs")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(model)
	if m.focus != sourcePane {
		t.Fatal("right arrow opened details without a selected file TODO")
	}
	updated, _ = m.Update(sourceScanMsg{matches: nil})
	m = updated.(model)
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
	m = updated.(model)
	if m.modal == nil || m.modal.title.Value() != "First task" || m.general[0].done {
		t.Fatal("Enter did not open the selected task for editing")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(model)
	if !m.general[0].done {
		t.Fatal("d did not complete the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(model)
	if m.general[0].done {
		t.Fatal("Space did not reopen the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)
	if m.modal == nil || m.focus != detailPane {
		t.Fatal("Enter from details did not open the edit form")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)
	if m.modal == nil || !m.indexMode {
		t.Fatal("Enter from All tasks did not open the edit form")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(model)
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
	m = updated.(model)
	if !m.indexMode || m.indexSort != "priority" {
		t.Fatalf("index opened with sort %q", m.indexSort)
	}
	if got := indexTitles(m.indexTasks()); got != "High task,Medium task,Low task,Plain task" {
		t.Fatalf("priority order = %q", got)
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
		m.width, m.height = size[0], size[1]
		view := m.View().Content
		if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
			t.Errorf("index size at %dx%d = %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
		}
		plain := ansi.Strip(view)
		for _, want := range []string{"All tasks  1/4", "!", "@zeta", "feature/a", "High task", "More context on another line."} {
			if !strings.Contains(plain, want) && size[0] == 120 {
				t.Errorf("index missing %q: %s", want, plain)
			}
		}
		if strings.Contains(plain, "File TODOs") {
			t.Error("source TODOs appeared in Markdown index")
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'B', Text: "B"})
	m = updated.(model)
	if got := indexTitles(m.indexTasks()); got != "High task,Low task,Medium task,Plain task" {
		t.Fatalf("branch order = %q", got)
	}
	if selected, _ := m.selectedTask(); selected.text != "High task" {
		t.Fatal("sort lost selected task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(model)
	if m.indexSort != "category" || !strings.Contains(ansi.Strip(m.renderIndex(120, 20)), "CATEGORY") {
		t.Fatal("category sort or column heading is missing")
	}
	if got := indexTitles(m.indexTasks()); got != "Medium task,High task,Low task,Plain task" {
		t.Fatalf("category order = %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(model)
	if m.indexMode {
		t.Fatal("Escape did not return to dashboard")
	}
}

func indexTitles(tasks []task) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.text
	}
	return strings.Join(names, ",")
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
	m = updated.(model)
	if selected, ok := m.selectedTask(); !ok || selected.text != "First" || !selected.done {
		t.Fatal("Space did not complete the selected indexed task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = updated.(model)
	if m.modal == nil || m.modal.title.Value() != "First" {
		t.Fatal("Edit did not open the indexed task")
	}
	if priorityMarker("high") != "!" || priorityMarker("medium") != "!" || priorityMarker("low") != "!" || priorityMarker("") != "-" {
		t.Fatal("wrong priority markers")
	}
	for priority, want := range map[string]color.Color{"high": colorHigh, "medium": colorMedium, "low": colorLow} {
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
	m = updated.(model)
	selected, ok = m.selectedTask()
	if !ok || selected.text != "Bare" || selected.priority != "high" {
		t.Fatalf("first p selected %+v", selected)
	}
	if rows := m.generalRows(); rows[2].todo.text != "Urgent" || rows[3].todo.text != "Routine" || rows[4].todo.text != "Bare" || rows[5].todo.text != "Another unprioritized" || m.generalCursor != 4 {
		t.Fatalf("priority edit moved the highlighted task: cursor %d, rows %+v", m.generalCursor, rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(model)
	selected, ok = m.selectedTask()
	if !ok || selected.text != "Bare" || selected.priority != "medium" {
		t.Fatalf("second p selected %+v", selected)
	}
	for _, want := range []string{"low", "", "high"} {
		updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		m = updated.(model)
		selected, ok = m.selectedTask()
		if !ok || selected.text != "Bare" || selected.priority != want {
			t.Fatalf("cycling to %q selected %+v", want, selected)
		}
		if rows := m.generalRows(); rows[4].todo.text != "Bare" || rows[5].todo.text != "Another unprioritized" || m.generalCursor != 4 {
			t.Fatalf("cycling to %q moved the row: cursor %d, rows %+v", want, m.generalCursor, rows)
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(model)
	if got := indexTitles(m.indexTasks()); !strings.HasPrefix(got, "Urgent,Routine,Bare,Another unprioritized") {
		t.Fatalf("opening the full list unexpectedly resorted tasks: %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(model)
	if got := indexTitles(m.indexTasks()); !strings.HasPrefix(got, "Urgent,Bare,Routine") {
		t.Fatalf("explicit full-list priority sort failed: %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(model)
	if rows := m.generalRows(); rows[3].todo.text != "Routine" || rows[4].todo.text != "Bare" || m.generalCursor != 4 {
		t.Fatalf("completing a task unexpectedly resorted the list: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(model)
	if rows := m.generalRows(); rows[3].todo.text != "Bare" || rows[4].todo.text != "Routine" {
		t.Fatalf("reload did not sort by priority: %+v", rows)
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
	m = updated.(model)
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
	for _, want := range []string{"", "high", "medium", "low"} {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		m = updated.(model)
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
	m = updated.(model)
	if rows := m.generalRows(); rows[2].todo.text != "No priority" || m.generalCursor != 2 {
		t.Fatalf("priority edit resorted rows: %+v", rows)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = updated.(model)
	if rows := m.generalRows(); rows[0].todo.text != "No priority" || rows[1].todo.text != "High priority" || rows[2].todo.text != "Medium priority" {
		t.Fatalf("reload order = %+v", rows)
	}
}
