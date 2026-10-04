package dashboard

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

const missingBranchStatus = "⚠ Branch not in Git"

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
		body = m.all.view(m.theme, m.tasks.all, m.branchMissing, width, bodyHeight)
	} else {
		switch m.focus {
		case branchPane:
			body = m.renderNavigationPane(m.rows(branchPane), m.branch.cursor, branchPane, width, bodyHeight)
		case sourcePane:
			body = m.files.View(m.theme, width, bodyHeight)
		case detailPane:
			body = m.renderDetailPane(width, bodyHeight)
		default:
			body = m.renderNavigationPane(m.rows(generalPane), m.general.cursor, generalPane, width, bodyHeight)
		}
	}
	content := header + "\n" + body + "\n" + footer
	if d, ok := m.overlay.(dialog); ok {
		content = placeOver(content, d.view(m.theme, width, height), width, height)
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
	return tabs + lipgloss.NewStyle().Background(m.theme.ColorBar).Render(gap) + project
}

// renderProject shows the repository and branch in at most width cells,
// dropping the repository name before shortening the branch below minBranch.
func (m *model) renderProject(width int) string {
	const minBranch = 8
	if m.project.Branch == "" {
		return ""
	}
	bar := lipgloss.NewStyle().Background(m.theme.ColorBar)
	icon := gitIcon + " "
	repo := ""
	if m.project.Root != "" {
		repo = filepath.Base(m.project.Root) + "  "
	}
	fixed := ansi.StringWidth(icon) + 2
	if width-fixed-ansi.StringWidth(repo) < min(minBranch, ansi.StringWidth(m.project.Branch)) {
		repo = ""
	}
	branchWidth := width - fixed - ansi.StringWidth(repo)
	if branchWidth < min(minBranch, ansi.StringWidth(m.project.Branch)) {
		return ""
	}
	return bar.Foreground(m.theme.ColorText).Render(repo) + bar.Foreground(m.theme.ColorGit).Render(icon) +
		bar.Foreground(m.theme.ColorGreen).Render(ansi.Truncate(m.project.Branch, branchWidth, "…")+"  ")
}

func (m *model) renderTabs() string {
	nameStyle := lipgloss.NewStyle().Foreground(m.theme.ColorMuted).Background(m.theme.ColorBar)
	barKeyStyle := m.theme.KeyStyle.Background(m.theme.ColorBar)
	gapStyle := lipgloss.NewStyle().Background(m.theme.ColorBar)
	items := []struct {
		key   string
		name  string
		count string
		pane  pane
	}{
		{"1", "General", generalCount(m.tasks), generalPane},
		{"2", "Branches", countText(countTasks(m.tasks.branches)), branchPane},
		{"3", "Files", m.filesCount(), sourcePane},
	}
	var tabs strings.Builder
	for i, item := range items {
		if i > 0 {
			tabs.WriteString(gapStyle.Render(" "))
		}
		if m.activePane() == item.pane {
			tabs.WriteString(m.theme.SelectedStyle.Render(" ") + m.theme.KeyStyle.Background(m.theme.ColorSelection).Render(item.key) +
				m.theme.SelectedStyle.Render(" "+item.name+" ") + m.theme.SelectedDoneStyle.Render(item.count+" "))
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
		return p.footer(m.theme, width)
	}
	return m.theme.Footer(m.status, m.footerHints(), pinnedHints, width)
}

func (m *model) footerHints() []ui.KeyHint {
	hints := m.contextHints()
	if m.lastRemoval != nil {
		hints = append([]ui.KeyHint{{Key: "u", Label: "undo"}}, hints...)
	}
	return hints
}

func (m *model) contextHints() []ui.KeyHint {
	back := ui.KeyHint{Key: "←", Label: "back"}
	reload := ui.KeyHint{Key: "r", Label: "reload"}
	index := ui.KeyHint{Key: "i", Label: "all tasks"}
	remove := ui.KeyHint{Key: "⌫", Label: "delete"}
	// A subtask keeps its task's category.
	category := []ui.KeyHint{{Key: "c", Label: "category"}}
	if t, ok := m.selectedTask(); ok && t.Subtask {
		category = nil
	}
	_, readmeTask := m.selectedReadmeTask()
	switch {
	case m.all != nil:
		hints := append([]ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}, {Key: "s", Label: "sort"}}, category...)
		hints = append(hints, remove, ui.KeyHint{Key: "a", Label: "add"}, ui.KeyHint{Key: "b", Label: "branch task"})
		return append(append(hints, m.clearHint()...), back, reload)
	case m.focus == detailPane && readmeTask:
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "open file"}, back}
	case readmeTask:
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "open file"}, back, {Key: "→", Label: "details"}, index, reload}
	case m.readmeSelected():
		return []ui.KeyHint{{Key: "→", Label: "open"}, back, index, reload}
	case m.focus == detailPane && m.activePane() == branchPane:
		return []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}, remove, back}
	case m.focus == detailPane:
		hints := append([]ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}}, category...)
		return append(hints, remove, back)
	case m.focus == sourcePane && m.files.Details():
		return m.files.Hints()
	case m.focus == sourcePane:
		return append(m.files.Hints(), index, reload)
	}
	if row, ok := m.selectedNavigationRow(); ok && !row.isTask() {
		hints := []ui.KeyHint{{Key: "→", Label: "open"}}
		if row.kind != rowReadme {
			hints = append(hints, remove)
		}
		hints = append(hints, ui.KeyHint{Key: "a", Label: "add"})
		if m.focus == generalPane {
			hints = append(hints, ui.KeyHint{Key: "b", Label: "branch task"})
		}
		return append(append(hints, m.clearHint()...), index, reload)
	}
	hints := []ui.KeyHint{{Key: "d", Label: "done"}, {Key: "e", Label: "edit"}, {Key: "p", Label: "priority"}}
	if m.focus == generalPane {
		hints = append(hints, category...)
	}
	hints = append(hints, remove, ui.KeyHint{Key: "a", Label: "add"})
	if m.focus == generalPane {
		hints = append(hints, ui.KeyHint{Key: "b", Label: "branch task"})
	}
	if len(m.breadcrumb(m.focus)) > 1 {
		hints = append(hints, back)
	}
	hints = append(hints, ui.KeyHint{Key: "→", Label: "details"})
	return append(append(hints, m.clearHint()...), index, reload)
}

// viewingMissingBranch is true inside a branch Git doesn't have, which may be
// gone or not created yet.
func (m *model) viewingMissingBranch() bool {
	return m.activePane() == branchPane && m.branchMissing(m.branch.open.name)
}

// breadcrumb names where a pane is; a single entry means its top level.
func (m *model) breadcrumb(kind pane) []string {
	switch kind {
	case branchPane:
		if m.branch.open != (group{}) {
			return []string{"Branches", branchIcon + " " + m.branch.open.name}
		}
		return []string{"Branches"}
	case detailPane:
		return append(m.breadcrumb(m.detailFrom), "Details")
	}
	switch m.general.open.kind {
	case rowReadme:
		return []string{"General", readmeGroup}
	case rowReadmeHeading:
		return []string{"General", readmeGroup, m.general.open.name}
	case rowCategory:
		return []string{"General", "@" + m.general.open.name}
	}
	return []string{"General"}
}

// panelStatus is the bar along the bottom of a list or detail panel: the
// selected row's status, then a warning inside a branch Git doesn't have.
func (m *model) panelStatus() string {
	status := m.selectionStatus()
	if !m.viewingMissingBranch() {
		return status
	}
	warning := m.theme.StatusBarStyle.Render(missingBranchStatus)
	if status == "" {
		return warning
	}
	return status + m.theme.MutedStyle.Render("  ·  ") + warning
}

// selectionStatus describes the selected row: a task's state, with how
// many of its subtasks are done, or how much of a group is done.
func (m *model) selectionStatus() string {
	if readme, ok := m.selectedReadmeTask(); ok {
		return readmeTaskStatus(m.theme, readme) + m.subtaskStatus()
	}
	row, ok := m.selectedNavigationRow()
	switch {
	case !ok:
		return ""
	case row.kind == rowTask:
		return taskStatus(m.theme, row.todo) + m.subtaskStatus()
	}
	return m.theme.MutedStyle.Render(fmt.Sprintf("%d of %d done", row.completed, row.count))
}

// subtaskStatus says how many of the selected task's subtasks are done, or
// nothing for a task without any.
func (m *model) subtaskStatus() string {
	done, total := m.selectedSubtasks()
	if total == 0 {
		return ""
	}
	return m.theme.MutedStyle.Render(fmt.Sprintf("  ·  %d of %d subtasks done", done, total))
}

// selectedSubtasks is how many of the selected task's subtasks are done, and
// how many it has: the subtask rows that follow it. A subtask has none.
func (m *model) selectedSubtasks() (done, total int) {
	if m.all != nil {
		tasks := m.all.sorted(m.tasks.all)
		if m.all.cursor >= len(tasks) || tasks[m.all.cursor].Subtask {
			return 0, 0
		}
		for _, t := range tasks[m.all.cursor+1:] {
			if !t.Subtask {
				break
			}
			total++
			if t.Done {
				done++
			}
		}
		return done, total
	}
	l := m.list(m.activePane())
	if l == nil {
		return 0, 0
	}
	rows := m.rows(m.activePane())
	if l.cursor >= len(rows) || rows[l.cursor].isTask() && rows[l.cursor].subtask() {
		return 0, 0
	}
	for _, row := range rows[l.cursor+1:] {
		if !row.isTask() || !row.subtask() {
			break
		}
		total++
		if row.done() {
			done++
		}
	}
	return done, total
}

func taskStatus(theme ui.Theme, t store.Task) string {
	status := lipgloss.NewStyle().Foreground(theme.ColorText).Render("Open")
	if t.Done {
		status = lipgloss.NewStyle().Foreground(theme.ColorGreen).Render("✓ Done")
	}
	priority := theme.MutedStyle.Render("No priority")
	if t.Priority != store.PriorityNone {
		priority = priorityStyle(theme, t.Priority).Render(priorityMark(t.Priority) + " " + t.Priority.Title() + " priority")
	}
	return status + theme.MutedStyle.Render("  ·  ") + priority
}

// readmeTaskStatus names where a README task is, as the Files tab does,
// since it has no priority.
func readmeTaskStatus(theme ui.Theme, t store.ReadmeTask) string {
	status := lipgloss.NewStyle().Foreground(theme.ColorText).Render("Open")
	if t.Done {
		status = lipgloss.NewStyle().Foreground(theme.ColorGreen).Render("✓ Done")
	}
	return status + theme.MutedStyle.Render(fmt.Sprintf("  ·  %s:%d", readmeGroup, t.Line+1))
}

func (m *model) panelContentHeight(height int) int {
	return ui.ContentHeight(height, m.panelStatus())
}

func (m *model) renderPanel(width, height int, lines []string) string {
	return m.theme.Panel(width, height, lines, m.panelStatus())
}

func (m *model) renderNavigationPane(rows []navigationRow, cursor int, kind pane, width, height int) string {
	innerWidth := max(1, width-4)
	var lines []string
	if crumbs := m.breadcrumb(kind); len(crumbs) > 1 {
		completed, count := 0, 0
		for _, row := range rows {
			switch {
			case !row.isTask():
				completed, count = completed+row.completed, count+row.count
			case row.subtask():
			case row.done():
				completed, count = completed+1, count+1
			default:
				count++
			}
		}
		lines = append(lines, m.theme.Breadcrumb(crumbs, fmt.Sprintf("%d/%d", completed, count), innerWidth), "")
	}
	visible := max(1, m.panelContentHeight(height)-len(lines))
	divider := firstTaskAfterGroups(rows)
	if divider > 0 {
		visible = max(1, visible-1)
	}
	start, end := ui.VisibleRange(cursor, len(rows), visible)
	levelWidth := 0
	for _, row := range rows {
		if row.kind == rowReadmeTask {
			levelWidth = max(levelWidth, len(ui.LevelLabel(row.readme.Level)))
		}
	}
	if len(rows) == 0 {
		empty := "No tasks"
		if kind == branchPane {
			empty = "No branch TODOs"
		}
		lines = append(lines, m.theme.MutedStyle.Render(empty))
	}
	for i := start; i < end; i++ {
		item := rows[i]
		selected := i == cursor && (m.focus == kind || (m.focus == detailPane && m.detailFrom == kind))
		if i == divider && i > start {
			lines = append(lines, "")
		}
		switch item.kind {
		case rowTask:
			lines = append(lines, renderTaskRow(m.theme, item, innerWidth, selected))
		case rowReadmeTask:
			lines = append(lines, renderReadmeTaskRow(m.theme, item, innerWidth, levelWidth, selected))
		default:
			lines = append(lines, renderGroupRow(m.theme, item, innerWidth, selected, item.kind == rowBranch && item.name == m.project.Branch))
		}
	}
	return m.renderPanel(width, height, lines)
}

// firstTaskAfterGroups is the row where loose tasks follow categories or
// branches, or -1 when the list is not mixed.
func firstTaskAfterGroups(rows []navigationRow) int {
	for i := 1; i < len(rows); i++ {
		if rows[i].isTask() && !rows[i-1].isTask() {
			return i
		}
	}
	return -1
}

func renderGroupRow(theme ui.Theme, item navigationRow, width int, selected, current bool) string {
	row := ui.GroupRow{Marker: "▸ ", Name: item.name, NoteStyle: theme.MutedStyle.Italic(true), Count: fmt.Sprintf("%d/%d", item.completed, item.count)}
	switch {
	case item.missingGitBranch:
		row.Marker, row.Note = "⚠ ", "not in Git"
		row.NameStyle = lipgloss.NewStyle().Foreground(theme.ColorHigh)
		row.NoteStyle = row.NameStyle.Italic(true)
		row.KeepColour = item.kind == rowBranch
	case current:
		row.Note = "current"
		row.NameStyle = lipgloss.NewStyle().Foreground(theme.ColorGreen)
		row.KeepColour = item.kind == rowBranch
	case item.kind == rowCategory:
		row.NameStyle = lipgloss.NewStyle().Foreground(theme.ColorPurple)
	}
	return row.Render(theme, width, selected)
}

// renderTaskRow shows a todo.md task row with its priority. A subtask is
// indented, and greyed out while its task is done.
func renderTaskRow(theme ui.Theme, row navigationRow, width int, selected bool) string {
	t := row.todo
	mark, markStyle := priorityMark(t.Priority), theme.MutedStyle
	if t.Priority != store.PriorityNone {
		markStyle = priorityStyle(theme, t.Priority)
	}
	suffix := ""
	if strings.TrimSpace(t.Details) != "" {
		suffix = "⋯"
	}
	return taskRow{indent: rowIndent(t.Subtask), mark: mark, markStyle: markStyle, text: t.Text, suffix: suffix, done: t.Done, muted: row.parentDone}.Render(theme, width, selected)
}

// rowIndent is the indent of a subtask's row under its task's.
func rowIndent(subtask bool) int {
	if subtask {
		return subtaskIndent
	}
	return 0
}

// generalCount is General's count: its tasks and README.md's, done of all.
func generalCount(tasks taskSet) string {
	done, total := countTasks(tasks.general)
	readmeDone, readmeTotal := countReadmeTasks(tasks.readme)
	return countText(done+readmeDone, total+readmeTotal)
}

// countText writes a count as "done/total".
func countText(done, total int) string {
	return fmt.Sprintf("%d/%d", done, total)
}

// renderReadmeTaskRow shows a README task row with its level in a column
// levelWidth wide. A list without levels has a levelWidth of 0, and shows the
// open bullet instead. A subtask is indented, and greyed out while its parent
// is done.
func renderReadmeTaskRow(theme ui.Theme, row navigationRow, width, levelWidth int, selected bool) string {
	t := row.readme
	mark, markStyle := priorityMark(store.PriorityNone), theme.MutedStyle
	if levelWidth > 0 {
		mark, markStyle = theme.LevelMark(t.Level)
	}
	suffix := ""
	if t.Details != "" {
		suffix = "⋯"
	}
	return taskRow{indent: rowIndent(t.Subtask), mark: mark, markStyle: markStyle, markWidth: levelWidth, text: t.Text, suffix: suffix, done: t.Done, muted: row.parentDone}.Render(theme, width, selected)
}

// subtaskIndent is how far a subtask's row is indented under its parent's.
const subtaskIndent = 2

// taskRow is a row for a task: its mark, then its text, with a suffix at the
// right edge.
type taskRow struct {
	indent    int // cells before the mark
	mark      string
	markStyle lipgloss.Style
	markWidth int // the width the mark is padded to, to line up a column of marks
	text      string
	suffix    string
	done      bool
	muted     bool // greyed out as a done task is, but keeping its mark
}

// Render draws the row in width cells. A done task shows a check mark and
// muted text instead of its mark.
func (r taskRow) Render(theme ui.Theme, width int, selected bool) string {
	mark, markStyle := r.mark, r.markStyle
	textStyle := lipgloss.NewStyle().Foreground(theme.ColorStrong)
	switch {
	case r.done:
		mark, markStyle, textStyle = "✓", theme.MutedStyle, theme.MutedStyle
	case r.muted:
		markStyle, textStyle = theme.MutedStyle, theme.MutedStyle
	}
	mark = strings.Repeat(" ", r.indent) + mark + strings.Repeat(" ", max(0, r.markWidth-ansi.StringWidth(mark))+1)
	reserved := ansi.StringWidth(mark)
	if r.suffix != "" {
		reserved += ansi.StringWidth(r.suffix) + 2
	}
	suffixStyle, fillStyle := theme.MutedStyle, lipgloss.NewStyle()
	if selected {
		markStyle = markStyle.Background(theme.ColorSelection)
		textStyle = textStyle.Background(theme.ColorSelection)
		if r.done || r.muted {
			textStyle = theme.SelectedDoneStyle
		}
		suffixStyle, fillStyle = theme.SelectedDoneStyle, theme.SelectedStyle
	}
	title := ansi.Truncate(theme.Inline(r.text, textStyle), max(0, width-reserved), "…")
	fill := ""
	if selected || r.suffix != "" {
		fill = strings.Repeat(" ", max(0, width-ansi.StringWidth(mark)-ansi.StringWidth(title)-ansi.StringWidth(r.suffix)))
	}
	return markStyle.Render(mark) + title + fillStyle.Render(fill) + suffixStyle.Render(r.suffix)
}

func (m *model) renderDetailPane(width, height int) string {
	innerWidth := max(1, width-4)
	lines := []string{m.theme.Breadcrumb(m.breadcrumb(detailPane), "", innerWidth), ""}
	if row, ok := m.selectedNavigationRow(); ok && !row.isTask() {
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
	case rowReadmeHeading:
		scope = "tasks under this heading"
	}
	return ui.WrapLines([]string{m.theme.TaskTitleStyle.Render(groupName(row)), "", fmt.Sprintf("%d/%d %s", row.completed, row.count, scope), "", m.theme.MutedStyle.Render("enter or → to open")}, width)
}

func (m *model) taskDetails(width int) []string {
	if readme, ok := m.selectedReadmeTask(); ok {
		result := []string{m.theme.LeveledTitle(m.theme.Inline(readme.Text, m.theme.TaskTitleStyle), readme.Level)}
		if readme.Details != "" {
			result = append(append(result, ""), m.theme.MarkdownLines(readme.Details)...)
		}
		return ui.WrapLines(result, width)
	}
	t, ok := m.selectedTask()
	if !ok {
		return []string{"", m.theme.MutedStyle.Render("Select a Markdown task.")}
	}
	result := []string{m.theme.Inline(t.Text, m.theme.TaskTitleStyle), ""}
	if t.Details == "" {
		result = append(result, m.theme.MutedStyle.Render("No details yet"))
	} else {
		result = append(result, m.theme.MarkdownLines(t.Details)...)
	}
	return ui.WrapLines(result, width)
}
