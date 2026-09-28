package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	colorText      = lipgloss.Color("#DCE4EF")
	colorMuted     = lipgloss.Color("#8190A5")
	colorBorder    = lipgloss.Color("#526177")
	colorFocus     = lipgloss.Color("#F4D35E")
	colorBlue      = lipgloss.Color("#2457A6")
	colorGreen     = lipgloss.Color("#80C99B")
	colorPurple    = lipgloss.Color("#B7A4EB")
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorFocus)
	taskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle     = lipgloss.NewStyle().Foreground(colorMuted)
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(colorBlue)
)

func (m model) View() tea.View {
	width, height := m.width, m.height
	if width < 1 {
		width = 100
	}
	if height < 1 {
		height = 30
	}
	if width < 56 || height < 16 || (width < 78 && height < 19) {
		v := tea.NewView("todo · enlarge your terminal\nq quit")
		v.AltScreen = true
		return v
	}

	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	bodyHeight := height - 2
	generalRows, branchRows := m.generalRows(), m.branchRows()
	var body string
	if width >= 78 {
		leftWidth := width * 36 / 100
		if leftWidth < 33 {
			leftWidth = 33
		}
		if leftWidth > 50 {
			leftWidth = 50
		}
		rightWidth := width - leftWidth - 1
		paneHeights := leftPaneHeights(bodyHeight, [3]int{len(generalRows), len(branchRows), len(m.source)})
		left := lipgloss.JoinVertical(lipgloss.Left,
			m.renderNavigationPane(m.generalTitle(), generalRows, m.generalCursor, generalPane, leftWidth, paneHeights[0]),
			m.renderNavigationPane(m.branchTitle(), branchRows, m.branchCursor, branchPane, leftWidth, paneHeights[1]),
			m.renderSourcePane(leftWidth, paneHeights[2]),
		)
		right := m.renderDetailPane(rightWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	} else {
		generalHeight, branchHeight := 4, 4
		sourceHeight := 4
		detailHeight := bodyHeight - generalHeight - branchHeight - sourceHeight
		body = lipgloss.JoinVertical(lipgloss.Left,
			m.renderNavigationPane(m.generalTitle(), generalRows, m.generalCursor, generalPane, width, generalHeight),
			m.renderNavigationPane(m.branchTitle(), branchRows, m.branchCursor, branchPane, width, branchHeight),
			m.renderSourcePane(width, sourceHeight),
			m.renderDetailPane(width, detailHeight),
		)
	}
	content := header + "\n" + body + "\n" + footer
	if m.modal != nil {
		modalWidth, modalHeight := m.modal.dimensions(width, height)
		x, y := (width-modalWidth)/2, (height-modalHeight)/2
		content = lipgloss.NewCompositor(
			lipgloss.NewLayer(content),
			lipgloss.NewLayer(m.modal.render(modalWidth, modalHeight)).X(x).Y(y).Z(1),
		).Render()
		v := tea.NewView(content)
		v.AltScreen = true
		v.Cursor = m.modal.cursor(x, y)
		return v
	}
	v := tea.NewView(content)
	v.AltScreen = true
	if m.inputMode != "" {
		v.Cursor = m.input.Cursor()
		if v.Cursor != nil {
			index := strings.LastIndex(content, m.input.View())
			if index >= 0 {
				v.Cursor.Y += strings.Count(content[:index], "\n")
			}
		}
	}
	return v
}

// leftPaneHeights keeps the three lists balanced unless an occupied list needs
// more rows. Empty lists can then shrink to their heading and empty-state row.
func leftPaneHeights(total int, counts [3]int) [3]int {
	heights := [3]int{total / 3, total / 3, total / 3}
	for i := 0; i < total%3; i++ {
		heights[i]++
	}

	needsSpace := false
	for i, count := range counts {
		if count > 0 && count+3 > heights[i] {
			needsSpace = true
			break
		}
	}
	if !needsSpace {
		return heights
	}

	free := 0
	for i, count := range counts {
		if count == 0 && heights[i] > 4 {
			free += heights[i] - 4
			heights[i] = 4
		}
	}
	for free > 0 {
		chosen, largestShortfall := -1, 0
		for i, count := range counts {
			if count == 0 {
				continue
			}
			if shortfall := count + 3 - heights[i]; shortfall > largestShortfall {
				chosen, largestShortfall = i, shortfall
			}
		}
		if chosen == -1 {
			for i, count := range counts {
				if count > 0 && (chosen == -1 || heights[i] < heights[chosen]) {
					chosen = i
				}
			}
		}
		if chosen == -1 {
			break
		}
		heights[chosen]++
		free--
	}
	return heights
}

func (m model) renderHeader(width int) string {
	repo := "No Git repository"
	if m.project.root != "" {
		repo = filepath.Base(m.project.root)
	}
	branch := m.project.branch
	if branch == "" {
		branch = "no branch"
	}
	line := fmt.Sprintf("  TODO  /  %s  /  %s", repo, branch)
	count := fmt.Sprintf("  %d general · %d branch · %d file", len(m.general), len(m.branches), len(m.source))
	available := width - ansi.StringWidth(count)
	if available < 1 {
		available = 1
	}
	line = ansi.Truncate(line, available, "…")
	spacer := strings.Repeat(" ", max(0, width-ansi.StringWidth(line)-ansi.StringWidth(count)))
	return lipgloss.NewStyle().Width(width).Foreground(colorText).Background(lipgloss.Color("#17253A")).Render(line + spacer + count)
}

func (m model) renderFooter(width int) string {
	if m.inputMode != "" {
		return ansi.Truncate(m.input.View()+"  Enter save · Esc cancel", width, "…")
	}
	hints := "1 General · 2 Branches · 3 File TODOs · → Open/Details · ← Back · a/b Add · q Quit"
	if width < 80 {
		hints = "1 General · 2 Branches · 3 Files · → Open · ← Back"
	}
	if m.status != "" {
		statusHints := hints
		if width < 100 {
			statusHints = "1 General · 2 Branches · 3 Files"
		}
		available := max(0, width-ansi.StringWidth(statusHints)-3)
		if available > 5 {
			return mutedStyle.Render(ansi.Truncate(m.status, available, "…") + " · " + statusHints)
		}
	}
	return mutedStyle.Render(ansi.Truncate(hints, width, "…"))
}

func (m model) generalTitle() string {
	if m.generalLabel != "" {
		return "General · @" + m.generalLabel
	}
	return "General"
}

func (m model) branchTitle() string {
	if m.branchFilter != "" {
		return "Branch · " + m.branchFilter
	}
	return "Branches"
}

func (m model) panelStyle(focused bool, width, height int) lipgloss.Style {
	border := colorBorder
	if focused {
		border = colorFocus
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(border)
}

func (m model) renderNavigationPane(title string, rows []navigationRow, cursor int, kind pane, width, height int) string {
	innerWidth := max(1, width-4)
	count := len(rows)
	if (kind == generalPane && m.generalLabel == "") || (kind == branchPane && m.branchFilter != "") {
		count = 0
		for _, row := range rows {
			if row.kind == rowTask {
				count++
			}
		}
	}
	heading := fmt.Sprintf("%s  %d", title, count)
	lines := []string{titleStyle.Render(ansi.Truncate(heading, innerWidth, "…"))}
	visible := max(1, height-3)
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
		row := ""
		switch item.kind {
		case rowLabel:
			row = fmt.Sprintf("@%s  %d ›", item.name, item.count)
		case rowBranchLabel:
			row = fmt.Sprintf("@%s  %d", item.name, item.count)
		case rowBranch:
			marker := "  "
			if item.name == m.project.branch {
				marker = "* "
			}
			row = fmt.Sprintf("%s%s  %d ›", marker, item.name, item.count)
		default:
			mark := ""
			if item.todo.done {
				mark = "✓ "
			}
			prefix := ""
			if item.todo.priority == "high" {
				prefix = "! "
			}
			if (kind == generalPane || kind == branchPane) && taskLabel(item.todo) != "" {
				prefix = "  " + prefix
			}
			row = mark + prefix + cleanDisplay(item.todo.text)
			if kind == generalPane && item.todo.branch != "" {
				row += "  (" + item.todo.branch + ")"
			}
		}
		row = ansi.Truncate(row, innerWidth, "…")
		if i == cursor && (m.focus == kind || (m.focus == detailPane && m.detailFrom == kind)) {
			lines = append(lines, selectedStyle.Width(innerWidth).Render(row))
		} else if item.kind == rowTask && item.todo.done {
			lines = append(lines, mutedStyle.Render(row))
		} else if item.kind == rowLabel || item.kind == rowBranchLabel {
			lines = append(lines, lipgloss.NewStyle().Foreground(colorPurple).Render(row))
		} else if item.kind == rowBranch && item.name == m.project.branch {
			lines = append(lines, lipgloss.NewStyle().Foreground(colorGreen).Render(row))
		} else if item.kind == rowTask {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return m.panelStyle(m.focus == kind, width, height).Render(strings.Join(lines, "\n"))
}

func (m model) renderSourcePane(width, height int) string {
	innerWidth := max(1, width-4)
	heading := fmt.Sprintf("File TODOs  %d", len(m.source))
	if m.sourceLoading {
		heading += "  ↻"
	}
	lines := []string{titleStyle.Render(ansi.Truncate(heading, innerWidth, "…"))}
	visible := max(1, height-3)
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
		row := fmt.Sprintf("%s:%d  %s", item.path, item.line, cleanDisplay(item.text))
		row = ansi.Truncate(row, innerWidth, "…")
		if i == m.sourceCursor && (m.focus == sourcePane || (m.focus == detailPane && m.detailFrom == sourcePane)) {
			lines = append(lines, selectedStyle.Width(innerWidth).Render(row))
		} else {
			lines = append(lines, row)
		}
	}
	return m.panelStyle(m.focus == sourcePane, width, height).Render(strings.Join(lines, "\n"))
}

func (m model) renderDetailPane(width, height int) string {
	innerWidth := max(1, width-4)
	lines := []string{titleStyle.Render("Details")}
	if height < 8 {
		lines = append(lines, m.compactDetails()...)
	} else if m.activePane() == sourcePane {
		lines = append(lines, m.sourceDetails(innerWidth)...)
	} else if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		lines = append(lines, m.groupDetails(row, innerWidth)...)
	} else {
		lines = append(lines, m.taskDetails(innerWidth)...)
	}
	maxLines := max(1, height-2)
	var footer []string
	if m.focus == detailPane {
		if m.activePane() == sourcePane {
			footer = []string{"Enter / (e): open file"}
		} else if _, ok := m.selectedTask(); ok {
			help := "Space/Enter: done · (e)dit · (p)riority · (l)abel"
			if ansi.StringWidth(help) <= innerWidth {
				footer = []string{help}
			} else {
				footer = []string{"Space/Enter: done", "(e)dit · (p)riority · (l)abel"}
			}
		}
	}
	separator := len(footer) > 0 && maxLines >= len(footer)+6
	contentLines := max(1, maxLines-len(footer))
	if separator {
		contentLines--
	}
	start := min(m.detailScroll, max(0, len(lines)-contentLines))
	lines = lines[start:min(len(lines), start+contentLines)]
	for len(lines) < contentLines {
		lines = append(lines, "")
	}
	if separator {
		lines = append(lines, "")
	}
	for _, item := range footer {
		lines = append(lines, mutedStyle.Render(item))
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, innerWidth, "…")
	}
	return m.panelStyle(m.focus == detailPane, width, height).Render(strings.Join(lines, "\n"))
}

func (m model) compactDetails() []string {
	if m.activePane() == sourcePane {
		if m.sourceCursor >= len(m.source) {
			return []string{"No file selected"}
		}
		item := m.source[m.sourceCursor]
		return []string{item.path + ":" + fmt.Sprint(item.line), cleanDisplay(item.text)}
	}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		return []string{groupName(row), fmt.Sprintf("%d tasks · Enter/→ open", row.count)}
	}
	t, ok := m.selectedTask()
	if !ok {
		return []string{"No task selected"}
	}
	if t.details != "" {
		return []string{cleanDisplay(t.text), cleanDisplay(strings.Split(t.details, "\n")[0])}
	}
	mark := "open"
	if t.done {
		mark = "done"
	}
	return []string{cleanDisplay(t.text), mark + " · " + taskLocation(t)}
}

func groupName(row navigationRow) string {
	if row.kind == rowLabel || row.kind == rowBranchLabel {
		return "@" + row.name
	}
	return row.name
}

func (m model) groupDetails(row navigationRow, width int) []string {
	scope := "tasks across General and branches"
	if row.kind == rowBranch {
		scope = "branch tasks"
	} else if row.kind == rowBranchLabel {
		scope = "tasks on this branch"
	}
	return wrapLines([]string{taskTitleStyle.Render(groupName(row)), "", fmt.Sprintf("%d %s", row.count, scope), "", mutedStyle.Render("Enter or → to open")}, width)
}

func (m model) taskDetails(width int) []string {
	t, ok := m.selectedTask()
	if !ok {
		return []string{"", mutedStyle.Render("Select a Markdown task.")}
	}
	status := "Open"
	if t.done {
		status = "Complete"
	}
	priority := "None"
	if t.priority != "" {
		priority = strings.ToUpper(t.priority[:1]) + t.priority[1:]
	}
	label := "None"
	if len(t.labels) > 0 {
		label = "@" + t.labels[0]
	}
	result := []string{taskTitleStyle.Render(cleanDisplay(t.text)), ""}
	if t.details == "" {
		result = append(result, mutedStyle.Render("No details yet"))
	} else {
		for _, line := range strings.Split(t.details, "\n") {
			result = append(result, cleanDisplay(line))
		}
	}
	result = append(result, "", mutedStyle.Render("Status    "+status), mutedStyle.Render("Scope     "+taskLocation(t)),
		mutedStyle.Render("Priority  "+priority), mutedStyle.Render("Label     "+label))
	return wrapLines(result, width)
}

func (m model) sourceDetails(width int) []string {
	if m.sourceCursor >= len(m.source) {
		if m.sourceError != "" {
			return wrapLines([]string{"", "Scan failed", m.sourceError, "", "Press r to try again"}, width)
		}
		return []string{"", mutedStyle.Render("No file TODO selected.")}
	}
	item := m.source[m.sourceCursor]
	result := []string{"", mutedStyle.Render("FILE TODO"), item.path + ":" + fmt.Sprint(item.line),
		"", cleanDisplay(item.text), "", mutedStyle.Render("CONTEXT")}
	if m.previewPath != item.path || m.previewLine != item.line {
		result = append(result, mutedStyle.Render("Loading preview…"))
	} else if m.previewError != "" {
		result = append(result, mutedStyle.Render(m.previewError))
	} else {
		for _, line := range m.preview {
			prefix := fmt.Sprintf("%4d │ ", line.number)
			row := prefix + cleanDisplay(line.text)
			if line.number == item.line {
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
	end := start + height
	if end > total {
		end = total
	}
	return start, end
}
