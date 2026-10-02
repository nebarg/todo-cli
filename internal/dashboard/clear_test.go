package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/project/projecttest"
	"github.com/nebarg/todo-cli/internal/ui"
)

const clearContent = "- [x] Loose done\n\n- [ ] Loose open\n\n# docs\n\n- [x] Doc done\n\n# auth\n\n- [x] Auth done\n\n- [ ] Auth open\n\n# Branches\n\n## feature/x\n\n- [x] Branch done\n"

func clearModel(t *testing.T) (*model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte(clearContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	return m, path
}

func press(m *model, key string) *model {
	msg := tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	switch key {
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEsc}
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "backspace":
		msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	updated, _ := m.Update(msg)
	return updated.(*model)
}

func fileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestClearDoneFollowsWhereYouAre(t *testing.T) {
	for _, item := range []struct {
		name   string
		setup  func(m *model)
		prompt string
		gone   []string
		kept   []string
	}{
		{"General", func(m *model) {}, "Remove 3 done tasks from General?", []string{"Loose done", "Doc done", "Auth done", "# docs"}, []string{"Branch done", "# auth"}},
		{"category", func(m *model) { m.general.open = categoryGroup("auth") }, "Remove 1 done task from @auth?", []string{"Auth done"}, []string{"Loose done", "Doc done"}},
		{"Branches", func(m *model) { m.focus = branchPane }, "Remove 1 done task from Branches?", []string{"Branch done", "# Branches"}, []string{"Loose done"}},
		{"all tasks", func(m *model) { m.openAllTasks() }, "Remove 4 done tasks from all tasks?", []string{"Loose done", "Doc done", "Auth done", "Branch done"}, []string{"Loose open", "Auth open"}},
	} {
		t.Run(item.name, func(t *testing.T) {
			m, path := clearModel(t)
			item.setup(m)
			m = press(m, "X")
			if !isOpen[*clearConfirmation](m) {
				t.Fatalf("X did not ask for confirmation: %q", m.status)
			}
			if dialog := ansi.Strip(m.View().Content); !strings.Contains(dialog, item.prompt) {
				t.Fatalf("dialog lacks %q: %s", item.prompt, dialog)
			}
			if fileContent(t, path) != clearContent {
				t.Fatal("asking for confirmation changed the file")
			}
			m = press(m, "y")
			content := fileContent(t, path)
			for _, gone := range item.gone {
				if strings.Contains(content, gone+"\n") {
					t.Errorf("%q was not removed: %s", gone, content)
				}
			}
			for _, kept := range item.kept {
				if !strings.Contains(content, kept+"\n") {
					t.Errorf("%q was removed: %s", kept, content)
				}
			}
			if isOpen[*clearConfirmation](m) || !strings.HasPrefix(m.status, "Removed ") {
				t.Fatalf("status after clearing = %q", m.status)
			}
		})
	}
}

func TestClearDoneNamesHeadingsItEmpties(t *testing.T) {
	m, _ := clearModel(t)
	m.openAllTasks()
	m = press(m, "X")
	dialog := ansi.Strip(confirmation(t, m).render(m.theme))
	if !strings.Contains(dialog, "The @docs and "+branchIcon+" feature/x headings will be empty") || !strings.Contains(dialog, "y remove") {
		t.Fatalf("dialog = %s", dialog)
	}
	for _, size := range [][2]int{{56, 16}, {120, 30}} {
		m.width, m.height = size[0], size[1]
		if view := m.View().Content; lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
			t.Errorf("dialog overflows a %dx%d terminal", size[0], size[1])
		}
	}
}

func TestClearDoneOnlyRemovesOnY(t *testing.T) {
	for _, key := range []string{"esc", "n", "enter", "X"} {
		t.Run(key, func(t *testing.T) {
			m, path := clearModel(t)
			m = press(m, "X")
			updated, _ := m.Update(tea.KeyPressMsg{Code: map[string]rune{"esc": tea.KeyEsc, "n": 'n', "enter": tea.KeyEnter, "X": 'X'}[key], Text: map[string]string{"n": "n", "X": "X"}[key]})
			m = updated.(*model)
			if isOpen[*clearConfirmation](m) || fileContent(t, path) != clearContent {
				t.Fatalf("%s did not cancel cleanly", key)
			}
		})
	}
}

func TestUndoClearRestoresTheFile(t *testing.T) {
	m, path := clearModel(t)
	m = press(press(m, "X"), "y")
	if hints := m.footerHints(); len(hints) == 0 || hints[0] != (ui.KeyHint{Key: "u", Label: "undo"}) {
		t.Fatalf("footer does not offer undo first: %v", hints)
	}
	if help := ansi.Strip(renderHelp(m.theme)); !strings.Contains(help, "X / u clear done / undo") {
		t.Fatalf("help does not list undo: %s", help)
	}
	m = press(m, "u")
	if fileContent(t, path) != clearContent || m.status != "Restored 3 tasks" {
		t.Fatalf("undo left %q with status %q", fileContent(t, path), m.status)
	}
	if m.lastRemoval != nil {
		t.Fatal("undo can be repeated")
	}

	m = press(press(m, "X"), "y")
	if err := os.WriteFile(path, []byte("- [ ] Written elsewhere\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m = press(m, "u")
	if fileContent(t, path) != "- [ ] Written elsewhere\n" || !strings.Contains(m.status, "cannot be undone") {
		t.Fatalf("undo over a changed file = %q, status %q", fileContent(t, path), m.status)
	}
}

func TestLaterEditsDropUndoClear(t *testing.T) {
	m, _ := clearModel(t)
	m.openAllTasks()
	m = press(press(m, "X"), "y")
	m = press(m, "d")
	if m.status != "" {
		t.Fatalf("toggle failed: %q", m.status)
	}
	if m.lastRemoval != nil {
		t.Fatal("undo clear survived another edit")
	}
	for _, hint := range m.footerHints() {
		if hint.Key == "u" {
			t.Fatal("footer still offers undo clear")
		}
	}
}

func TestClearHintOnlyWithDoneTasks(t *testing.T) {
	m, _ := clearModel(t)
	if !strings.Contains(ansi.Strip(m.renderFooter(200)), "X clear 3 done") {
		t.Fatalf("footer lacks the clear hint: %q", ansi.Strip(m.renderFooter(200)))
	}
	m = press(press(m, "X"), "y")
	if footer := ansi.Strip(m.renderFooter(200)); strings.Contains(footer, "X clear") {
		t.Fatalf("footer offers clearing with nothing done: %q", footer)
	}
	m = press(m, "X")
	if isOpen[*clearConfirmation](m) || m.status != "No done tasks to clear" {
		t.Fatalf("X with nothing done = %q", m.status)
	}
	m.focus = sourcePane
	m = press(m, "X")
	if isOpen[*clearConfirmation](m) || m.status != "File TODOs are read only" {
		t.Fatalf("X on file TODOs = %q", m.status)
	}
}

const missingContent = "- [x] Loose done\n\n# Branches\n\n## feature/gone\n\n- [ ] Gone open\n\n- [x] Gone done\n\n## feature/live\n\n- [ ] Live open\n\n- [x] Live done\n"

func missingBranchModel(t *testing.T) (*model, string) {
	t.Helper()
	dir := t.TempDir()
	repo := projecttest.Repo(t, dir, "main", "feature/live")
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte(missingContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, repo, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.focus = branchPane
	return m, path
}

func TestClearInsideMissingBranchDeletesIt(t *testing.T) {
	m, path := missingBranchModel(t)
	m.branch.open = branchGroup("feature/gone")
	if footer := ansi.Strip(m.renderFooter(200)); !strings.Contains(footer, "X remove its tasks") {
		t.Fatalf("missing branch footer lacks remove: %q", footer)
	}
	m = press(m, "X")
	if !isOpen[*clearConfirmation](m) {
		t.Fatalf("X did not ask for confirmation: %q", m.status)
	}
	if dialog := ansi.Strip(confirmation(t, m).render(m.theme)); !strings.Contains(dialog, "Remove the 2 tasks of "+branchIcon+" feature/gone?") || !strings.Contains(dialog, "no longer exists in Git") {
		t.Fatalf("dialog = %s", dialog)
	}
	m = press(m, "y")
	want := "- [x] Loose done\n\n# Branches\n\n## feature/live\n\n- [ ] Live open\n\n- [x] Live done\n"
	if got := fileContent(t, path); got != want || m.status != "Removed the tasks of feature/gone" || m.branch.open.name != "" {
		t.Fatalf("file = %q, status %q, filter %q", got, m.status, m.branch.open.name)
	}
	if press(m, "u"); fileContent(t, path) != missingContent {
		t.Fatal("undo did not bring the branch back")
	}
}

func TestClearingBranchesRemovesMissingOnes(t *testing.T) {
	m, path := missingBranchModel(t)
	if footer := ansi.Strip(m.renderFooter(200)); !strings.Contains(footer, "X clear 1 done + 1 missing") {
		t.Fatalf("footer = %q", footer)
	}
	m = press(m, "X")
	dialog := strings.Join(strings.Fields(strings.ReplaceAll(ansi.Strip(confirmation(t, m).render(m.theme)), "│", " ")), " ")
	for _, want := range []string{"Remove 1 done task and 2 tasks of missing branches from Branches?", branchIcon + " feature/gone no longer exists in Git, so all of its 2 tasks go too."} {
		if !strings.Contains(dialog, want) {
			t.Errorf("dialog lacks %q: %s", want, dialog)
		}
	}
	m = press(m, "y")
	want := "- [x] Loose done\n\n# Branches\n\n## feature/live\n\n- [ ] Live open\n"
	if got := fileContent(t, path); got != want || m.status != "Removed 1 done task and 2 tasks of missing branches" {
		t.Fatalf("file = %q, status %q", got, m.status)
	}
}

func TestIndexMarksMissingBranches(t *testing.T) {
	m, _ := missingBranchModel(t)
	m.openAllTasks()
	view := m.all.view(m.theme, m.tasks.all, m.branchMissing, 100, 20)
	gone := lipgloss.NewStyle().Foreground(m.theme.ColorHigh).Render(ui.Column("⚠ feature/gone", 24))
	if !strings.Contains(view, gone) || !strings.Contains(ansi.Strip(view), branchIcon+" feature/live") {
		t.Fatalf("index does not mark the missing branch:\n%s", view)
	}
}
