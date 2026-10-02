package filesui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/ui"
)

func scanned(matches ...scan.Match) Model {
	var m Model
	m.Update(ScannedMsg{Matches: matches})
	return m
}

func press(m *Model, key string) tea.Cmd {
	msg := tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	switch key {
	case "right":
		msg = tea.KeyPressMsg{Code: tea.KeyRight}
	case "left":
		msg = tea.KeyPressMsg{Code: tea.KeyLeft}
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEsc}
	}
	return m.Update(msg)
}

func TestCategoriesOpenLikeGeneral(t *testing.T) {
	m := scanned(
		scan.Match{Path: "a.go", Line: 1, Note: "urgent", Level: "0"},
		scan.Match{Path: "b.css", Line: 2, Note: "hide", Category: "Boundary"},
		scan.Match{Path: "c.css", Line: 3, Note: "show", Category: "boundary"},
	)
	if rows := m.rows(); len(rows) != 2 || rows[0].category != "Boundary" || rows[0].count != 2 {
		t.Fatalf("root rows = %+v", rows)
	}
	if cmd := press(&m, "enter"); m.category != "Boundary" || len(m.rows()) != 2 || cmd == nil {
		t.Fatalf("enter did not open the category with a preview: %q, %+v", m.category, m.rows())
	}
	if !slices.Equal(m.breadcrumb(), []string{"Files", "@Boundary"}) {
		t.Fatalf("breadcrumb = %v", m.breadcrumb())
	}
	if selected, ok := m.Selected(); !ok || selected.Note != "hide" {
		t.Fatalf("selected %+v", selected)
	}
	m.Top()
	if m.category != "" || m.cursor != 0 {
		t.Fatalf("Top: category %q, cursor %d", m.category, m.cursor)
	}
	press(&m, "enter")
	press(&m, "esc")
	if m.category != "" {
		t.Fatal("esc did not leave the category")
	}
	press(&m, "enter")
	m.Update(ScannedMsg{Matches: []scan.Match{{Path: "a.go", Line: 1, Note: "urgent", Level: "0"}}})
	if m.category != "" || m.cursor != 0 {
		t.Fatalf("a category gone after a rescan stayed open: %q", m.category)
	}
	if selected, ok := m.Selected(); !ok || selected.Note != "urgent" {
		t.Fatalf("selected %+v after rescan", selected)
	}
}

func TestDetailPageOpensAndCloses(t *testing.T) {
	m := scanned(
		scan.Match{Path: "a.go", Line: 1, Note: "first"},
		scan.Match{Path: "b.go", Line: 2, Note: "second", Category: "ui"},
	)
	press(&m, "right")
	press(&m, "j")
	press(&m, "right")
	if !m.Details() || !slices.Equal(m.breadcrumb(), []string{"Files", "@ui", "Details"}) {
		t.Fatalf("right did not open details: %v", m.breadcrumb())
	}
	if hints := ansi.Strip(ui.RenderHints(m.Hints())); hints != "← back  e open file" {
		t.Fatalf("detail hints = %q", hints)
	}
	press(&m, "down")
	press(&m, "down")
	if m.scroll != 2 {
		t.Fatalf("down did not scroll the details: %d", m.scroll)
	}
	m.Top()
	if m.Details() || m.category != "ui" || m.scroll != 0 {
		t.Fatalf("Top from details should return to the category's list: details %v, category %q", m.Details(), m.category)
	}
	press(&m, "right")
	if cmd := press(&m, "left"); m.Details() || cmd == nil {
		t.Fatal("left did not return to the list with a preview")
	}
	if hints := ansi.Strip(ui.RenderHints(m.Hints())); hints != "e open file  → details  ← back" {
		t.Fatalf("list hints = %q", hints)
	}
	m.Top()
	if hints := ansi.Strip(ui.RenderHints(m.Hints())); hints != "→ open" {
		t.Fatalf("category row hints = %q", hints)
	}
}

func TestEmptyListOpensNothing(t *testing.T) {
	m := New("/nowhere", scan.Exclude{}, nil)
	if !m.Loading() || !strings.Contains(ansi.Strip(m.View(60, 10)), "Scanning…") {
		t.Fatal("a list before its first scan should say it's scanning")
	}
	if m.Hints() != nil {
		t.Fatalf("hints while scanning = %+v", m.Hints())
	}
	m.Update(ScannedMsg{})
	if m.Hints() != nil {
		t.Fatalf("hints for an empty list = %+v", m.Hints())
	}
	if press(&m, "right"); m.Details() || m.Loading() || m.Total() != 0 {
		t.Fatal("right opened details without a TODO")
	}
	if cmd := press(&m, "e"); cmd != nil {
		t.Fatal("e opened an editor without a TODO")
	}
	m.Update(ScannedMsg{Err: fmt.Errorf("permission denied")})
	if m.Err() != "permission denied" || !strings.Contains(ansi.Strip(m.View(60, 10)), "Scan failed") {
		t.Fatalf("scan error not shown: %q", m.Err())
	}
}

func TestRescanReplacesARunningScan(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("// TODO: found\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(dir, scan.Exclude{}, nil)
	first := m.Scan()
	second := m.Scan()
	// The rescan cancelled the first scan, which then reports that error.
	if m.Update(first()); !m.Loading() || m.Err() != "" {
		t.Fatalf("the replaced scan was used: loading %v, error %q", m.Loading(), m.Err())
	}
	if m.Update(second()); m.Loading() || m.Err() != "" || m.Total() != 1 {
		t.Fatalf("the rescan wasn't used: loading %v, error %q, %d TODOs", m.Loading(), m.Err(), m.Total())
	}
	// Arriving after the rescan's results, it still mustn't replace them.
	if m.Update(first()); m.Err() != "" || m.Total() != 1 {
		t.Fatalf("the replaced scan overwrote the rescan: error %q, %d TODOs", m.Err(), m.Total())
	}
}

func TestScanErrorVisibleInDetails(t *testing.T) {
	m := Model{err: "permission denied"}
	got := strings.Join(m.detailLines(60, 20), "\n")
	if !strings.Contains(got, "permission denied") {
		t.Fatalf("scan error missing from detail pane: %s", got)
	}
}

func TestLongZeroLevelsAreShortened(t *testing.T) {
	m := scanned(scan.Match{Path: "a.go", Line: 1, Note: "urgent", Level: "000000"}, scan.Match{Path: "b.go", Line: 1, Note: "next", Level: "1"})
	pane := ansi.Strip(m.View(80, 8))
	if !strings.Contains(pane, "0x6 urgent") || !strings.Contains(pane, "1   next") {
		t.Fatalf("long level was not shortened:\n%s", pane)
	}
}

func TestListShowsTheTodoBeforeItsFile(t *testing.T) {
	long := "ebuyer_site/private/common/classes/Blocks/BlockPage.class.php"
	m := scanned(
		scan.Match{Path: "retry.go", Line: 8, Note: "solve first", Level: "000"},
		scan.Match{Path: "retry.go", Line: 9, Note: "then this", Level: "1"},
		scan.Match{Path: long, Line: 30, Note: "Remove these once the new layout ships"},
		scan.Match{Path: "mobile.css", Line: 3, Note: "Hide on mobile", Category: "responsive"},
	)
	pane := ansi.Strip(m.View(100, 12))
	rows := strings.Split(pane, "\n")
	for i, want := range []string{
		"▸ responsive",
		"",
		"000 solve first",
		"1   then this",
		"·   Remove these once the new layout ships",
	} {
		if got := strings.Trim(rows[i+1], "│ "); !strings.HasPrefix(got, want) {
			t.Fatalf("row %d = %q, want prefix %q\n%s", i, got, want, pane)
		}
	}
	if !strings.Contains(rows[5], "…/Blocks/BlockPage.class.php:30") || !strings.HasSuffix(strings.TrimRight(rows[1], "│ "), " 1") {
		t.Fatalf("file or count column missing:\n%s", pane)
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != 100 {
			t.Fatalf("row is %d wide: %q", ansi.StringWidth(row), row)
		}
	}
	if status := ansi.Strip(m.status()); status != "1 TODO" {
		t.Fatalf("category status = %q", status)
	}
	styled := m.View(100, 12)
	if !strings.Contains(styled, lipgloss.NewStyle().Foreground(ui.ColorHigh).Render("000 ")) || !strings.Contains(styled, lipgloss.NewStyle().Foreground(ui.ColorMedium).Render("1   ")) {
		t.Error("zero levels are not red, or numbered levels not yellow")
	}
	m.cursor = 3
	if status := ansi.Strip(m.status()); status != long+":30" {
		t.Fatalf("status = %q", status)
	}
	m.category = "responsive"
	if row := strings.Split(ansi.Strip(m.View(100, 12)), "\n")[3]; !strings.HasPrefix(strings.Trim(row, "│ "), "Hide on mobile") {
		t.Fatalf("a list without levels kept the level column: %q", row)
	}
	for _, c := range []struct {
		width int
		want  string
	}{{80, long}, {28, "…/Blocks/BlockPage.class.php"}, {27, "…/BlockPage.class.php"}, {10, "…class.php"}} {
		if got := truncatePath(long, c.width); got != c.want {
			t.Errorf("truncatePath(%d) = %q, want %q", c.width, got, c.want)
		}
	}
}

func TestDetailsFillThePageAroundTheTodo(t *testing.T) {
	var preview []scan.ContextLine
	for n := 1; n <= 60; n++ {
		preview = append(preview, scan.ContextLine{Number: n, Text: fmt.Sprintf("\tline %d", n)})
	}
	preview[29].Text = "// todo00000000000 this is urgent " + strings.Repeat("x", 80)
	item := scan.Match{Path: "classes copy/Clients.class.php", Line: 30, Note: "this is urgent", Level: "00000000000"}
	m := scanned(item)
	m.Update(previewMsg{path: item.Path, line: item.Line, lines: preview})
	lines := m.detailLines(60, 12)
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = ansi.Strip(line)
		if ansi.StringWidth(line) > 60 {
			t.Fatalf("line %d is wider than the page: %q", i, plain[i])
		}
	}
	if plain[0] != "0x9+ this is urgent" || plain[1] != "" || len(lines) != 12 {
		t.Fatalf("details = %q", plain)
	}
	if got := strings.Join(plain, "\n"); strings.Contains(got, item.Path) {
		t.Fatalf("details repeat the path from the status bar:\n%s", got)
	}
	if plain[2] != "  26 │     line 26" || !strings.HasPrefix(plain[6], "  30 │ // todo") || !strings.HasSuffix(strings.TrimRight(plain[6], " "), "…") || plain[11] != "  35 │     line 35" {
		t.Fatalf("context is not centred on the TODO, or wrapped:\n%s", strings.Join(plain, "\n"))
	}
	m.matches[0].Line, m.previewLine, preview[1].Text = 2, 2, "// TODO near the top"
	if lines := m.detailLines(60, 12); !strings.HasPrefix(ansi.Strip(lines[2]), "   1 │") {
		t.Fatalf("a TODO near the top of the file did not start at line 1: %q", ansi.Strip(lines[2]))
	}
}

func TestScanKeepsOnlyWhatTheFilterAccepts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("// TODO: plain\n// todo0 urgent\n// todo1 later\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		keep func(scan.Match) bool
		want int
	}{
		{nil, 3},
		{scan.LevelFilter(true, nil), 2},
		{scan.LevelFilter(false, []string{"0"}), 1},
	} {
		m := New(dir, scan.Exclude{}, c.keep)
		m.Update(m.Scan()())
		if m.Err() != "" || m.Total() != c.want {
			t.Errorf("scan kept %d (%s), want %d", m.Total(), m.Err(), c.want)
		}
	}
}
