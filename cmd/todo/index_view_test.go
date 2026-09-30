package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

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
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
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
		})
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
		t.Run(item.task, func(t *testing.T) {
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
		})
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
		t.Run(string(priority), func(t *testing.T) {
			if got := priorityStyle(priority).GetForeground(); got != want {
				t.Errorf("%s priority color = %v, want %v", priority, got, want)
			}
		})
	}
}
