package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func priorityStyle(priority string) lipgloss.Style {
	color := colorMuted
	switch priority {
	case "high":
		color = colorHigh
	case "medium":
		color = colorMedium
	case "low":
		color = colorLow
	}
	return lipgloss.NewStyle().Bold(priority != "").Foreground(color)
}

func indexColumn(value string, width int) string {
	value = ansi.Truncate(cleanDisplay(value), width, "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}

func (m model) renderIndex(width, height int) string {
	innerWidth := max(1, width-4)
	tasks := m.indexTasks()
	scopeWidth := min(24, max(17, innerWidth/3))
	taskWidth := max(1, innerWidth-2-scopeWidth)
	heading := fmt.Sprintf("All tasks  %d/%d · Sort: %s", completedCount(tasks), len(tasks), m.indexSort)
	lines := []string{titleStyle.Render(ansi.Truncate(heading, innerWidth, "…")), ""}
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
		if taskLabel(t) != "" {
			scope = "@" + taskLabel(t)
		}
		if t.branch != "" {
			scope = " " + t.branch
		}
		mark := "○ "
		if t.done {
			mark = "✓ "
		}
		title := mark + cleanDisplay(t.text)
		if details := strings.Join(strings.Fields(cleanDisplay(t.details)), " "); details != "" {
			title += ": " + details
		}
		selected := i == m.indexCursor
		taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		if t.priority != "" {
			taskStyle = priorityStyle(t.priority)
		}
		if t.done {
			taskStyle = mutedStyle
		}
		scopeStyle := mutedStyle
		if t.branch != "" {
			scopeStyle = lipgloss.NewStyle().Foreground(colorGreen)
		} else if taskLabel(t) != "" {
			scopeStyle = lipgloss.NewStyle().Foreground(colorPurple)
		}
		if selected {
			taskStyle = taskStyle.Background(colorBlue)
			scopeStyle = scopeStyle.Background(colorBlue)
		}
		gap := "  "
		if selected {
			gap = lipgloss.NewStyle().Background(colorBlue).Render(gap)
		}
		row := taskStyle.Render(indexColumn(title, taskWidth)) + gap +
			scopeStyle.Render(indexColumn(scope, scopeWidth))
		lines = append(lines, row)
	}
	return m.panelStyle(true, width, height).Render(strings.Join(lines, "\n"))
}
