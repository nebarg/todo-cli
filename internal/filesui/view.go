package filesui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/ui"
)

// View draws the list, or the detail page, in a panel of the size set by
// SetSize, coloured by the theme set by SetTheme.
func (m *Model) View() string {
	if m.details {
		return m.detailsView()
	}
	return m.listView()
}

// Hints are the browser's keys for the footer.
func (m *Model) Hints() []ui.KeyHint {
	back := ui.Hint(keys.Back, "back")
	if m.details {
		return []ui.KeyHint{back, ui.Hint(keys.Edit, "open file")}
	}
	if _, ok := m.Selected(); !ok {
		if len(m.rows()) == 0 {
			return nil
		}
		return []ui.KeyHint{ui.Hint(keys.Open, "open")}
	}
	hints := []ui.KeyHint{ui.Hint(keys.Edit, "open file"), ui.Hint(keys.Open, "details")}
	if m.category != "" {
		hints = append(hints, back)
	}
	return hints
}

func (m *Model) breadcrumb() []string {
	crumbs := []string{"Files"}
	if m.category != "" {
		crumbs = append(crumbs, "@"+m.category)
	}
	if m.details {
		crumbs = append(crumbs, "Details")
	}
	return crumbs
}

// status is the bar along the bottom of the panel: where the highlighted
// TODO is, or how many TODOs a highlighted category holds.
func (m *Model) status(theme ui.Theme) string {
	if item, ok := m.Selected(); ok {
		return theme.MutedStyle.Render(fmt.Sprintf("%s:%d", ui.CleanDisplay(item.Path), item.Line))
	}
	if rows := m.rows(); m.cursor < len(rows) {
		return theme.MutedStyle.Render(ui.Plural(rows[m.cursor].count, "TODO", "TODOs"))
	}
	return ""
}

func (m *Model) listView() string {
	innerWidth := max(1, m.width-4)
	rows := m.rows()
	status := m.status(m.theme)
	var lines []string
	if crumbs := m.breadcrumb(); len(crumbs) > 1 {
		lines = append(lines, m.theme.Breadcrumb(crumbs, fmt.Sprint(len(rows)), innerWidth), "")
	}
	visible := max(1, ui.ContentHeight(m.height, status)-len(lines))
	divider := firstTodoAfterGroups(rows)
	if divider > 0 {
		visible = max(1, visible-1)
	}
	start, end := ui.VisibleRange(m.cursor, len(rows), visible)
	if len(rows) == 0 {
		empty := "No matches"
		if m.loading {
			empty = "Scanning…"
		}
		if m.err != nil {
			empty = "Scan failed"
		}
		lines = append(lines, m.theme.MutedStyle.Render(empty))
	}
	levelWidth := 0
	for _, row := range rows {
		levelWidth = max(levelWidth, len(ui.LevelLabel(row.match.Level)))
	}
	for i := start; i < end; i++ {
		selected := i == m.cursor
		if i == divider && i > start {
			lines = append(lines, "")
		}
		if rows[i].isGroup() {
			lines = append(lines, groupRow(m.theme, rows[i]).Render(m.theme, innerWidth, selected))
		} else {
			lines = append(lines, todoRow(m.theme, rows[i].match, innerWidth, levelWidth, selected))
		}
	}
	return m.theme.Panel(m.width, m.height, lines, status)
}

// firstTodoAfterGroups is the row where TODOs follow category groups, or -1
// when the list is not mixed.
func firstTodoAfterGroups(rows []row) int {
	for i := 1; i < len(rows); i++ {
		if !rows[i].isGroup() && rows[i-1].isGroup() {
			return i
		}
	}
	return -1
}

func groupRow(theme ui.Theme, r row) ui.GroupRow {
	return ui.GroupRow{
		Marker: "▸ ", Name: r.category, Count: fmt.Sprint(r.count),
		NameStyle: lipgloss.NewStyle().Foreground(theme.ColorPurple),
	}
}

// todoRow shows a TODO's level, its text, and where it is. A levelWidth of
// 0 means no row has a level, so the column is left out.
func todoRow(theme ui.Theme, item scan.Match, width, levelWidth int, selected bool) string {
	level, levelStyle := theme.LevelMark(item.Level)
	if levelWidth == 0 {
		level = ""
	}
	if levelWidth > 0 {
		level += strings.Repeat(" ", levelWidth-ansi.StringWidth(level)+1)
	}
	note := lipgloss.NewStyle().Foreground(theme.ColorStrong)
	file := theme.MutedStyle
	gap := "  "
	if selected {
		levelStyle, note, file = levelStyle.Background(theme.ColorSelection), note.Background(theme.ColorSelection), file.Background(theme.ColorSelection)
		gap = lipgloss.NewStyle().Background(theme.ColorSelection).Render(gap)
	}
	fileWidth := min(40, max(20, width*2/5))
	noteWidth := max(1, width-ansi.StringWidth(level)-2-fileWidth)
	location := truncatePath(fmt.Sprintf("%s:%d", item.Path, item.Line), fileWidth)
	return levelStyle.Render(level) + note.Render(ui.Column(item.Note, noteWidth)) + gap + file.Render(ui.Column(location, fileWidth))
}

// truncatePath shortens a path from the left, a whole directory at a time,
// so its file name and line stay visible.
func truncatePath(value string, width int) string {
	value = ui.CleanDisplay(value)
	for rest := value; ansi.StringWidth(value) > width; {
		cut := strings.Index(rest, "/")
		if cut < 0 {
			return "…" + ansi.TruncateLeft(rest, ansi.StringWidth(rest)-width+1, "")
		}
		rest = rest[cut+1:]
		value = "…/" + rest
	}
	return value
}

func (m *Model) detailsView() string {
	lines, maxLines := m.detailsPage()
	start := min(m.scroll, max(0, len(lines)-maxLines))
	lines = lines[start:min(len(lines), start+maxLines)]
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	innerWidth := max(1, m.width-4)
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, innerWidth, "…")
	}
	return m.theme.Panel(m.width, m.height, lines, m.status(m.theme))
}

// detailsPage is everything the detail page shows, wrapped to the panel's
// width but not yet cut to its height, and how many lines the panel has room
// for.
func (m *Model) detailsPage() (lines []string, maxLines int) {
	innerWidth := max(1, m.width-4)
	maxLines = ui.ContentHeight(m.height, m.status(m.theme))
	lines = []string{m.theme.Breadcrumb(m.breadcrumb(), "", innerWidth), ""}
	lines = append(lines, m.detailLines(m.theme, innerWidth, maxLines-len(lines))...)
	return lines, maxLines
}

// detailsBottom is the furthest the detail page scrolls at the panel's size:
// its last line at the bottom of the panel.
func (m *Model) detailsBottom() int {
	lines, maxLines := m.detailsPage()
	return max(0, len(lines)-maxLines)
}

// detailLines shows a TODO's text, then as much of the file around it as
// fits in height. The status bar names the file, so it isn't repeated.
func (m *Model) detailLines(theme ui.Theme, width, height int) []string {
	item, ok := m.Selected()
	if !ok {
		if m.err != nil {
			return ui.WrapLines([]string{"Scan failed", m.err.Error(), "", "Press r to try again"}, width)
		}
		return []string{theme.MutedStyle.Render("No file TODO selected.")}
	}
	result := append(ui.WrapLines([]string{theme.LeveledTitle(theme.TaskTitleStyle.Render(ui.CleanDisplay(item.Note)), item.Level)}, width), "")
	switch {
	case m.previewPath != item.Path || m.previewLine != item.Line:
		return append(result, theme.MutedStyle.Render("Loading preview…"))
	case m.previewErr != nil:
		return append(result, ui.WrapLines([]string{theme.MutedStyle.Render(m.previewErr.Error())}, width)...)
	}
	return append(result, contextWindow(theme, m.preview, item.Line, width, height-len(result))...)
}

// contextWindow shows the source lines that fit in height, centred on the
// TODO's line where the file allows. Long lines are cut rather than wrapped,
// so the line numbers stay in one column.
func contextWindow(theme ui.Theme, lines []scan.ContextLine, target, width, height int) []string {
	at := slices.IndexFunc(lines, func(line scan.ContextLine) bool { return line.Number == target })
	height = max(1, height)
	end := min(len(lines), max(0, at-(height-1)/2)+height)
	start := max(0, end-height)
	var result []string
	for _, line := range lines[start:end] {
		gutter := fmt.Sprintf("%4d │ ", line.Number)
		text := ansi.Truncate(ui.CleanDisplay(strings.ReplaceAll(line.Text, "\t", "    ")), max(1, width-ansi.StringWidth(gutter)), "…")
		if line.Number == target {
			result = append(result, theme.SelectedStyle.Width(width).Render(gutter+text))
			continue
		}
		result = append(result, theme.MutedStyle.Render(gutter)+text)
	}
	return result
}
