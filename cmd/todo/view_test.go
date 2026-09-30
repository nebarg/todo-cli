package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
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
		})
	}
}

func TestInactiveTabsShareTheBarBackground(t *testing.T) {
	m := &model{focus: sourcePane}
	tabs := m.renderTabs(80)
	inactive := lipgloss.NewStyle().Foreground(colorMuted).Background(colorBar)
	for _, tab := range []string{" 1 General ", " 2 Branches "} {
		if !strings.Contains(tabs, inactive.Render(tab)) {
			t.Errorf("inactive tab %q has the wrong background: %q", tab, tabs)
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
	if taskTitleStyle.GetForeground() != colorStrong {
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
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
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
		})
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
		t.Run(string(priority), func(t *testing.T) {
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
		})
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
	m.generalCategory = "Docs"
	if view := ansi.Strip(m.renderNavigationPane(m.generalTitle(), m.generalRows(), 0, generalPane, 60, 20)); !strings.Contains(view, "General · @Docs  1/2") {
		t.Fatalf("category count = %s", view)
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

func TestFooterOmitsObviousMovementHints(t *testing.T) {
	m := &model{}
	for _, pane := range []pane{generalPane, branchPane, sourcePane, detailPane} {
		t.Run(fmt.Sprint("pane ", pane), func(t *testing.T) {
			m.focus = pane
			if footer := ansi.Strip(m.renderFooter(120)); strings.Contains(footer, "↑/↓") || strings.Contains(footer, "move") || strings.Contains(footer, "scroll") {
				t.Errorf("movement hint remains on pane %d: %q", pane, footer)
			}
		})
	}
	m.indexMode = true
	if footer := ansi.Strip(m.renderFooter(120)); strings.Contains(footer, "↑/↓") || strings.Contains(footer, "move") {
		t.Errorf("movement hint remains in All tasks: %q", footer)
	}
}
