package main

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
		bodyHeight--
		switch m.focus {
		case branchPane:
			body = m.renderNavigationPane(m.branchTitle(), m.branchRows(), m.branchCursor, branchPane, width, bodyHeight)
		case sourcePane:
			body = m.renderSourcePane(width, bodyHeight)
		case detailPane:
			body = m.renderDetailPane(width, bodyHeight)
		default:
			body = m.renderNavigationPane(m.generalTitle(), m.generalRows(), m.generalCursor, generalPane, width, bodyHeight)
		}
		header += "\n" + m.renderTabs(width)
	}
	content := header + "\n" + body + "\n" + footer
	if m.modal != nil {
		modalWidth, modalHeight := m.modal.dimensions(width, height)
		x, y := (width-modalWidth)/2, (height-modalHeight)/2
		content = lipgloss.NewCompositor(
			lipgloss.NewLayer(content),
			lipgloss.NewLayer(m.modal.render(modalWidth, modalHeight)).X(x).Y(y).Z(1),
		).Render()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *model) renderHeader(width int) string {
	line := "  To Do"
	count := fmt.Sprintf("  %d/%d general · %d/%d branch · %d files",
		completedCount(m.general), len(m.general), completedCount(m.branches), len(m.branches), len(m.source))
	available := max(width-ansi.StringWidth(count), 1)
	line = ansi.Truncate(line, available, "…")
	spacer := strings.Repeat(" ", max(0, width-ansi.StringWidth(line)-ansi.StringWidth(count)))
	return lipgloss.NewStyle().Width(width).Foreground(colorText).Background(colorBar).Render(line + spacer + count)
}

func (m *model) renderTabs(width int) string {
	barColor := colorBar
	inactiveStyle := lipgloss.NewStyle().Foreground(colorMuted).Background(barColor)
	gapStyle := lipgloss.NewStyle().Background(barColor)
	items := []struct {
		name string
		pane pane
	}{
		{"1 General", generalPane},
		{"2 Branches", branchPane},
		{"3 Files", sourcePane},
	}
	var tabs strings.Builder
	for _, item := range items {
		tab := " " + item.name + " "
		if m.activePane() == item.pane {
			tabs.WriteString(selectedStyle.Render(tab))
		} else {
			tabs.WriteString(inactiveStyle.Render(tab))
		}
		tabs.WriteString(gapStyle.Render(" "))
	}
	line := ansi.Truncate(tabs.String(), width, "…")
	return line + gapStyle.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(line))))
}

func (m *model) renderFooter(width int) string {
	if m.categoryInput {
		return ansi.Truncate(m.input.View()+"  enter save · esc cancel", width, "…")
	}
	if m.indexMode {
		hints := "i/esc back · (d)one/space · (e)dit/enter · (s)ort · (p)riority · (c)ategory · (b)ranch add · (r)eload · (q)uit"
		if width < 80 {
			hints = "s sort · p priority · c category · e edit · d done"
		}
		if m.status != "" {
			return mutedStyle.Render(ansi.Truncate(m.status, width, "…"))
		}
		return mutedStyle.Render(ansi.Truncate(hints, width, "…"))
	}
	hints := m.footerHints(width < 80)
	if m.status != "" {
		available := max(0, width-ansi.StringWidth(hints)-3)
		if available > 5 {
			return mutedStyle.Render(ansi.Truncate(m.status, available, "…") + " · " + hints)
		}
		return mutedStyle.Render(ansi.Truncate(m.status, width, "…"))
	}
	return mutedStyle.Render(ansi.Truncate(hints, width, "…"))
}

func (m *model) footerHints(narrow bool) string {
	switch {
	case m.focus == detailPane && m.activePane() == sourcePane:
		if narrow {
			return "←/esc back · (e)dit/enter file"
		}
		return "←/esc back · (e)dit/enter file · (q)uit"
	case m.focus == detailPane && m.activePane() == branchPane:
		if narrow {
			return "esc · (d)one/space · (e)dit/enter · (p)riority"
		}
		return "←/esc back · (d)one/space · (e)dit/enter · (p)riority · (q)uit"
	case m.focus == detailPane:
		if narrow {
			return "esc · (d)one/space · (e)dit/enter · p · (c)ategory"
		}
		return "←/esc back · (d)one/space · (e)dit/enter · (p)riority · (c)ategory · (q)uit"
	case m.focus == sourcePane:
		if narrow {
			return "→ details · (e)dit/enter file · (i)ndex"
		}
		return "→ details · (e)dit/enter file · (i)ndex · (r)eload · (q)uit"
	case m.focus == branchPane:
		if narrow {
			return "(a)dd · (d)one/space · (e)dit/enter · (p)riority"
		}
		return "→ open · (d)one/space · (e)dit/enter · (p)riority · (a)dd · (b)ranch add · (i)ndex · (r)eload · (q)uit"
	case narrow:
		return "(d)one/space · (e)dit/enter · (p)riority · (c)ategory"
	default:
		return "→ details · (d)one/space · (e)dit/enter · (p)riority · (c)ategory · (a)dd · (b)ranch add · (i)ndex · (r)eload · (q)uit"
	}
}

func (m *model) generalTitle() string {
	if m.generalCategory != "" {
		return "General · @" + m.generalCategory
	}
	return "General"
}

func (m *model) branchTitle() string {
	if m.branchFilter != "" {
		return "Git Branches · " + m.branchFilter
	}
	return "Git Branches"
}

func (m *model) panelStyle(focused bool, width, height int) lipgloss.Style {
	border := colorBorder
	if focused {
		border = colorFocus
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(border)
}

func (m *model) panelStatus() string {
	if m.activePane() == branchPane && m.branchMissing(m.branchFilter) {
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

func (m *model) renderPanel(kind pane, width, height int, lines []string) string {
	if status := m.panelStatus(); status != "" {
		contentHeight := m.panelContentHeight(height)
		lines = lines[:min(len(lines), contentHeight)]
		for len(lines) < contentHeight {
			lines = append(lines, "")
		}
		innerWidth := max(1, width-4)
		lines = append(lines, statusBarStyle.Width(innerWidth).Render(ansi.Truncate(status, innerWidth, "…")))
	}
	return m.panelStyle(m.focus == kind, width, height).Render(strings.Join(lines, "\n"))
}

func (m *model) renderNavigationPane(title string, rows []navigationRow, cursor int, kind pane, width, height int) string {
	innerWidth := max(1, width-4)
	completed, count := 0, 0
	switch {
	case kind == generalPane && m.generalCategory == "":
		completed, count = completedCount(m.general), len(m.general)
	case kind == branchPane && m.branchFilter == "":
		completed, count = completedCount(m.branches), len(m.branches)
	default:
		for _, row := range rows {
			if row.kind == rowTask {
				count++
				if row.todo.done {
					completed++
				}
			}
		}
	}
	heading := fmt.Sprintf("%s  %d/%d", title, completed, count)
	lines := []string{titleStyle.Render(ansi.Truncate(heading, innerWidth, "…")), ""}
	visible := max(1, m.panelContentHeight(height)-2)
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
		if item.kind == rowTask {
			lines = append(lines, renderTaskRow(item.todo, innerWidth, selected))
			continue
		}
		if item.kind == rowBranch && item.missingGitBranch {
			lines = append(lines, renderMissingBranchRow(item, innerWidth, selected))
			continue
		}
		row := ""
		switch item.kind {
		case rowCategory:
			row = fmt.Sprintf("@%s  %d/%d ›", item.name, item.completed, item.count)
		case rowBranch:
			marker := "  "
			if item.name == m.project.branch {
				marker = "* "
			}
			row = fmt.Sprintf("%s%s  %d/%d ›", marker, item.name, item.completed, item.count)
		}
		row = ansi.Truncate(row, innerWidth, "…")
		switch {
		case selected:
			lines = append(lines, selectedStyle.Width(innerWidth).Render(row))
		case item.kind == rowCategory:
			lines = append(lines, lipgloss.NewStyle().Foreground(colorPurple).Render(row))
		case item.kind == rowBranch && item.name == m.project.branch:
			lines = append(lines, lipgloss.NewStyle().Foreground(colorGreen).Render(row))
		default:
			lines = append(lines, row)
		}
	}
	return m.renderPanel(kind, width, height, lines)
}

func renderMissingBranchRow(item navigationRow, width int, selected bool) string {
	prefix := "⚠ "
	suffix := fmt.Sprintf("  %d/%d › ", item.completed, item.count)
	note := "missing"
	nameWidth := max(0, width-ansi.StringWidth(prefix+suffix+note))
	name := ansi.Truncate(item.name, nameWidth, "…")
	style := lipgloss.NewStyle().Foreground(colorHigh)
	if selected {
		style = style.Background(colorSelection)
	}
	line := style.Render(prefix+name+suffix) + style.Italic(true).Render(note)
	if selected {
		line += selectedStyle.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(line))))
	}
	return line
}

func renderTaskRow(t task, width int, selected bool) string {
	mark := "○ "
	if t.done {
		mark = "✓ "
	}
	suffix := ""
	if strings.TrimSpace(t.details) != "" {
		suffix = "  ⋯"
	}
	titleWidth := max(0, width-ansi.StringWidth(mark)-ansi.StringWidth(suffix))
	title := ansi.Truncate(cleanDisplay(t.text), titleWidth, "…")
	taskStyle := lipgloss.NewStyle().Foreground(colorStrong)
	if t.priority != "" {
		taskStyle = priorityStyle(t.priority)
	}
	if t.done {
		taskStyle = mutedStyle
	}
	if !selected {
		return mutedStyle.Render(mark) + taskStyle.Render(title) + mutedStyle.Render(suffix)
	}
	titleStyle := taskStyle.Background(colorSelection)
	if t.done {
		titleStyle = selectedDoneStyle
	}
	padding := strings.Repeat(" ", max(0, width-ansi.StringWidth(mark)-ansi.StringWidth(title)-ansi.StringWidth(suffix)))
	return selectedStyle.Render(mark) + titleStyle.Render(title) + selectedStyle.Render(suffix+padding)
}

func (m *model) renderSourcePane(width, height int) string {
	innerWidth := max(1, width-4)
	heading := fmt.Sprintf("File TODOs  %d", len(m.source))
	if m.sourceLoading {
		heading += "  ↻"
	}
	lines := []string{titleStyle.Render(ansi.Truncate(heading, innerWidth, "…")), ""}
	visible := max(1, height-4)
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

func (m *model) renderDetailPane(width, height int) string {
	innerWidth := max(1, width-4)
	lines := []string{titleStyle.Render("Details"), ""}
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
	return m.renderPanel(detailPane, width, height, lines)
}

func (m *model) compactDetails() []string {
	if m.activePane() == sourcePane {
		if m.sourceCursor >= len(m.source) {
			return []string{"No file selected"}
		}
		item := m.source[m.sourceCursor]
		return []string{item.path + ":" + fmt.Sprint(item.line), cleanDisplay(item.text)}
	}
	if row, ok := m.selectedNavigationRow(); ok && row.kind != rowTask {
		return []string{groupName(row), fmt.Sprintf("%d/%d tasks · enter/→ open", row.completed, row.count)}
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
	if t.done {
		status = "Complete"
	}
	priority := mutedStyle.Render("None")
	if t.priority != "" {
		priority = priorityStyle(t.priority).Render(t.priority.title())
	}
	category := "None"
	if t.category != "" {
		category = "@" + t.category
	}
	result := []string{taskTitleStyle.Render(cleanDisplay(t.text)), ""}
	if t.details == "" {
		result = append(result, mutedStyle.Render("No details yet"))
	} else {
		for line := range strings.SplitSeq(t.details, "\n") {
			result = append(result, cleanDisplay(line))
		}
	}
	result = append(result, "", mutedStyle.Render("Status    "+status), mutedStyle.Render("Scope     "+taskLocation(t)),
		mutedStyle.Render("Priority  ")+priority, mutedStyle.Render("Category  "+category))
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
	result := []string{"", mutedStyle.Render("FILE TODO"), item.path + ":" + fmt.Sprint(item.line),
		"", cleanDisplay(item.text), "", mutedStyle.Render("CONTEXT")}
	switch {
	case m.previewPath != item.path || m.previewLine != item.line:
		result = append(result, mutedStyle.Render("Loading preview…"))
	case m.previewError != "":
		result = append(result, mutedStyle.Render(m.previewError))
	default:
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
	end := min(start+height, total)
	return start, end
}
