package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
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
	if m.all != nil {
		body = m.all.view(m.allTasks, m.branchMissing, width, bodyHeight)
	} else {
		switch m.focus {
		case branchPane:
			body = m.renderNavigationPane(m.branchRows(), m.branchCursor, branchPane, width, bodyHeight)
		case sourcePane:
			body = m.files.View(width, bodyHeight)
		case detailPane:
			body = m.renderDetailPane(width, bodyHeight)
		default:
			body = m.renderNavigationPane(m.generalRows(), m.generalCursor, generalPane, width, bodyHeight)
		}
	}
	content := header + "\n" + body + "\n" + footer
	if d, ok := m.overlay.(dialog); ok {
		content = placeOver(content, d.view(width, height), width, height)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// placeOver draws box over the middle of content.
func placeOver(content, box string, width, height int) string {
	x, y := (width-lipgloss.Width(box))/2, (height-lipgloss.Height(box))/2
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(content),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()
}

// renderHeader puts the tabs on the left and the Git context on the right,
// giving the tabs priority when the terminal is narrow.
func (m *model) renderHeader(width int) string {
	tabs := ""
	if m.all == nil {
		tabs = m.renderTabs()
	}
	project := m.renderProject(width - ansi.StringWidth(tabs) - 1)
	tabs = ansi.Truncate(tabs, width-ansi.StringWidth(project), "…")
	gap := strings.Repeat(" ", max(0, width-ansi.StringWidth(tabs)-ansi.StringWidth(project)))
	return tabs + lipgloss.NewStyle().Background(ui.ColorBar).Render(gap) + project
}

// renderProject shows the repository and branch in at most width cells,
// dropping the repository name before shortening the branch below minBranch.
func (m *model) renderProject(width int) string {
	const minBranch = 8
	if m.project.branch == "" {
		return ""
	}
	bar := lipgloss.NewStyle().Background(ui.ColorBar)
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
	return bar.Foreground(ui.ColorText).Render(repo) + bar.Foreground(ui.ColorGit).Render(icon) +
		bar.Foreground(ui.ColorGreen).Render(ansi.Truncate(m.project.branch, branchWidth, "…")+"  ")
}

func (m *model) renderTabs() string {
	nameStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted).Background(ui.ColorBar)
	barKeyStyle := ui.KeyStyle.Background(ui.ColorBar)
	gapStyle := lipgloss.NewStyle().Background(ui.ColorBar)
	items := []struct {
		key   string
		name  string
		count string
		pane  pane
	}{
		{"1", "General", fmt.Sprintf("%d/%d", completedCount(m.general)+completedCount(m.readme), len(m.general)+len(m.readme)), generalPane},
		{"2", "Branches", fmt.Sprintf("%d/%d", completedCount(m.branches), len(m.branches)), branchPane},
		{"3", "Files", m.filesCount(), sourcePane},
	}
	var tabs strings.Builder
	for i, item := range items {
		if i > 0 {
			tabs.WriteString(gapStyle.Render(" "))
		}
		if m.activePane() == item.pane {
			tabs.WriteString(ui.SelectedStyle.Render(" ") + ui.KeyStyle.Background(ui.ColorSelection).Render(item.key) +
				ui.SelectedStyle.Render(" "+item.name+" ") + ui.SelectedDoneStyle.Render(item.count+" "))
		} else {
			tabs.WriteString(nameStyle.Render(" ") + barKeyStyle.Render(item.key) +
				nameStyle.Render(" "+item.name+" ") + nameStyle.Render(item.count+" "))
		}
	}
	return tabs.String()
}

func (m *model) filesCount() string {
	if m.files.Loading() {
		return "…"
	}
	return fmt.Sprint(m.files.Total())
}

var pinnedHints = []ui.KeyHint{{Key: "?", Label: "help"}, {Key: "q", Label: "quit"}}

func (m *model) renderFooter(width int) string {
	if p, ok := m.overlay.(*categoryPrompt); ok {
		return p.footer(width)
	}
	return ui.Footer(m.status, m.footerHints(), pinnedHints, width)
}

func (m *model) footerHints() []ui.KeyHint {
	hints := m.contextHints()
	if m.lastClear != nil {
		hints = append([]ui.KeyHint{{Key: "u", Label: "undo clear"}}, hints...)
	}
	return hints
}

func (m *model) contextHints() []ui.KeyHint {
	back := ui.KeyHint{Key: "←", Label: "back"}
	reload := ui.KeyHint{Key: "r", Label: "reload"}
	index := ui.KeyHint{Key: "i", Label: "all tasks"}
	switch {
	case m.all != nil:
		hints := []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}, {Key: "s", Label: "sort"}, {Key: "c", Label: "category"}, {Key: "a", Label: "add"}, {Key: "b", Label: "branch task"}}
		return append(append(hints, m.clearHint()...), back, reload)
	case m.focus == detailPane && m.viewingMissingBranch():
		return []ui.KeyHint{back}
	case m.viewingMissingBranch():
		hints := []ui.KeyHint{back, {Key: "a", Label: "add"}, {Key: "→", Label: "details"}}
		return append(append(hints, m.clearHint()...), index, reload)
	case m.focus == detailPane && m.readmeSelected():
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "open file"}, back}
	case m.readmeSelected():
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "open file"}, back, {Key: "→", Label: "details"}, index, reload}
	case m.focus == detailPane && m.activePane() == branchPane:
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}, back}
	case m.focus == detailPane:
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}, {Key: "c", Label: "category"}, back}
	case m.focus == sourcePane && m.files.Details():
		return m.files.Hints()
	case m.focus == sourcePane:
		return append(m.files.Hints(), index, reload)
	}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		hints := []ui.KeyHint{{Key: "→", Label: "open"}, {Key: "a", Label: "add"}}
		if m.focus == generalPane {
			hints = append(hints, ui.KeyHint{Key: "b", Label: "branch task"})
		}
		return append(append(hints, m.clearHint()...), index, reload)
	}
	hints := []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}}
	if m.focus == generalPane {
		hints = append(hints, ui.KeyHint{Key: "c", Label: "category"})
	}
	hints = append(hints, ui.KeyHint{Key: "a", Label: "add"})
	if m.focus == generalPane {
		hints = append(hints, ui.KeyHint{Key: "b", Label: "branch task"})
	}
	if len(m.breadcrumb(m.focus)) > 1 {
		hints = append(hints, back)
	}
	hints = append(hints, ui.KeyHint{Key: "→", Label: "details"})
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

// panelStatus is the bar along the bottom of a list or detail panel: a
// warning for a missing branch, otherwise the selected row's status.
func (m *model) panelStatus() string {
	if m.viewingMissingBranch() {
		return ui.StatusBarStyle.Render(missingBranchStatus)
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
	return ui.MutedStyle.Render(fmt.Sprintf("%d of %d done", row.completed, row.count))
}

func taskStatus(t store.Task) string {
	status := lipgloss.NewStyle().Foreground(ui.ColorText).Render("Open")
	if t.Done {
		status = lipgloss.NewStyle().Foreground(ui.ColorGreen).Render("✓ Done")
	}
	priority := ui.MutedStyle.Render("No priority")
	if t.Priority != "" {
		priority = priorityStyle(t.Priority).Render(priorityMark(t.Priority) + " " + t.Priority.Title() + " priority")
	}
	return status + ui.MutedStyle.Render("  ·  ") + priority
}

// readmeTaskStatus names where a README task is, as the Files tab does,
// since it has no priority.
func readmeTaskStatus(t store.Task) string {
	status := lipgloss.NewStyle().Foreground(ui.ColorText).Render("Open")
	if t.Done {
		status = lipgloss.NewStyle().Foreground(ui.ColorGreen).Render("✓ Done")
	}
	return status + ui.MutedStyle.Render(fmt.Sprintf("  ·  %s:%d", readmeGroup, t.Line+1))
}

func (m *model) panelContentHeight(height int) int {
	return ui.ContentHeight(height, m.panelStatus())
}

func (m *model) renderPanel(width, height int, lines []string) string {
	return ui.Panel(width, height, lines, m.panelStatus())
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
		lines = append(lines, ui.Breadcrumb(crumbs, fmt.Sprintf("%d/%d", completed, count), innerWidth), "")
	}
	visible := max(1, m.panelContentHeight(height)-len(lines))
	divider := firstTaskAfterGroups(rows)
	if divider > 0 {
		visible = max(1, visible-1)
	}
	start, end := ui.VisibleRange(cursor, len(rows), visible)
	levelWidth := 0
	for _, row := range rows {
		levelWidth = max(levelWidth, len(ui.LevelLabel(row.todo.Level)))
	}
	if len(rows) == 0 {
		empty := "No tasks"
		if kind == branchPane {
			empty = "No branch TODOs"
		}
		lines = append(lines, ui.MutedStyle.Render(empty))
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
		if rows[i].kind == rowTask && rows[i-1].kind != rowTask {
			return i
		}
	}
	return -1
}

func renderGroupRow(item navigationRow, width int, selected, current bool) string {
	row := ui.GroupRow{Marker: "▸ ", Name: item.name, NoteStyle: ui.MutedStyle.Italic(true), Count: fmt.Sprintf("%d/%d", item.completed, item.count)}
	switch {
	case item.missingGitBranch:
		row.Marker, row.Note = "⚠ ", "missing"
		row.NameStyle = lipgloss.NewStyle().Foreground(ui.ColorHigh)
		row.NoteStyle = row.NameStyle.Italic(true)
		row.KeepColour = item.kind == rowBranch
	case current:
		row.Note = "current"
		row.NameStyle = lipgloss.NewStyle().Foreground(ui.ColorGreen)
		row.KeepColour = item.kind == rowBranch
	case item.kind == rowCategory:
		row.NameStyle = lipgloss.NewStyle().Foreground(ui.ColorPurple)
	}
	return row.Render(width, selected)
}

// renderTaskRow shows a task's priority, or with a levelWidth above 0, a
// column of README levels in its place.
func renderTaskRow(t store.Task, width, levelWidth int, selected bool) string {
	mark, markStyle := priorityMark(t.Priority), ui.MutedStyle
	if t.Priority != "" {
		markStyle = priorityStyle(t.Priority)
	}
	if levelWidth > 0 {
		mark, markStyle = ui.LevelMark(t.Level)
	}
	textStyle := lipgloss.NewStyle().Foreground(ui.ColorStrong)
	if t.Done {
		mark, markStyle, textStyle = "✓", ui.MutedStyle, ui.MutedStyle
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
	title := ansi.Truncate(ui.CleanDisplay(t.Text), max(0, width-reserved), "…")
	suffixStyle, fillStyle := ui.MutedStyle, lipgloss.NewStyle()
	if selected {
		markStyle = markStyle.Background(ui.ColorSelection)
		textStyle = textStyle.Background(ui.ColorSelection)
		if t.Done {
			textStyle = ui.SelectedDoneStyle
		}
		suffixStyle, fillStyle = ui.SelectedDoneStyle, ui.SelectedStyle
	}
	fill := ""
	if selected || suffix != "" {
		fill = strings.Repeat(" ", max(0, width-ansi.StringWidth(mark)-ansi.StringWidth(title)-ansi.StringWidth(suffix)))
	}
	return markStyle.Render(mark) + textStyle.Render(title) + fillStyle.Render(fill) + suffixStyle.Render(suffix)
}

func (m *model) renderDetailPane(width, height int) string {
	innerWidth := max(1, width-4)
	lines := []string{ui.Breadcrumb(m.breadcrumb(detailPane), "", innerWidth), ""}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
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
	return ui.WrapLines([]string{ui.TaskTitleStyle.Render(groupName(row)), "", fmt.Sprintf("%d/%d %s", row.completed, row.count, scope), "", ui.MutedStyle.Render("enter or → to open")}, width)
}

func (m *model) taskDetails(width int) []string {
	t, ok := m.selectedTask()
	if !ok {
		return []string{"", ui.MutedStyle.Render("Select a Markdown task.")}
	}
	result := []string{ui.LeveledTitle(t.Text, t.Level), ""}
	switch {
	case m.readmeSelected():
		result = result[:1]
	case t.Details == "":
		result = append(result, ui.MutedStyle.Render("No details yet"))
	default:
		for line := range strings.SplitSeq(t.Details, "\n") {
			result = append(result, ui.CleanDisplay(line))
		}
	}
	return ui.WrapLines(result, width)
}
