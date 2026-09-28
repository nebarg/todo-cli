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
	path := filepath.Join(t.TempDir(), "TODO.md")
	content := "## General\n\n- [ ] Fix failing tests\n\n## Branches\n\n### feature/login\n\n- [ ] Add login check\n"
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
		view := m.View()
		if got := lipgloss.Width(view.Content); got != size[0] {
			t.Errorf("width at %dx%d = %d", size[0], size[1], got)
		}
		if got := lipgloss.Height(view.Content); got != size[1] {
			t.Errorf("height at %dx%d = %d", size[0], size[1], got)
		}
		plain := ansi.Strip(view.Content)
		for _, want := range []string{"General", "Branch", "File TODOs", "Details"} {
			if !strings.Contains(plain, want) {
				t.Errorf("missing %q at %dx%d", want, size[0], size[1])
			}
		}
	}
}

func TestScanErrorVisibleInDetails(t *testing.T) {
	m := model{focus: sourcePane, sourceError: "permission denied"}
	got := strings.Join(m.sourceDetails(60), "\n")
	if !strings.Contains(got, "permission denied") {
		t.Fatalf("scan error missing from detail pane: %s", got)
	}
}

func TestLeftPaneHeights(t *testing.T) {
	tests := []struct {
		name   string
		total  int
		counts [3]int
		want   [3]int
	}{
		{"all occupied", 30, [3]int{15, 1, 1}, [3]int{10, 10, 10}},
		{"empty but everything fits", 31, [3]int{0, 1, 2}, [3]int{11, 10, 10}},
		{"one empty lends space", 30, [3]int{15, 1, 0}, [3]int{16, 10, 4}},
		{"two empty lend space", 30, [3]int{15, 0, 0}, [3]int{22, 4, 4}},
		{"two busy lists share space", 30, [3]int{12, 12, 0}, [3]int{13, 13, 4}},
		{"small terminal", 16, [3]int{9, 0, 0}, [3]int{8, 4, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := leftPaneHeights(tt.total, tt.counts); got != tt.want {
				t.Errorf("leftPaneHeights(%d, %v) = %v, want %v", tt.total, tt.counts, got, tt.want)
			}
		})
	}
}

func TestTaskDetailsShownInDashboard(t *testing.T) {
	if taskTitleStyle.GetForeground() != lipgloss.Color("#FFFFFF") {
		t.Fatal("task title is not styled with the white text color")
	}
	m := model{general: []task{{text: "Fix login redirect", details: "When a session expires, return to the previous page.\n\n- Add a regression test"}}}
	got := strings.Join(m.taskDetails(60), "\n")
	for _, want := range []string{"Fix login redirect", "When a session expires", "- Add a regression test", "Status"} {
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
	for _, size := range [][2]int{{120, 35}, {60, 20}} {
		m.width, m.height = size[0], size[1]
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "When a session expires") {
			t.Errorf("description missing from %dx%d dashboard", size[0], size[1])
		}
	}
	m.focus, m.detailFrom = detailPane, generalPane
	if view := ansi.Strip(m.renderDetailPane(60, 20)); !strings.Contains(view, "Space/Enter: done") || !strings.Contains(view, "(e)dit") || !strings.Contains(view, "(p)riority") || !strings.Contains(view, "(l)abel") {
		t.Error("focused details do not show edit shortcut")
	}
}

func TestLabelsAndBranchesDrillDown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	content := "## General\n\n- [ ] Unlabeled\n\n### @auth\n\n- [ ] Login task\n\n### @tests\n\n- [ ] Another test\n\n## Branches\n\n### feature/login\n\n#### @auth\n\n- [ ] Branch login\n\n### fix/api\n\n- [ ] Branch API\n"
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
	if strings.Contains(branchList, "[ ]") || !strings.Contains(branchList, "@auth") || strings.Contains(branchList, "Branch login") {
		t.Fatalf("branch root showed labeled tasks: %s", branchList)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)
	if m.branchLabel != "auth" || len(m.branchRows()) != 1 || m.branchRows()[0].todo.text != "Branch login" {
		t.Fatalf("branch label did not open: %+v", m.branchRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(model)
	if m.branchLabel != "" || m.branchFilter != "feature/login" || m.branchCursor != 0 {
		t.Fatal("left arrow did not return to branch labels")
	}
}

func TestChangingLabelKeepsTaskSelected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("## General\n\n### @auth\n\n- [ ] Fix login\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	m.enterSelectedGroup()
	opened, _ := m.startInput("labels")
	m = opened.(model)
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
	if err := os.WriteFile(path, []byte("## General\n\n- [ ] Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, projectContext{})
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := m.startInput("labels")
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
	path := filepath.Join(t.TempDir(), "TODO.md")
	content := "## General\n\n### @auth\n\n- [ ] Existing\n\n## Branches\n\n### feature/login\n\n#### @auth\n\n- [ ] Branch task\n"
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
	m.branchCursor = 0
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(model)
	if m.branchLabel != "auth" {
		t.Fatal("branch label did not open before adding")
	}
	opened, _ = m.startTaskModal("add-branch")
	m = opened.(model)
	if m.modal.addLabel != "auth" {
		t.Fatal("add form did not inherit branch label")
	}
	m.modal.title.SetValue("Another auth task")
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	m = updated.(model)
	if m.branchLabel != "auth" || len(m.branchRows()) != 2 || m.branchRows()[m.branchCursor].todo.text != "Another auth task" {
		t.Fatalf("new task was not added inside branch label: %+v", m.branchRows())
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m = updated.(model)
	if m.branchLabel != "" || m.branchFilter != "" || m.branchRows()[m.branchCursor].name != "feature/login" {
		t.Fatal("v did not return to the branch list")
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
	if help := ansi.Strip(m.modal.render(76, 20)); !strings.Contains(help, "↓/Tab Details · Ctrl+Enter Save · Esc Cancel") {
		t.Fatalf("title help has wrong shortcuts or separators: %s", help)
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
		m.width, m.height = size[0], size[1]
		m.modal.resize(m.width, m.height)
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
		t.Fatal("down arrow did not focus details")
	}
	if help := ansi.Strip(m.modal.render(76, 20)); !strings.Contains(help, "↑/Tab Title · Ctrl+Enter Save · Esc Cancel") {
		t.Fatalf("details help has wrong shortcuts or separators: %s", help)
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = opened.(model)
	if m.modal.field != 0 {
		t.Fatal("up arrow on the first details line did not focus title")
	}
	opened, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = opened.(model)
	if m.modal.field != 1 {
		t.Fatal("down arrow did not return to details")
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
	if m.modal.field != 1 || m.modal.details.Line() != 0 {
		t.Fatal("up arrow should move within multiline details before returning to title")
	}
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

func TestArrowFocusesDetailsAndScanKeepsFooter(t *testing.T) {
	m := model{general: []task{{text: "First task"}}, width: 100, height: 30}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(model)
	if m.focus != detailPane || m.detailFrom != generalPane {
		t.Fatalf("right arrow focus = %v from %v", m.focus, m.detailFrom)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(model)
	if m.focus != generalPane {
		t.Fatal("left arrow did not return to task list")
	}
	updated, _ = m.Update(sourceScanMsg{matches: nil})
	m = updated.(model)
	footer := ansi.Strip(m.renderFooter(100))
	if !strings.Contains(footer, "1 General") || !strings.Contains(footer, "3 File TODOs") || strings.Contains(footer, "Found 0") {
		t.Fatalf("scan changed the navigation footer: %q", footer)
	}
	m.status = "No current Git branch"
	footer = ansi.Strip(m.renderFooter(60))
	if !strings.Contains(footer, "No current") || !strings.Contains(footer, "1 General") || !strings.Contains(footer, "3 Files") {
		t.Fatalf("small footer lost error or navigation: %q", footer)
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
	path := filepath.Join(t.TempDir(), "TODO.md")
	content := "## General\n\n- [ ] Plain task\n\n### @zeta\n\n- [ ] Medium task\n  - Priority: Medium\n\n  More context\n  on another line.\n\n## Branches\n\n### feature/z\n\n#### @alpha\n\n- [ ] Low task\n  - Priority: Low\n\n### feature/a\n\n- [x] High task\n  - Priority: High\n"
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
		for _, want := range []string{"All tasks  4", "!!!", "!!", "!", "@zeta", "feature/a", "High task", "More context on another line."} {
			if !strings.Contains(plain, want) && size[0] == 120 {
				t.Errorf("index missing %q: %s", want, plain)
			}
		}
		if strings.Contains(plain, "File TODOs") {
			t.Error("source TODOs appeared in Markdown index")
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(model)
	if got := indexTitles(m.indexTasks()); got != "High task,Low task,Medium task,Plain task" {
		t.Fatalf("branch order = %q", got)
	}
	if selected, _ := m.selectedTask(); selected.text != "High task" {
		t.Fatal("sort lost selected task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = updated.(model)
	if got := indexTitles(m.indexTasks()); got != "Low task,Medium task,High task,Plain task" {
		t.Fatalf("label order = %q", got)
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
	if priorityMarker("high") != "!!!" || priorityMarker("medium") != "!!" || priorityMarker("low") != "!" || priorityMarker("") != "-" {
		t.Fatal("wrong priority markers")
	}
	for priority, want := range map[string]color.Color{"high": colorHigh, "medium": colorMedium, "low": colorLow} {
		if got := priorityStyle(priority).GetForeground(); got != want {
			t.Errorf("%s priority color = %v, want %v", priority, got, want)
		}
	}
}
