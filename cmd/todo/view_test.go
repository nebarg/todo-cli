package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
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
	m.source = []scan.Match{{Path: "main.go", Line: 12, Text: "// TODO: improve"}}
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
				{"details", detailPane, "Open  ·  No priority", "main.go:12"},
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
				if len(lines) < 4 || !strings.HasPrefix(lines[0], " 1 General") || (size[0] >= 80 && !strings.HasSuffix(lines[0], gitIcon+" feature/login  ")) || !strings.HasPrefix(lines[1], "╭") {
					t.Errorf("%s header is not one bar of tabs and branch at %dx%d: %q", page.name, size[0], size[1], lines[:2])
				}
				if page.focus == detailPane {
					if len(lines) < 5 || strings.Trim(lines[3], " │") != "" {
						t.Errorf("%s has no gap below its heading at %dx%d", page.name, size[0], size[1])
					}
				} else if len(lines) < 3 || strings.Trim(lines[2], " │") == "" {
					t.Errorf("%s wastes a line above its list at %dx%d", page.name, size[0], size[1])
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
	tabs := m.renderHeader(80)
	inactive := lipgloss.NewStyle().Foreground(colorMuted).Background(colorBar)
	for _, tab := range []string{" General ", " Branches "} {
		if !strings.Contains(tabs, inactive.Render(tab)) {
			t.Errorf("inactive tab %q has the wrong background: %q", tab, tabs)
		}
	}
	if !strings.Contains(tabs, keyStyle.Background(colorBar).Render("1")) || !strings.Contains(tabs, keyStyle.Background(colorSelection).Render("3")) {
		t.Errorf("tab numbers are not styled as keys: %q", tabs)
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
	m := &model{general: []store.Task{{Text: "Fix login redirect", Details: "When a session expires, return to the previous page.\n\n- Add a regression test"}}}
	got := strings.Join(m.taskDetails(60), "\n")
	for _, want := range []string{"Fix login redirect", "When a session expires", "- Add a regression test"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in task details: %s", want, got)
		}
	}
	for _, gone := range []string{"Status", "Scope", "Priority", "Category"} {
		if strings.Contains(got, gone) {
			t.Errorf("task details still list %q, which belongs in the status bar: %s", gone, got)
		}
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
	if footer := ansi.Strip(m.renderFooter(120)); !strings.Contains(footer, "← back") || !strings.Contains(footer, "d done  e edit  p priority  c category") {
		t.Errorf("detail footer does not show navigation and edit shortcuts: %q", footer)
	}
	if footer := ansi.Strip(m.renderFooter(56)); !strings.Contains(footer, "d done  e edit  p priority  c category") || !strings.HasSuffix(footer, "? help  q quit") {
		t.Errorf("narrow detail footer lost task shortcuts: %q", footer)
	}
}

func TestTaskRowsColourPriorityOnTheBullet(t *testing.T) {
	for p, want := range map[store.Priority]struct{ foreground, mark string }{
		"high": {"240;119;119", "●"}, "medium": {"244;211;94", "●"}, "low": {"125;207;223", "●"},
	} {
		foreground, mark := want.foreground, want.mark
		t.Run(string(p), func(t *testing.T) {
			task := store.Task{Text: "Highlighted title", Priority: p, Details: "Extra context"}
			selected := renderTaskRow(task, 30, true)
			if ansi.StringWidth(selected) != 30 || !strings.HasPrefix(ansi.Strip(selected), mark+" Highlighted title") || !strings.HasSuffix(ansi.Strip(selected), "⋯") {
				t.Fatalf("%s selected task is not full width with a right-aligned details marker: %q", p, ansi.Strip(selected))
			}
			if !strings.Contains(selected, "38;2;"+foreground+";48;2;36;87;166m"+mark) || strings.Contains(selected, foreground+";48;2;36;87;166mHighlighted") {
				t.Fatalf("%s selected task should colour only its bullet: %q", p, selected)
			}
			unselected := renderTaskRow(task, 30, false)
			if ansi.StringWidth(unselected) != 30 || !strings.HasSuffix(ansi.Strip(unselected), "⋯") {
				t.Fatalf("%s details marker is not right-aligned: %q", p, ansi.Strip(unselected))
			}
			if !strings.Contains(unselected, "38;2;"+foreground+"m"+mark) || strings.Contains(unselected, foreground+"mHighlighted") {
				t.Fatalf("%s unselected task should colour only its bullet: %q", p, unselected)
			}
			done := renderTaskRow(store.Task{Text: "Highlighted title", Priority: p, Done: true}, 30, false)
			if ansi.Strip(done) != "✓ Highlighted title" || strings.Contains(done, "38;2;"+foreground) {
				t.Fatalf("%s done task is not muted: %q", p, done)
			}
		})
	}
	open := ansi.Strip(renderTaskRow(store.Task{Text: "Same title"}, 24, false))
	done := ansi.Strip(renderTaskRow(store.Task{Text: "Same title", Done: true}, 24, false))
	if strings.Index(open, "Same title") != strings.Index(done, "Same title") || strings.Contains(open, "⋯") || strings.Contains(done, "⋯") {
		t.Fatalf("completion changed title alignment or added details marker: %q / %q", open, done)
	}
	truncated := ansi.Strip(renderTaskRow(store.Task{Text: strings.Repeat("x", 50), Details: "More"}, 24, false))
	if ansi.StringWidth(truncated) != 24 || !strings.HasSuffix(truncated, "…  ⋯") {
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
	m.sourceLoading = false
	if header := ansi.Strip(m.renderHeader(100)); !strings.HasSuffix(header, " "+gitIcon+" main  ") || strings.Contains(header, "To Do") || strings.Contains(header, ".") {
		t.Fatalf("header without a repository root = %q", header)
	}
	m.project.branch = ""
	if header := ansi.Strip(m.renderHeader(100)); ansi.StringWidth(header) != 100 || !strings.HasSuffix(strings.TrimRight(header, " "), "3 Files 0") || strings.Contains(header, gitIcon) {
		t.Fatalf("header outside Git = %q", header)
	}
	m.project.branch = "main"
	if tabs := ansi.Strip(m.renderHeader(100)); !strings.Contains(tabs, " 1 General 2/4 ") || !strings.Contains(tabs, " 2 Branches 1/2 ") || !strings.Contains(tabs, " 3 Files 0 ") {
		t.Fatalf("tab counts = %q", tabs)
	}
	general := ansi.Strip(m.renderNavigationPane(m.generalRows(), 0, generalPane, 60, 20))
	if strings.Contains(general, "General") || !regexp.MustCompile(`▸ Docs +1/2 │`).MatchString(general) {
		t.Fatalf("general list = %s", general)
	}
	branches := ansi.Strip(m.renderNavigationPane(m.branchRows(), 0, branchPane, 60, 20))
	if strings.Contains(branches, "Branches") || !regexp.MustCompile(`▸ main  current +1/2 │`).MatchString(branches) {
		t.Fatalf("branch list = %s", branches)
	}
	m.generalCategory = "Docs"
	if view := ansi.Strip(m.renderNavigationPane(m.generalRows(), 0, generalPane, 60, 20)); !strings.Contains(view, "General › @Docs  1/2") {
		t.Fatalf("category breadcrumb = %s", view)
	}
	m.branchFilter = "main"
	if branch := ansi.Strip(m.renderNavigationPane(m.branchRows(), 0, branchPane, 60, 20)); !strings.Contains(branch, "Branches › "+branchIcon+" main  1/2") {
		t.Fatalf("branch breadcrumb = %s", branch)
	}
	m.focus, m.detailFrom = detailPane, branchPane
	if detail := ansi.Strip(m.renderDetailPane(60, 20)); !strings.Contains(detail, "Branches › "+branchIcon+" main › Details") {
		t.Fatalf("detail breadcrumb = %s", detail)
	}
	m.indexMode = true
	if index := ansi.Strip(m.renderIndex(100, 20)); !regexp.MustCompile(`All tasks  3/6 +sorted by`).MatchString(index) {
		t.Fatalf("all tasks heading = %s", index)
	}
}

func TestHeaderShowsRepositoryAndBranch(t *testing.T) {
	m := &model{project: projectContext{root: "/src/todo-cli", branch: "feature/login"}}
	header := m.renderHeader(80)
	if plain := ansi.Strip(header); ansi.StringWidth(header) != 80 || !strings.HasPrefix(plain, " 1 General") || !strings.HasSuffix(plain, "todo-cli  "+gitIcon+" feature/login  ") {
		t.Fatalf("header = %q", plain)
	}
	if !strings.Contains(header, lipgloss.NewStyle().Foreground(colorGit).Background(colorBar).Render(gitIcon+" ")) {
		t.Fatalf("git icon is not coloured: %q", header)
	}
	m.project.branch = strings.Repeat("b", 100)
	header = m.renderHeader(60)
	if plain := ansi.Strip(header); ansi.StringWidth(header) != 60 || !strings.Contains(plain, "3 Files") || strings.Contains(plain, "todo-cli") || !strings.HasSuffix(plain, "…  ") {
		t.Fatalf("long branch should drop the repo and shorten before the tabs: %q", plain)
	}
	m.indexMode = true
	if plain := ansi.Strip(m.renderHeader(60)); strings.Contains(plain, "General") || !strings.Contains(plain, "todo-cli") {
		t.Fatalf("All tasks header = %q", plain)
	}
}

func TestLooseTasksAreSeparatedFromCategories(t *testing.T) {
	m := &model{general: []store.Task{{Text: "Loose"}, {Text: "Filed", Category: "Docs"}}}
	rows := m.generalRows()
	if got := firstTaskAfterGroups(rows); got != 1 {
		t.Fatalf("divider row = %d", got)
	}
	lines := strings.Split(ansi.Strip(m.renderNavigationPane(rows, 0, generalPane, 40, 10)), "\n")
	if !strings.Contains(lines[1], "▸ Docs") || strings.Trim(lines[2], " │") != "" || !strings.Contains(lines[3], "○ Loose") {
		t.Fatalf("loose tasks are not separated from categories: %q", lines)
	}
	if got := firstTaskAfterGroups(m.branchRows()); got != -1 {
		t.Fatalf("unmixed list has a divider at %d", got)
	}
}

func TestFooterFitsHintsAndPinsHelp(t *testing.T) {
	m := &model{general: []store.Task{{Text: "Loose"}}}
	for _, width := range []int{56, 80, 160} {
		footer := m.renderFooter(width)
		plain := ansi.Strip(footer)
		if ansi.StringWidth(footer) != width || !strings.HasSuffix(plain, "? help  q quit") || !strings.HasPrefix(plain, "d done  e edit") {
			t.Errorf("footer at %d = %q", width, plain)
		}
		if strings.Contains(plain, "…") || strings.Contains(plain, "·") {
			t.Errorf("footer at %d truncated a hint or used a separator: %q", width, plain)
		}
	}
	if wide := ansi.Strip(m.renderFooter(160)); !strings.Contains(wide, "r reload") {
		t.Errorf("wide footer dropped hints: %q", wide)
	}
	if narrow := ansi.Strip(m.renderFooter(56)); strings.Contains(narrow, "r reload") {
		t.Errorf("narrow footer kept low-priority hints: %q", narrow)
	}
	m.status = "Saved"
	if footer := ansi.Strip(m.renderFooter(80)); !strings.HasPrefix(footer, "Saved   d done") || !strings.HasSuffix(footer, "q quit") {
		t.Errorf("status footer = %q", footer)
	}
	m.general = []store.Task{{Text: "Filed", Category: "Docs"}}
	m.status = ""
	if footer := ansi.Strip(m.renderFooter(120)); !strings.HasPrefix(footer, "→ open") || strings.Contains(footer, "d done") {
		t.Errorf("category row footer offers task actions: %q", footer)
	}
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	m := &model{width: 56, height: 16}
	updated, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = updated.(*model)
	view := m.View().Content
	if !m.helpOpen || !strings.Contains(ansi.Strip(view), "toggle done") || lipgloss.Width(view) != 56 || lipgloss.Height(view) != 16 {
		t.Fatalf("help overlay did not open within the terminal: %s", ansi.Strip(view))
	}
	if got := lipgloss.Height(renderHelp()); got > 16 {
		t.Fatalf("help is %d rows, taller than the smallest supported terminal", got)
	}
	for line := range strings.SplitSeq(renderHelp(), "\n") {
		if strings.Contains(line, "\x1b[m ") {
			t.Fatalf("help lets the terminal background through after a reset: %q", line)
		}
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = updated.(*model)
	if m.helpOpen || cmd != nil {
		t.Fatal("a key press while help is open should only close it")
	}
	m.indexMode = true
	updated, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !updated.(*model).helpOpen {
		t.Fatal("help did not open from All tasks")
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

func TestPriorityReadsFromShapeAsWellAsColour(t *testing.T) {
	marks := map[store.Priority]string{store.PriorityHigh: "●", store.PriorityMedium: "●", store.PriorityLow: "●", store.PriorityNone: "○"}
	for p, want := range marks {
		if got := priorityMark(p); got != want {
			t.Errorf("priorityMark(%q) = %q, want %q", p, got, want)
		}
	}
	if status := ansi.Strip(taskStatus(store.Task{Text: "Urgent", Priority: store.PriorityHigh})); status != "Open  ·  ● High priority" {
		t.Fatalf("status bar = %q", status)
	}
	if help := ansi.Strip(renderHelp()); !regexp.MustCompile(`quit[ │]*\n[│ ]*\n[│ ]*Priority  ● high  ● medium  ● low`).MatchString(help) {
		t.Fatalf("help lacks the priority legend: %s", help)
	}
}

func TestStatusBarDescribesTheHighlightedRow(t *testing.T) {
	m := &model{
		general: []store.Task{{Text: "Loose", Priority: store.PriorityMedium}, {Text: "Filed", Category: "Docs", Done: true}, {Text: "Open filed", Category: "Docs"}},
		source:  []scan.Match{{Path: "main.go", Line: 1, Text: "// TODO"}},
		width:   80, height: 20,
	}
	bottom := func() string {
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		return strings.Trim(lines[len(lines)-3], " │")
	}
	if got := bottom(); got != "1 of 2 done" {
		t.Fatalf("category row status = %q", got)
	}
	m.generalCursor = 1
	if got := bottom(); got != "Open  ·  ● Medium priority" {
		t.Fatalf("task row status = %q", got)
	}
	m.focus, m.detailFrom = detailPane, generalPane
	if got := bottom(); got != "Open  ·  ● Medium priority" {
		t.Fatalf("detail page status = %q", got)
	}
	m.focus, m.generalCategory, m.generalCursor = generalPane, "Docs", 1
	if got := bottom(); got != "✓ Done  ·  No priority" {
		t.Fatalf("done task status = %q", got)
	}
	m.focus = sourcePane
	if got := bottom(); strings.Contains(got, "priority") || strings.Contains(got, "done") {
		t.Fatalf("file TODOs have no status, but the bar showed %q", got)
	}
}
