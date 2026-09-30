package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
)

const missingBranchStatus = "⚠ Branch no longer exists · tasks cannot be edited"

func (m *model) View() tea.View {
	width, height := m.width, m.height
	if width < 1 {
		width = 100
	}
	if height < 1 {
		height = 30
	}
	if width < 56 || height < 16 {
		v := tea.NewView("todo · enlarge your terminal\nq quit")
		v.AltScreen = true
		return v
	}

	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	bodyHeight := height - 2
	var body string
	if m.indexMode {
		body = m.renderIndex(width, bodyHeight)
	} else {
		switch m.focus {
		case branchPane:
			body = m.renderNavigationPane(m.branchRows(), m.branchCursor, branchPane, width, bodyHeight)
		case sourcePane:
			body = m.renderSourcePane(width, bodyHeight)
		case detailPane:
			body = m.renderDetailPane(width, bodyHeight)
		default:
			body = m.renderNavigationPane(m.generalRows(), m.generalCursor, generalPane, width, bodyHeight)
		}
	}
	content := header + "\n" + body + "\n" + footer
	if m.modal != nil {
		modalWidth, modalHeight := m.modal.dimensions(width, height)
		content = overlay(content, m.modal.render(modalWidth, modalHeight), width, height)
	}
	if m.confirmClear != nil {
		content = overlay(content, m.confirmClear.render(), width, height)
	}
	if m.helpOpen {
		content = overlay(content, renderHelp(), width, height)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func overlay(content, dialog string, width, height int) string {
	x, y := (width-lipgloss.Width(dialog))/2, (height-lipgloss.Height(dialog))/2
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(content),
		lipgloss.NewLayer(dialog).X(x).Y(y).Z(1),
	).Render()
}

// renderHeader puts the tabs on the left and the Git context on the right,
// giving the tabs priority when the terminal is narrow.
func (m *model) renderHeader(width int) string {
	tabs := ""
	if !m.indexMode {
		tabs = m.renderTabs()
	}
	project := m.renderProject(width - ansi.StringWidth(tabs) - 1)
	tabs = ansi.Truncate(tabs, width-ansi.StringWidth(project), "…")
	gap := strings.Repeat(" ", max(0, width-ansi.StringWidth(tabs)-ansi.StringWidth(project)))
	return tabs + lipgloss.NewStyle().Background(colorBar).Render(gap) + project
}

// renderProject shows the repository and branch in at most width cells,
// dropping the repository name before shortening the branch below minBranch.
func (m *model) renderProject(width int) string {
	const minBranch = 8
	if m.project.branch == "" {
		return ""
	}
	bar := lipgloss.NewStyle().Background(colorBar)
	icon := gitIcon + " "
	repo := ""
	if m.project.root != "" {
		repo = filepath.Base(m.project.root) + "  "
	}
	fixed := ansi.StringWidth(icon) + 2
	if width-fixed-ansi.StringWidth(repo) < min(minBranch, ansi.StringWidth(m.project.branch)) {
		repo = ""
	}
	branchWidth := width - fixed - ansi.StringWidth(repo)
	if branchWidth < min(minBranch, ansi.StringWidth(m.project.branch)) {
		return ""
	}
	return bar.Foreground(colorText).Render(repo) + bar.Foreground(colorGit).Render(icon) +
		bar.Foreground(colorGreen).Render(ansi.Truncate(m.project.branch, branchWidth, "…")+"  ")
}

func (m *model) renderTabs() string {
	nameStyle := lipgloss.NewStyle().Foreground(colorMuted).Background(colorBar)
	barKeyStyle := keyStyle.Background(colorBar)
	gapStyle := lipgloss.NewStyle().Background(colorBar)
	items := []struct {
		key   string
		name  string
		count string
		pane  pane
	}{
		{"1", "General", fmt.Sprintf("%d/%d", completedCount(m.general)+completedCount(m.readme), len(m.general)+len(m.readme)), generalPane},
		{"2", "Branches", fmt.Sprintf("%d/%d", completedCount(m.branches), len(m.branches)), branchPane},
		{"3", "Files", m.sourceCount(), sourcePane},
	}
	var tabs strings.Builder
	for i, item := range items {
		if i > 0 {
			tabs.WriteString(gapStyle.Render(" "))
		}
		if m.activePane() == item.pane {
			tabs.WriteString(selectedStyle.Render(" ") + keyStyle.Background(colorSelection).Render(item.key) +
				selectedStyle.Render(" "+item.name+" ") + selectedDoneStyle.Render(item.count+" "))
		} else {
			tabs.WriteString(nameStyle.Render(" ") + barKeyStyle.Render(item.key) +
				nameStyle.Render(" "+item.name+" ") + nameStyle.Render(item.count+" "))
		}
	}
	return tabs.String()
}

func (m *model) sourceCount() string {
	if m.sourceLoading {
		return "…"
	}
	return fmt.Sprint(len(m.source))
}

type keyHint struct{ key, label string }

var pinnedHints = []keyHint{{"?", "help"}, {"q", "quit"}}

func (m *model) renderFooter(width int) string {
	if m.categoryInput {
		input := m.input.View()
		return input + "  " + fitHints([]keyHint{{"enter", "save"}, {"esc", "cancel"}}, width-ansi.StringWidth(input)-2)
	}
	pinned := renderHints(pinnedHints)
	available := max(0, width-ansi.StringWidth(pinned)-3)
	left := ""
	if m.status != "" {
		left = statusStyle.Render(ansi.Truncate(m.status, max(1, available), "…"))
		if hints := fitHints(m.footerHints(), available-ansi.StringWidth(left)-3); hints != "" {
			left += "   " + hints
		}
	} else {
		left = fitHints(m.footerHints(), available)
	}
	return left + strings.Repeat(" ", max(0, width-ansi.StringWidth(left)-ansi.StringWidth(pinned))) + pinned
}

// fitHints keeps the leading hints that fit, so each list is ordered by importance.
func fitHints(hints []keyHint, width int) string {
	for n := len(hints); n > 0; n-- {
		if line := renderHints(hints[:n]); ansi.StringWidth(line) <= width {
			return line
		}
	}
	return ""
}

func renderHints(hints []keyHint) string {
	parts := make([]string, len(hints))
	for i, hint := range hints {
		parts[i] = keyStyle.Render(hint.key) + " " + mutedStyle.Render(hint.label)
	}
	return strings.Join(parts, "  ")
}

func (m *model) footerHints() []keyHint {
	hints := m.contextHints()
	if m.lastClear != nil {
		hints = append([]keyHint{{"u", "undo clear"}}, hints...)
	}
	return hints
}

func (m *model) contextHints() []keyHint {
	back := keyHint{"←", "back"}
	reload := keyHint{"r", "reload"}
	index := keyHint{"i", "all tasks"}
	switch {
	case m.indexMode:
		hints := []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}, {"s", "sort"}, {"c", "category"}, {"a", "add"}, {"b", "branch task"}}
		return append(append(hints, m.clearHint()...), back, reload)
	case m.focus == detailPane && m.viewingMissingBranch():
		return []keyHint{back}
	case m.viewingMissingBranch():
		hints := []keyHint{back, {"a", "add"}, {"→", "details"}}
		return append(append(hints, m.clearHint()...), index, reload)
	case m.focus == detailPane && m.activePane() == sourcePane:
		return []keyHint{back, {"e", "open file"}}
	case m.focus == detailPane && m.readmeSelected():
		return []keyHint{{"d", "done"}, {"e", "open file"}, back}
	case m.readmeSelected():
		return []keyHint{{"d", "done"}, {"e", "open file"}, back, {"→", "details"}, index, reload}
	case m.focus == detailPane && m.activePane() == branchPane:
		return []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}, back}
	case m.focus == detailPane:
		return []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}, {"c", "category"}, back}
	case m.focus == sourcePane:
		if _, ok := m.selectedSource(); !ok && len(m.sourceRows()) > 0 {
			return []keyHint{{"→", "open"}, index, reload}
		}
		hints := []keyHint{{"e", "open file"}, {"→", "details"}}
		if m.sourceCategory != "" {
			hints = append(hints, back)
		}
		return append(hints, index, reload)
	}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		hints := []keyHint{{"→", "open"}, {"a", "add"}}
		if m.focus == generalPane {
			hints = append(hints, keyHint{"b", "branch task"})
		}
		return append(append(hints, m.clearHint()...), index, reload)
	}
	hints := []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}}
	if m.focus == generalPane {
		hints = append(hints, keyHint{"c", "category"})
	}
	hints = append(hints, keyHint{"a", "add"})
	if m.focus == generalPane {
		hints = append(hints, keyHint{"b", "branch task"})
	}
	if len(m.breadcrumb(m.focus)) > 1 {
		hints = append(hints, back)
	}
	hints = append(hints, keyHint{"→", "details"})
	return append(append(hints, m.clearHint()...), index, reload)
}

// viewingMissingBranch is true inside a branch whose Git branch is gone, where
// its tasks are read only.
func (m *model) viewingMissingBranch() bool {
	return m.activePane() == branchPane && m.branchMissing(m.branchFilter)
}

// breadcrumb names where a pane is; a single entry means its top level.
func (m *model) breadcrumb(kind pane) []string {
	switch kind {
	case branchPane:
		if m.branchFilter != "" {
			return []string{"Branches", branchIcon + " " + m.branchFilter}
		}
		return []string{"Branches"}
	case sourcePane:
		if m.sourceCategory != "" {
			return []string{"Files", "@" + m.sourceCategory}
		}
		return []string{"Files"}
	case detailPane:
		return append(m.breadcrumb(m.detailFrom), "Details")
	}
	switch {
	case m.readmeOpen:
		return []string{"General", readmeGroup}
	case m.generalCategory != "":
		return []string{"General", "@" + m.generalCategory}
	}
	return []string{"General"}
}

func renderBreadcrumb(parts []string, suffix string, width int) string {
	last := len(parts) - 1
	var line strings.Builder
	for _, part := range parts[:last] {
		line.WriteString(mutedStyle.Render(part + " › "))
	}
	line.WriteString(titleStyle.Render(parts[last]))
	if suffix != "" {
		line.WriteString(mutedStyle.Render("  " + suffix))
	}
	return ansi.Truncate(line.String(), width, "…")
}

func (m *model) panelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder)
}

// panelStatus is the bar along the bottom of a list or detail panel: a
// warning for a missing branch, otherwise the selected row's status.
func (m *model) panelStatus() string {
	if m.viewingMissingBranch() {
		return statusBarStyle.Render(missingBranchStatus)
	}
	if m.activePane() == sourcePane {
		if item, ok := m.selectedSource(); ok {
			return mutedStyle.Render(fmt.Sprintf("%s:%d", cleanDisplay(item.Path), item.Line))
		}
		if rows := m.sourceRows(); m.sourceCursor < len(rows) {
			return mutedStyle.Render(plural(rows[m.sourceCursor].count, "TODO", "TODOs"))
		}
		return ""
	}
	row, ok := m.selectedNavigationRow()
	switch {
	case !ok:
		return ""
	case row.kind == rowTask && m.readmeOpen:
		return readmeTaskStatus(row.todo)
	case row.kind == rowTask:
		return taskStatus(row.todo)
	}
	return mutedStyle.Render(fmt.Sprintf("%d of %d done", row.completed, row.count))
}

func taskStatus(t store.Task) string {
	status := lipgloss.NewStyle().Foreground(colorText).Render("Open")
	if t.Done {
		status = lipgloss.NewStyle().Foreground(colorGreen).Render("✓ Done")
	}
	priority := mutedStyle.Render("No priority")
	if t.Priority != "" {
		priority = priorityStyle(t.Priority).Render(priorityMark(t.Priority) + " " + t.Priority.Title() + " priority")
	}
	return status + mutedStyle.Render("  ·  ") + priority
}

// readmeTaskStatus names where a README task is, as the Files tab does,
// since it has no priority.
func readmeTaskStatus(t store.Task) string {
	status := lipgloss.NewStyle().Foreground(colorText).Render("Open")
	if t.Done {
		status = lipgloss.NewStyle().Foreground(colorGreen).Render("✓ Done")
	}
	return status + mutedStyle.Render(fmt.Sprintf("  ·  %s:%d", readmeGroup, t.Line+1))
}

func (m *model) panelContentHeight(height int) int {
	lines := height - 2
	if m.panelStatus() != "" {
		lines--
	}
	return max(1, lines)
}

func (m *model) renderPanel(width, height int, lines []string) string {
	if status := m.panelStatus(); status != "" {
		contentHeight := m.panelContentHeight(height)
		lines = lines[:min(len(lines), contentHeight)]
		for len(lines) < contentHeight {
			lines = append(lines, "")
		}
		innerWidth := max(1, width-4)
		status = ansi.Truncate(status, max(1, innerWidth-2), "…")
		lines = append(lines, lipgloss.NewStyle().Background(colorBar).Width(innerWidth).Padding(0, 1).Render(onBackground(status, colorBar)))
	}
	return m.panelStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (m *model) renderNavigationPane(rows []navigationRow, cursor int, kind pane, width, height int) string {
	innerWidth := max(1, width-4)
	var lines []string
	if crumbs := m.breadcrumb(kind); len(crumbs) > 1 {
		completed, count := 0, 0
		for _, row := range rows {
			if row.kind == rowTask {
				count++
				if row.todo.Done {
					completed++
				}
			}
		}
		lines = append(lines, renderBreadcrumb(crumbs, fmt.Sprintf("%d/%d", completed, count), innerWidth), "")
	}
	visible := max(1, m.panelContentHeight(height)-len(lines))
	divider := firstTaskAfterGroups(rows)
	if divider > 0 {
		visible = max(1, visible-1)
	}
	start, end := visibleRange(cursor, len(rows), visible)
	levelWidth := 0
	for _, row := range rows {
		levelWidth = max(levelWidth, len(levelLabel(row.todo.Level)))
	}
	if len(rows) == 0 {
		empty := "No tasks"
		if kind == branchPane {
			empty = "No branch TODOs"
		}
		lines = append(lines, mutedStyle.Render(empty))
	}
	for i := start; i < end; i++ {
		item := rows[i]
		selected := i == cursor && (m.focus == kind || (m.focus == detailPane && m.detailFrom == kind))
		if i == divider && i > start {
			lines = append(lines, "")
		}
		if item.kind == rowTask {
			lines = append(lines, renderTaskRow(item.todo, innerWidth, levelWidth, selected))
		} else {
			lines = append(lines, renderGroupRow(item, innerWidth, selected, item.kind == rowBranch && item.name == m.project.branch))
		}
	}
	return m.renderPanel(width, height, lines)
}

// firstTaskAfterGroups is the row where loose tasks follow categories or
// branches, or -1 when the list is not mixed.
func firstTaskAfterGroups(rows []navigationRow) int {
	for i := 1; i < len(rows); i++ {
		if isItem(rows[i].kind) && !isItem(rows[i-1].kind) {
			return i
		}
	}
	return -1
}

func isItem(kind navigationKind) bool {
	return kind == rowTask || kind == rowFile
}

func renderGroupRow(item navigationRow, width int, selected, current bool) string {
	marker, note := "▸ ", ""
	nameStyle := lipgloss.NewStyle()
	noteStyle := mutedStyle.Italic(true)
	switch {
	case item.missingGitBranch:
		marker, note = "⚠ ", "missing"
		nameStyle = nameStyle.Foreground(colorHigh)
		noteStyle = nameStyle.Italic(true)
	case current:
		note = "current"
		nameStyle = nameStyle.Foreground(colorGreen)
	case item.kind == rowCategory || item.kind == rowFileCategory:
		nameStyle = nameStyle.Foreground(colorPurple)
	}
	countStyle, fillStyle := mutedStyle, lipgloss.NewStyle()
	if selected {
		if item.kind != rowBranch || (!current && !item.missingGitBranch) {
			nameStyle = nameStyle.Foreground(colorStrong)
		}
		nameStyle = nameStyle.Background(colorSelection)
		noteStyle = noteStyle.Background(colorSelection)
		countStyle, fillStyle = selectedDoneStyle, selectedStyle
	}
	if note != "" {
		note = "  " + note
	}
	count := fmt.Sprintf("%d/%d", item.completed, item.count)
	if item.kind == rowFileCategory {
		count = fmt.Sprint(item.count)
	}
	fixed := ansi.StringWidth(marker) + ansi.StringWidth(note) + ansi.StringWidth(count) + 2
	name := ansi.Truncate(item.name, max(0, width-fixed), "…")
	gap := max(2, width-fixed+2-ansi.StringWidth(name))
	return nameStyle.Render(marker+name) + noteStyle.Render(note) + fillStyle.Render(strings.Repeat(" ", gap)) + countStyle.Render(count)
}

// renderTaskRow shows a task's priority, or with a levelWidth above 0, a
// column of README levels in its place.
func renderTaskRow(t store.Task, width, levelWidth int, selected bool) string {
	mark, markStyle := priorityMark(t.Priority), mutedStyle
	if t.Priority != "" {
		markStyle = priorityStyle(t.Priority)
	}
	if levelWidth > 0 {
		mark, markStyle = levelMark(t.Level)
	}
	titleStyle := lipgloss.NewStyle().Foreground(colorStrong)
	if t.Done {
		mark, markStyle, titleStyle = "✓", mutedStyle, mutedStyle
	}
	mark += strings.Repeat(" ", max(0, levelWidth-ansi.StringWidth(mark))+1)
	suffix := ""
	if strings.TrimSpace(t.Details) != "" {
		suffix = "⋯"
	}
	reserved := ansi.StringWidth(mark)
	if suffix != "" {
		reserved += ansi.StringWidth(suffix) + 2
	}
	title := ansi.Truncate(cleanDisplay(t.Text), max(0, width-reserved), "…")
	suffixStyle, fillStyle := mutedStyle, lipgloss.NewStyle()
	if selected {
		markStyle = markStyle.Background(colorSelection)
		titleStyle = titleStyle.Background(colorSelection)
		if t.Done {
			titleStyle = selectedDoneStyle
		}
		suffixStyle, fillStyle = selectedDoneStyle, selectedStyle
	}
	fill := ""
	if selected || suffix != "" {
		fill = strings.Repeat(" ", max(0, width-ansi.StringWidth(mark)-ansi.StringWidth(title)-ansi.StringWidth(suffix)))
	}
	return markStyle.Render(mark) + titleStyle.Render(title) + fillStyle.Render(fill) + suffixStyle.Render(suffix)
}

func (m *model) renderSourcePane(width, height int) string {
	innerWidth := max(1, width-4)
	rows := m.sourceRows()
	var lines []string
	if crumbs := m.breadcrumb(sourcePane); len(crumbs) > 1 {
		lines = append(lines, renderBreadcrumb(crumbs, fmt.Sprint(len(rows)), innerWidth), "")
	}
	visible := max(1, m.panelContentHeight(height)-len(lines))
	divider := firstTaskAfterGroups(rows)
	if divider > 0 {
		visible = max(1, visible-1)
	}
	start, end := visibleRange(m.sourceCursor, len(rows), visible)
	if len(rows) == 0 {
		empty := "No matches"
		if m.sourceLoading {
			empty = "Scanning…"
		}
		if m.sourceError != "" {
			empty = "Scan failed"
		}
		lines = append(lines, mutedStyle.Render(empty))
	}
	levelWidth := 0
	for _, row := range rows {
		levelWidth = max(levelWidth, len(levelLabel(row.match.Level)))
	}
	for i := start; i < end; i++ {
		selected := i == m.sourceCursor && (m.focus == sourcePane || (m.focus == detailPane && m.detailFrom == sourcePane))
		if i == divider && i > start {
			lines = append(lines, "")
		}
		if rows[i].kind == rowFileCategory {
			lines = append(lines, renderGroupRow(rows[i], innerWidth, selected, false))
		} else {
			lines = append(lines, renderSourceRow(rows[i].match, innerWidth, levelWidth, selected))
		}
	}
	return m.renderPanel(width, height, lines)
}

// levelMark is a file TODO's level label and colour: red for levels of
// zeros, yellow for the rest, and a dim dot without a level.
func levelMark(level string) (string, lipgloss.Style) {
	switch {
	case level == "":
		return "·", mutedStyle
	case strings.Trim(level, "0") == "":
		return levelLabel(level), lipgloss.NewStyle().Foreground(colorHigh)
	}
	return levelLabel(level), lipgloss.NewStyle().Foreground(colorMedium)
}

// levelLabel keeps a level short: four or more zeros are written as 0x4 up
// to 0x9, and 0x9+ for anything longer; the list still sorts by the real
// count.
func levelLabel(level string) string {
	switch {
	case len(level) < 4 || strings.Trim(level, "0") != "":
		return level
	case len(level) > 9:
		return "0x9+"
	}
	return fmt.Sprintf("0x%d", len(level))
}

// renderSourceRow shows a file TODO's level, its text, and where it is.
// A levelWidth of 0 means no row has a level, so the column is left out.
func renderSourceRow(item scan.Match, width, levelWidth int, selected bool) string {
	level, levelStyle := levelMark(item.Level)
	if levelWidth == 0 {
		level = ""
	}
	if levelWidth > 0 {
		level += strings.Repeat(" ", levelWidth-ansi.StringWidth(level)+1)
	}
	note := lipgloss.NewStyle().Foreground(colorStrong)
	file := mutedStyle
	gap := "  "
	if selected {
		levelStyle, note, file = levelStyle.Background(colorSelection), note.Background(colorSelection), file.Background(colorSelection)
		gap = lipgloss.NewStyle().Background(colorSelection).Render(gap)
	}
	fileWidth := min(40, max(20, width*2/5))
	noteWidth := max(1, width-ansi.StringWidth(level)-2-fileWidth)
	location := truncatePath(fmt.Sprintf("%s:%d", item.Path, item.Line), fileWidth)
	return levelStyle.Render(level) + note.Render(indexColumn(item.Note, noteWidth)) + gap + file.Render(indexColumn(location, fileWidth))
}

// truncatePath shortens a path from the left, a whole directory at a time,
// so its file name and line stay visible.
func truncatePath(value string, width int) string {
	value = cleanDisplay(value)
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

func (m *model) renderDetailPane(width, height int) string {
	innerWidth := max(1, width-4)
	lines := []string{renderBreadcrumb(m.breadcrumb(detailPane), "", innerWidth), ""}
	if m.activePane() == sourcePane {
		lines = append(lines, m.sourceDetails(innerWidth, m.panelContentHeight(height)-len(lines))...)
	} else if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		lines = append(lines, m.groupDetails(row, innerWidth)...)
	} else {
		lines = append(lines, m.taskDetails(innerWidth)...)
	}
	maxLines := m.panelContentHeight(height)
	start := min(m.detailScroll, max(0, len(lines)-maxLines))
	lines = lines[start:min(len(lines), start+maxLines)]
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, innerWidth, "…")
	}
	return m.renderPanel(width, height, lines)
}

func groupName(row navigationRow) string {
	if row.kind == rowCategory {
		return "@" + row.name
	}
	return row.name
}

func (m *model) groupDetails(row navigationRow, width int) []string {
	scope := "general tasks"
	switch row.kind {
	case rowBranch:
		scope = "branch tasks"
	case rowReadme:
		scope = "tasks under TODO headings"
	}
	return wrapLines([]string{taskTitleStyle.Render(groupName(row)), "", fmt.Sprintf("%d/%d %s", row.completed, row.count, scope), "", mutedStyle.Render("enter or → to open")}, width)
}

func (m *model) taskDetails(width int) []string {
	t, ok := m.selectedTask()
	if !ok {
		return []string{"", mutedStyle.Render("Select a Markdown task.")}
	}
	result := []string{leveledTitle(t.Text, t.Level), ""}
	switch {
	case m.readmeSelected():
		result = result[:1]
	case t.Details == "":
		result = append(result, mutedStyle.Render("No details yet"))
	default:
		for line := range strings.SplitSeq(t.Details, "\n") {
			result = append(result, cleanDisplay(line))
		}
	}
	return wrapLines(result, width)
}

// sourceDetails shows a file TODO's text, then as much of the file around
// it as fits in height. The status bar names the file, so it isn't repeated.
func (m *model) sourceDetails(width, height int) []string {
	item, ok := m.selectedSource()
	if !ok {
		if m.sourceError != "" {
			return wrapLines([]string{"Scan failed", m.sourceError, "", "Press r to try again"}, width)
		}
		return []string{mutedStyle.Render("No file TODO selected.")}
	}
	result := append(wrapLines([]string{leveledTitle(item.Note, item.Level)}, width), "")
	switch {
	case m.previewPath != item.Path || m.previewLine != item.Line:
		return append(result, mutedStyle.Render("Loading preview…"))
	case m.previewError != "":
		return append(result, wrapLines([]string{mutedStyle.Render(m.previewError)}, width)...)
	}
	return append(result, contextWindow(m.preview, item.Line, width, height-len(result))...)
}

// contextWindow shows the source lines that fit in height, centred on the
// TODO's line where the file allows. Long lines are cut rather than wrapped,
// so the line numbers stay in one column.
// leveledTitle is a detail page's title, led by its todo-system level.
func leveledTitle(text, level string) string {
	title := taskTitleStyle.Render(cleanDisplay(text))
	if level == "" {
		return title
	}
	label, style := levelMark(level)
	return style.Bold(true).Render(label) + " " + title
}

func contextWindow(lines []scan.ContextLine, target, width, height int) []string {
	at := slices.IndexFunc(lines, func(line scan.ContextLine) bool { return line.Number == target })
	height = max(1, height)
	end := min(len(lines), max(0, at-(height-1)/2)+height)
	start := max(0, end-height)
	var result []string
	for _, line := range lines[start:end] {
		gutter := fmt.Sprintf("%4d │ ", line.Number)
		text := ansi.Truncate(cleanDisplay(strings.ReplaceAll(line.Text, "\t", "    ")), max(1, width-ansi.StringWidth(gutter)), "…")
		if line.Number == target {
			result = append(result, selectedStyle.Width(width).Render(gutter+text))
			continue
		}
		result = append(result, mutedStyle.Render(gutter)+text)
	}
	return result
}

func wrapLines(lines []string, width int) []string {
	var result []string
	for _, line := range lines {
		if line == "" {
			result = append(result, "")
			continue
		}
		wrapped := ansi.Wrap(line, width, "")
		result = append(result, strings.Split(wrapped, "\n")...)
	}
	return result
}

func cleanDisplay(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func visibleRange(cursor, total, height int) (int, int) {
	if height < 1 {
		height = 1
	}
	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	end := min(start+height, total)
	return start, end
}
