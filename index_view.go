package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func priorityMarker(priority string) string {
	switch priority {
	case "high":
		return "!!!"
	case "medium":
		return "!!"
	case "low":
		return "!"
	default:
		return "-"
	}
}

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
	labelWidth := min(18, max(8, (innerWidth-22)/4))
	branchWidth := min(26, max(10, (innerWidth-22)/3))
	taskWidth := max(1, innerWidth-3-6-labelWidth-branchWidth)
	heading := fmt.Sprintf("All tasks  %d · Sort: %s", len(tasks), m.indexSort)
	lines := []string{titleStyle.Render(ansi.Truncate(heading, innerWidth, "…"))}
	columns := indexColumn("PRI", 3) + "  " + indexColumn("LABEL", labelWidth) + "  " + indexColumn("BRANCH", branchWidth) + "  " + indexColumn("TASK: DETAILS", taskWidth)
	lines = append(lines, mutedStyle.Render(columns))
	visible := max(1, height-4)
	start, end := visibleRange(m.indexCursor, len(tasks), visible)
	if len(tasks) == 0 {
		lines = append(lines, mutedStyle.Render("No tasks in the Markdown file"))
	}
	for i := start; i < end; i++ {
		t := tasks[i]
		label, branch := "-", "-"
		if taskLabel(t) != "" {
			label = "@" + taskLabel(t)
		}
		if t.branch != "" {
			branch = t.branch
		}
		title := cleanDisplay(t.text)
		if t.done {
			title = "✓ " + title
		}
		if details := strings.Join(strings.Fields(cleanDisplay(t.details)), " "); details != "" {
			title += ": " + details
		}
		selected := i == m.indexCursor
		priority := priorityStyle(t.priority)
		labelStyle := lipgloss.NewStyle().Foreground(colorPurple)
		branchStyle := lipgloss.NewStyle().Foreground(colorGreen)
		taskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		if t.done {
			taskStyle = mutedStyle
		}
		if selected {
			priority = priority.Background(colorBlue)
			labelStyle = labelStyle.Background(colorBlue)
			branchStyle = branchStyle.Background(colorBlue)
			taskStyle = taskStyle.Background(colorBlue)
		}
		gap := "  "
		if selected {
			gap = lipgloss.NewStyle().Background(colorBlue).Render(gap)
		}
		row := priority.Render(indexColumn(priorityMarker(t.priority), 3)) + gap +
			labelStyle.Render(indexColumn(label, labelWidth)) + gap +
			branchStyle.Render(indexColumn(branch, branchWidth)) + gap +
			taskStyle.Render(indexColumn(title, taskWidth))
		lines = append(lines, row)
	}
	return m.panelStyle(true, width, height).Render(strings.Join(lines, "\n"))
}
