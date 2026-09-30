package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
)

func priorityStyle(p store.Priority) lipgloss.Style {
	color := colorMuted
	switch p {
	case store.PriorityHigh:
		color = colorHigh
	case store.PriorityMedium:
		color = colorMedium
	case store.PriorityLow:
		color = colorLow
	}
	return lipgloss.NewStyle().Bold(p != store.PriorityNone).Foreground(color)
}

// priorityMark fills an open task's circle by urgency, so priority reads
// from shape as well as colour.
func priorityMark(p store.Priority) string {
	switch p {
	case store.PriorityHigh:
		return "●"
	case store.PriorityMedium:
		return "◐"
	}
	return "○"
}

func indexColumn(value string, width int) string {
	value = ansi.Truncate(cleanDisplay(value), width, "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}

func (m *model) renderIndex(width, height int) string {
	innerWidth := max(1, width-4)
	tasks := m.indexTasks()
	scopeWidth := min(24, max(17, innerWidth/3))
	taskWidth := max(1, innerWidth-2-scopeWidth)
	heading := titleStyle.Render("All tasks") + mutedStyle.Render(fmt.Sprintf("  %d/%d", completedCount(tasks), len(tasks)))
	sorted := mutedStyle.Render("sorted by ") + lipgloss.NewStyle().Foreground(colorText).Render(string(m.indexSort))
	if gap := innerWidth - ansi.StringWidth(heading) - ansi.StringWidth(sorted); gap >= 2 {
		heading += strings.Repeat(" ", gap) + sorted
	}
	lines := []string{ansi.Truncate(heading, innerWidth, "…"), ""}
	columns := indexColumn("TASK: DETAILS", taskWidth) + "  " + indexColumn("CATEGORY / BRANCH", scopeWidth)
	lines = append(lines, mutedStyle.Render(columns))
	visible := max(1, height-5)
	start, end := visibleRange(m.indexCursor, len(tasks), visible)
	if len(tasks) == 0 {
		lines = append(lines, mutedStyle.Render("No tasks in the Markdown file"))
	}
	for i := start; i < end; i++ {
		t := tasks[i]
		scope := "-"
		if t.Category != "" {
			scope = "@" + t.Category
		}
		if t.Branch != "" {
			scope = branchIcon + " " + t.Branch
		}
		mark, markStyle := priorityMark(t.Priority)+" ", mutedStyle
		if t.Priority != "" {
			markStyle = priorityStyle(t.Priority)
		}
		taskStyle := lipgloss.NewStyle().Foreground(colorStrong)
		if t.Done {
			mark, markStyle, taskStyle = "✓ ", mutedStyle, mutedStyle
		}
		title := cleanDisplay(t.Text)
		if details := strings.Join(strings.Fields(cleanDisplay(t.Details)), " "); details != "" {
			title += ": " + details
		}
		selected := i == m.indexCursor
		scopeStyle := mutedStyle
		if t.Branch != "" {
			scopeStyle = lipgloss.NewStyle().Foreground(colorGreen)
		} else if t.Category != "" {
			scopeStyle = lipgloss.NewStyle().Foreground(colorPurple)
		}
		if selected {
			markStyle = markStyle.Background(colorSelection)
			taskStyle = taskStyle.Background(colorSelection)
			scopeStyle = scopeStyle.Background(colorSelection)
		}
		gap := "  "
		if selected {
			gap = lipgloss.NewStyle().Background(colorSelection).Render(gap)
		}
		row := markStyle.Render(mark) + taskStyle.Render(indexColumn(title, taskWidth-ansi.StringWidth(mark))) + gap +
			scopeStyle.Render(indexColumn(scope, scopeWidth))
		lines = append(lines, row)
	}
	return m.panelStyle(width, height).Render(strings.Join(lines, "\n"))
}
