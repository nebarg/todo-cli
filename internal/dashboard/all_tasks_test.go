package dashboard

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/store"
)

func TestIndexShowsEveryMarkdownTaskAndSorts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Plain task\n\n# zeta\n\n- [ ] Medium task !medium\n\n  More context\n  on another line.\n\n# Branches\n\n## feature/z\n\n- [ ] Low task !low\n\n## feature/a\n\n- [x] High task !high\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = updated.(*model)
	if m.all == nil || m.all.sort != "priority" {
		t.Fatalf("index opened with sort %q", m.all.sort)
	}
	if got := indexTitles(m.all.sorted(m.tasks.all)); got != "Medium task,Low task,Plain task,High task" {
		t.Fatalf("priority order = %q", got)
	}
	for _, size := range [][2]int{{120, 35}, {78, 16}, {60, 20}, {56, 19}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m.width, m.height = size[0], size[1]
			view := m.View().Content
			if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
				t.Errorf("index size at %dx%d = %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
			}
			plain := ansi.Strip(view)
			for _, want := range []string{"All tasks  1/4", "TASK: DETAILS", "CATEGORY / BRANCH", "@zeta", branchIcon + " feature/a", "High task", "More context on another line."} {
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
		})
	}
	rendered := m.all.view(m.theme, m.tasks.all, m.branchMissing, 120, 20)
	plain := ansi.Strip(rendered)
	lines := strings.Split(plain, "\n")
	if len(lines) < 4 || strings.Trim(lines[2], " │") != "" || strings.Index(lines[3], "TASK: DETAILS") > strings.Index(lines[3], "CATEGORY / BRANCH") {
		t.Fatalf("all tasks heading and columns are out of order: %s", plain)
	}
	if !regexp.MustCompile(`38;2;244;211;94(;48;2;[0-9;]+)?m●`).MatchString(rendered) {
		t.Error("medium priority did not color the task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'B', Text: "B"})
	m = updated.(*model)
	if m.all.sort != "priority" {
		t.Fatal("old branch sort shortcut still changed the sort")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(*model)
	if got := indexTitles(m.all.sorted(m.tasks.all)); m.all.sort != "branch" || got != "High task,Low task,Medium task,Plain task" {
		t.Fatalf("branch order = %q", got)
	}
	if selected, _ := m.selectedTask(); selected.Text != "Medium task" {
		t.Fatal("sort lost selected task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(*model)
	if m.all.sort != "category" || !strings.Contains(ansi.Strip(m.all.view(m.theme, m.tasks.all, m.branchMissing, 120, 20)), "CATEGORY / BRANCH") {
		t.Fatal("category sort or column heading is missing")
	}
	if got := indexTitles(m.all.sorted(m.tasks.all)); got != "Medium task,High task,Low task,Plain task" {
		t.Fatalf("category order = %q", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(*model)
	if m.all.sort != "priority" || indexTitles(m.all.sorted(m.tasks.all)) != "Medium task,Low task,Plain task,High task" {
		t.Fatal("sort did not cycle back to priority")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	if m.all != nil {
		t.Fatal("Escape did not return to dashboard")
	}
}

func indexTitles(tasks []store.Task) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.Text
	}
	return strings.Join(names, ",")
}

func TestIndexEditShowsTaskLocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- [ ] Plain task\n\n# auth\n\n- [ ] Category task\n\n# Branches\n\n## feature/ui\n\n- [ ] Branch task\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.openAllTasks()
	for _, item := range []struct{ task, location string }{
		{"Plain task", "General › Edit task"},
		{"Category task", "General › @auth › Edit task"},
		{"Branch task", "Branches › " + branchIcon + " feature/ui › Edit task"},
	} {
		t.Run(item.task, func(t *testing.T) {
			for i, task := range m.all.sorted(m.tasks.all) {
				if task.Text == item.task {
					m.all.cursor = i
					break
				}
			}
			updated, _ := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
			m = updated.(*model)
			if !isOpen[*taskModal](m) {
				t.Fatalf("edit did not open for %s", item.task)
			}
			for _, size := range [][2]int{{76, 20}, {54, 12}} {
				form(t, m).resize(m.theme, size[0]+2, size[1]+4)
				width, height := form(t, m).dimensions(size[0]+2, size[1]+4)
				rendered := form(t, m).render(m.theme, width, height)
				if !strings.Contains(ansi.Strip(rendered), item.location) || lipgloss.Width(rendered) != width || lipgloss.Height(rendered) != height {
					t.Errorf("edit location missing or overflowing for %s at %dx%d: %s", item.task, size[0], size[1], ansi.Strip(rendered))
				}
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			m = updated.(*model)
		})
	}
}

func TestIndexTaskActionsAndPriorityPalette(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("## General\n\n- [ ] First !high\n\n- [ ] Second !low\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.openAllTasks()
	m.all.sort = "priority"
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); !ok || selected.Text != "First" || !selected.Done {
		t.Fatal("Space did not complete the selected indexed task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = updated.(*model)
	if !isOpen[*taskModal](m) || form(t, m).title.Value() != "First" {
		t.Fatal("Edit did not open the indexed task")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(*model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*model)
	if selected, ok := m.selectedTask(); !ok || selected.Text != "First" || selected.Priority != "medium" || m.all.sort != "priority" {
		t.Fatal("p should change the selected task's priority without changing sort")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(*model)
	if !isOpen[*categoryPrompt](m) || m.all.sort != "priority" {
		t.Fatal("c should edit the selected task's category without changing sort")
	}
	for p, want := range map[store.Priority]color.Color{"high": m.theme.ColorHigh, "medium": m.theme.ColorMedium, "low": m.theme.ColorLow} {
		t.Run(string(p), func(t *testing.T) {
			if got := priorityStyle(m.theme, p).GetForeground(); got != want {
				t.Errorf("%s priority color = %v, want %v", p, got, want)
			}
		})
	}
}
