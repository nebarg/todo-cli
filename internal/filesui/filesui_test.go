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
	theme := ui.NewTheme(true)
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
	if hints := ansi.Strip(theme.RenderHints(m.Hints())); hints != "← back  e open file" {
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
	if hints := ansi.Strip(theme.RenderHints(m.Hints())); hints != "e open file  → details  ← back" {
		t.Fatalf("list hints = %q", hints)
	}
	m.Top()
	if hints := ansi.Strip(theme.RenderHints(m.Hints())); hints != "→ open" {
		t.Fatalf("category row hints = %q", hints)
	}
}

func TestEmptyListOpensNothing(t *testing.T) {
	theme := ui.NewTheme(true)
	m := New("/nowhere", scan.Exclude{}, nil)
	m.SetTheme(theme)
	m.SetSize(60, 10)
	if !m.Loading() || !strings.Contains(ansi.Strip(m.View()), "Scanning…") {
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
	if m.Err() == nil || m.Err().Error() != "permission denied" || !strings.Contains(ansi.Strip(m.View()), "Scan failed") {
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
	if m.Update(first()); !m.Loading() || m.Err() != nil {
		t.Fatalf("the replaced scan was used: loading %v, error %q", m.Loading(), m.Err())
	}
	if m.Update(second()); m.Loading() || m.Err() != nil || m.Total() != 1 {
		t.Fatalf("the rescan wasn't used: loading %v, error %q, %d TODOs", m.Loading(), m.Err(), m.Total())
	}
	// Arriving after the rescan's results, it still mustn't replace them.
	if m.Update(first()); m.Err() != nil || m.Total() != 1 {
		t.Fatalf("the replaced scan overwrote the rescan: error %q, %d TODOs", m.Err(), m.Total())
	}
}

func TestScanErrorVisibleInDetails(t *testing.T) {
	theme := ui.NewTheme(true)
	m := Model{err: fmt.Errorf("permission denied")}
	got := strings.Join(m.detailLines(theme, 60, 20), "\n")
	if !strings.Contains(got, "permission denied") {
		t.Fatalf("scan error missing from detail pane: %s", got)
	}
}

func TestLongZeroLevelsAreShortened(t *testing.T) {
	theme := ui.NewTheme(true)
	m := scanned(scan.Match{Path: "a.go", Line: 1, Note: "urgent", Level: "000000"}, scan.Match{Path: "b.go", Line: 1, Note: "next", Level: "1"})
	m.SetTheme(theme)
	m.SetSize(80, 8)
	pane := ansi.Strip(m.View())
	if !strings.Contains(pane, "0x6 urgent") || !strings.Contains(pane, "1   next") {
		t.Fatalf("long level was not shortened:\n%s", pane)
	}
}

func TestListShowsTheTodoBeforeItsFile(t *testing.T) {
	theme := ui.NewTheme(true)
	long := "ebuyer_site/private/common/classes/Blocks/BlockPage.class.php"
	m := scanned(
		scan.Match{Path: "retry.go", Line: 8, Note: "solve first", Level: "000"},
		scan.Match{Path: "retry.go", Line: 9, Note: "then this", Level: "1"},
		scan.Match{Path: long, Line: 30, Note: "Remove these once the new layout ships"},
		scan.Match{Path: "mobile.css", Line: 3, Note: "Hide on mobile", Category: "responsive"},
	)
	m.SetTheme(theme)
	m.SetSize(100, 12)
	pane := ansi.Strip(m.View())
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
	if status := ansi.Strip(m.status(theme)); status != "1 TODO" {
		t.Fatalf("category status = %q", status)
	}
	styled := m.View()
	if !strings.Contains(styled, lipgloss.NewStyle().Foreground(theme.ColorHigh).Render("000 ")) || !strings.Contains(styled, lipgloss.NewStyle().Foreground(theme.ColorMedium).Render("1   ")) {
		t.Error("zero levels are not red, or numbered levels not yellow")
	}
	m.cursor = 3
	if status := ansi.Strip(m.status(theme)); status != long+":30" {
		t.Fatalf("status = %q", status)
	}
	m.category = "responsive"
	if row := strings.Split(ansi.Strip(m.View()), "\n")[3]; !strings.HasPrefix(strings.Trim(row, "│ "), "Hide on mobile") {
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
	theme := ui.NewTheme(true)
	var preview []scan.ContextLine
	for n := 1; n <= 60; n++ {
		preview = append(preview, scan.ContextLine{Number: n, Text: fmt.Sprintf("\tline %d", n)})
	}
	preview[29].Text = "// todo00000000000 this is urgent " + strings.Repeat("x", 80)
	item := scan.Match{Path: "classes copy/Clients.class.php", Line: 30, Note: "this is urgent", Level: "00000000000"}
	m := scanned(item)
	m.Update(previewMsg{path: item.Path, line: item.Line, lines: preview})
	lines := m.detailLines(theme, 60, 12)
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
	if lines := m.detailLines(theme, 60, 12); !strings.HasPrefix(ansi.Strip(lines[2]), "   1 │") {
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
		if m.Err() != nil || m.Total() != c.want {
			t.Errorf("scan kept %d (%s), want %d", m.Total(), m.Err(), c.want)
		}
	}
}

func TestUpAndDownWrapPastTheEnds(t *testing.T) {
	m := scanned(
		scan.Match{Path: "a.go", Line: 1, Note: "first"},
		scan.Match{Path: "a.go", Line: 2, Note: "second"},
		scan.Match{Path: "a.go", Line: 3, Note: "third"},
	)
	for _, step := range []struct {
		key  string
		want string
	}{
		{"k", "third"}, // up from the top wraps to the end
		{"j", "first"}, // and down from the end to the top
		{"j", "second"},
	} {
		press(&m, step.key)
		if selected, ok := m.Selected(); !ok || selected.Note != step.want {
			t.Fatalf("after %s, selected %q, want %q", step.key, selected.Note, step.want)
		}
	}
}

// detailsModel is a browser in a panel of width × height, showing the details
// of a TODO with note and the code around it.
func detailsModel(t *testing.T, note string, width, height int) Model {
	t.Helper()
	item := scan.Match{Path: "a.go", Line: 30, Note: note}
	m := scanned(item)
	m.SetTheme(ui.NewTheme(true))
	m.SetSize(width, height)
	var preview []scan.ContextLine
	for n := 1; n <= 60; n++ {
		preview = append(preview, scan.ContextLine{Number: n, Text: fmt.Sprintf("line %d", n)})
	}
	m.Update(previewMsg{path: item.Path, line: item.Line, lines: preview})
	if press(&m, "right"); !m.Details() {
		t.Fatal("right did not open the details")
	}
	return m
}

func TestDetailsStopScrollingAtTheirLastLine(t *testing.T) {
	m := detailsModel(t, strings.Repeat("word ", 150), 40, 16)
	bottom := m.detailsBottom()
	if bottom == 0 {
		t.Fatal("the details fit, so can't scroll")
	}
	for range bottom + 20 {
		press(&m, "j")
	}
	if m.scroll != bottom {
		t.Fatalf("holding down scrolled to %d, past the bottom at %d", m.scroll, bottom)
	}
	atBottom := ansi.Strip(m.View())
	if !strings.Contains(atBottom, "30 │ line 30") {
		t.Fatalf("the last line isn't showing at the bottom:\n%s", atBottom)
	}
	if press(&m, "k"); m.scroll != bottom-1 || ansi.Strip(m.View()) == atBottom {
		t.Fatalf("one up from the bottom didn't scroll up: scroll %d, bottom %d", m.scroll, bottom)
	}
	for range bottom + 20 {
		press(&m, "k")
	}
	if m.scroll != 0 {
		t.Fatalf("holding up scrolled to %d, not the top", m.scroll)
	}
}

func TestDetailsThatFitDoNotScroll(t *testing.T) {
	m := detailsModel(t, "short", 40, 16)
	if bottom := m.detailsBottom(); bottom != 0 {
		t.Fatalf("the details don't fit: they scroll to %d", bottom)
	}
	if press(&m, "j"); m.scroll != 0 {
		t.Fatalf("down on details that fit scrolled to %d", m.scroll)
	}
}

func TestDetailsScrollUpStraightAwayAfterThePanelGrows(t *testing.T) {
	m := detailsModel(t, strings.Repeat("word ", 150), 40, 14)
	for range m.detailsBottom() {
		press(&m, "j")
	}
	m.SetSize(40, 24)
	bottom := m.detailsBottom()
	if bottom == 0 || bottom >= m.scroll {
		t.Fatalf("growing the panel didn't move the bottom (%d) above the scroll (%d)", bottom, m.scroll)
	}
	before := ansi.Strip(m.View())
	if press(&m, "k"); m.scroll != bottom-1 || ansi.Strip(m.View()) == before {
		t.Fatalf("one up after growing didn't scroll up: scroll %d, bottom %d", m.scroll, bottom)
	}
}

func TestPagesFillThePanelSetBySetSize(t *testing.T) {
	m := scanned(scan.Match{Path: "a.go", Line: 1, Note: "urgent", Level: "0"})
	m.SetTheme(ui.NewTheme(true))
	for _, size := range [][2]int{{80, 20}, {56, 12}, {100, 30}} {
		m.SetSize(size[0], size[1])
		for _, page := range []string{"list", "details"} {
			if page == "details" {
				press(&m, "right")
			}
			view := m.View()
			if w, h := lipgloss.Width(view), lipgloss.Height(view); w != size[0] || h != size[1] {
				t.Errorf("%s at %dx%d is %dx%d", page, size[0], size[1], w, h)
			}
		}
		press(&m, "left")
	}
}

func TestPagesAreDrawnInTheThemeSetBySetTheme(t *testing.T) {
	m := scanned(scan.Match{Path: "a.go", Line: 1, Note: "urgent", Level: "0"})
	m.SetSize(40, 8)
	dark, light := ui.NewTheme(true), ui.NewTheme(false)
	// Only the panel's border is drawn in the border colour.
	border := func(theme ui.Theme) string {
		return ansi.NewStyle().ForegroundColor(theme.ColorBorder).String() + "╭"
	}
	for _, page := range []string{"list", "details"} {
		if page == "details" {
			press(&m, "right")
		}
		for _, c := range []struct{ drawn, other ui.Theme }{{dark, light}, {light, dark}} {
			m.SetTheme(c.drawn)
			if view := m.View(); !strings.Contains(view, border(c.drawn)) || strings.Contains(view, border(c.other)) {
				t.Errorf("the %s is not drawn only in the theme set: %q", page, strings.Split(view, "\n")[0])
			}
		}
	}
}
