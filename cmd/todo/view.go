package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
		{"1", "General", fmt.Sprintf("%d/%d", completedCount(m.general), len(m.general)), generalPane},
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
		return "↻"
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
	back := keyHint{"←", "back"}
	reload := keyHint{"r", "reload"}
	index := keyHint{"i", "all tasks"}
	switch {
	case m.indexMode:
		return []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}, {"s", "sort"}, {"c", "category"}, {"b", "branch task"}, back, reload}
	case m.focus == detailPane && m.viewingMissingBranch():
		return []keyHint{back}
	case m.viewingMissingBranch():
		return []keyHint{back, {"a", "add"}, {"→", "details"}, index, reload}
	case m.focus == detailPane && m.activePane() == sourcePane:
		return []keyHint{back, {"e", "open file"}}
	case m.focus == detailPane && m.activePane() == branchPane:
		return []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}, back}
	case m.focus == detailPane:
		return []keyHint{{"d", "done"}, {"e", "edit"}, {"p", "priority"}, {"c", "category"}, back}
	case m.focus == sourcePane:
		return []keyHint{{"e", "open file"}, {"→", "details"}, index, reload}
	}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		hints := []keyHint{{"→", "open"}, {"a", "add"}}
		if m.focus == generalPane {
			hints = append(hints, keyHint{"b", "branch task"})
		}
		return append(hints, index, reload)
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
	return append(hints, keyHint{"→", "details"}, index, reload)
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
		return []string{"Files"}
	case detailPane:
		return append(m.breadcrumb(m.detailFrom), "Details")
	}
	if m.generalCategory != "" {
		return []string{"General", "@" + m.generalCategory}
	}
	return []string{"General"}
}

func renderBreadcrumb(parts []string, suffix string, width int) string {
	last := len(parts) - 1
	line := ""
	for _, part := range parts[:last] {
		line += mutedStyle.Render(part + " › ")
	}
	line += titleStyle.Render(parts[last])
	if suffix != "" {
		line += mutedStyle.Render("  " + suffix)
	}
	return ansi.Truncate(line, width, "…")
}

func (m *model) panelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder)
}

func (m *model) panelStatus() string {
	if m.viewingMissingBranch() {
		return missingBranchStatus
	}
	return ""
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
		lines = append(lines, statusBarStyle.Width(innerWidth).Render(ansi.Truncate(status, innerWidth, "…")))
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
			lines = append(lines, renderTaskRow(item.todo, innerWidth, selected))
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
		if rows[i].kind == rowTask && rows[i-1].kind != rowTask {
			return i
		}
	}
	return -1
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
	case item.kind == rowCategory:
		nameStyle = nameStyle.Foreground(colorPurple)
	}
	countStyle, fillStyle := mutedStyle, lipgloss.NewStyle()
	if selected {
		if item.kind == rowCategory || (item.kind == rowBranch && !current && !item.missingGitBranch) {
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
	fixed := ansi.StringWidth(marker) + ansi.StringWidth(note) + ansi.StringWidth(count) + 2
	name := ansi.Truncate(item.name, max(0, width-fixed), "…")
	gap := max(2, width-fixed+2-ansi.StringWidth(name))
	return nameStyle.Render(marker+name) + noteStyle.Render(note) + fillStyle.Render(strings.Repeat(" ", gap)) + countStyle.Render(count)
}

func renderTaskRow(t store.Task, width int, selected bool) string {
	mark, markStyle := "○ ", mutedStyle
	if t.Priority != "" {
		markStyle = priorityStyle(t.Priority)
	}
	titleStyle := lipgloss.NewStyle().Foreground(colorStrong)
	if t.Done {
		mark, markStyle, titleStyle = "✓ ", mutedStyle, mutedStyle
	}
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
	var lines []string
	visible := max(1, height-2)
	start, end := visibleRange(m.sourceCursor, len(m.source), visible)
	if len(m.source) == 0 {
		empty := "No matches"
		if m.sourceLoading {
			empty = "Scanning…"
		}
		if m.sourceError != "" {
			empty = "Scan failed"
		}
		lines = append(lines, mutedStyle.Render(empty))
	}
	for i := start; i < end; i++ {
		item := m.source[i]
		row := fmt.Sprintf("%s:%d  %s", item.Path, item.Line, cleanDisplay(item.Text))
		row = ansi.Truncate(row, innerWidth, "…")
		if i == m.sourceCursor && (m.focus == sourcePane || (m.focus == detailPane && m.detailFrom == sourcePane)) {
			lines = append(lines, selectedStyle.Width(innerWidth).Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return m.panelStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (m *model) renderDetailPane(width, height int) string {
	innerWidth := max(1, width-4)
	lines := []string{renderBreadcrumb(m.breadcrumb(detailPane), "", innerWidth), ""}
	if height < 8 {
		lines = append(lines, m.compactDetails()...)
	} else if m.activePane() == sourcePane {
		lines = append(lines, m.sourceDetails(innerWidth)...)
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

func (m *model) compactDetails() []string {
	if m.activePane() == sourcePane {
		if m.sourceCursor >= len(m.source) {
			return []string{"No file selected"}
		}
		item := m.source[m.sourceCursor]
		return []string{item.Path + ":" + fmt.Sprint(item.Line), cleanDisplay(item.Text)}
	}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		return []string{groupName(row), fmt.Sprintf("%d/%d tasks · enter/→ open", row.completed, row.count)}
	}
	t, ok := m.selectedTask()
	if !ok {
		return []string{"No task selected"}
	}
	if t.Details != "" {
		return []string{cleanDisplay(t.Text), cleanDisplay(strings.Split(t.Details, "\n")[0])}
	}
	mark := "open"
	if t.Done {
		mark = "done"
	}
	return []string{cleanDisplay(t.Text), mark + " · " + taskLocation(t)}
}

func taskLocation(t store.Task) string {
	if t.Branch == "" {
		return "General"
	}
	return "Branch " + t.Branch
}

func groupName(row navigationRow) string {
	if row.kind == rowCategory {
		return "@" + row.name
	}
	return row.name
}

func (m *model) groupDetails(row navigationRow, width int) []string {
	scope := "general tasks"
	if row.kind == rowBranch {
		scope = "branch tasks"
	}
	return wrapLines([]string{taskTitleStyle.Render(groupName(row)), "", fmt.Sprintf("%d/%d %s", row.completed, row.count, scope), "", mutedStyle.Render("enter or → to open")}, width)
}

func (m *model) taskDetails(width int) []string {
	t, ok := m.selectedTask()
	if !ok {
		return []string{"", mutedStyle.Render("Select a Markdown task.")}
	}
	status := "Open"
	if t.Done {
		status = "Complete"
	}
	priorityText := mutedStyle.Render("None")
	if t.Priority != "" {
		priorityText = priorityStyle(t.Priority).Render(t.Priority.Title())
	}
	category := "None"
	if t.Category != "" {
		category = "@" + t.Category
	}
	result := []string{taskTitleStyle.Render(cleanDisplay(t.Text)), ""}
	if t.Details == "" {
		result = append(result, mutedStyle.Render("No details yet"))
	} else {
		for line := range strings.SplitSeq(t.Details, "\n") {
			result = append(result, cleanDisplay(line))
		}
	}
	result = append(result, "", mutedStyle.Render("Status    "+status), mutedStyle.Render("Scope     "+taskLocation(t)),
		mutedStyle.Render("Priority  ")+priorityText, mutedStyle.Render("Category  "+category))
	return wrapLines(result, width)
}

func (m *model) sourceDetails(width int) []string {
	if m.sourceCursor >= len(m.source) {
		if m.sourceError != "" {
			return wrapLines([]string{"", "Scan failed", m.sourceError, "", "Press r to try again"}, width)
		}
		return []string{"", mutedStyle.Render("No file TODO selected.")}
	}
	item := m.source[m.sourceCursor]
	result := []string{"", mutedStyle.Render("FILE TODO"), item.Path + ":" + fmt.Sprint(item.Line),
		"", cleanDisplay(item.Text), "", mutedStyle.Render("CONTEXT")}
	switch {
	case m.previewPath != item.Path || m.previewLine != item.Line:
		result = append(result, mutedStyle.Render("Loading preview…"))
	case m.previewError != "":
		result = append(result, mutedStyle.Render(m.previewError))
	default:
		for _, line := range m.preview {
			prefix := fmt.Sprintf("%4d │ ", line.Number)
			row := prefix + cleanDisplay(line.Text)
			if line.Number == item.Line {
				row = selectedStyle.Render(ansi.Truncate(row, width, "…"))
			}
			result = append(result, row)
		}
	}
	return wrapLines(result, width)
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
