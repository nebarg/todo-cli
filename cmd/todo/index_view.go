package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

func priorityStyle(p store.Priority) lipgloss.Style {
	color := ui.ColorMuted
	switch p {
	case store.PriorityHigh:
		color = ui.ColorHigh
	case store.PriorityMedium:
		color = ui.ColorMedium
	case store.PriorityLow:
		color = ui.ColorLow
	}
	return lipgloss.NewStyle().Bold(p != store.PriorityNone).Foreground(color)
}

// priorityMark fills an open task's circle when it has a priority, whose
// colour then tells the levels apart; an empty circle means no priority.
func priorityMark(p store.Priority) string {
	if p == store.PriorityNone {
		return "○"
	}
	return "●"
}

func (m *model) renderIndex(width, height int) string {
	innerWidth := max(1, width-4)
	tasks := m.indexTasks()
	scopeWidth := min(24, max(17, innerWidth/3))
	taskWidth := max(1, innerWidth-2-scopeWidth)
	heading := ui.TitleStyle.Render("All tasks") + ui.MutedStyle.Render(fmt.Sprintf("  %d/%d", completedCount(tasks), len(tasks)))
	sorted := ui.MutedStyle.Render("sorted by ") + lipgloss.NewStyle().Foreground(ui.ColorText).Render(string(m.indexSort))
	if gap := innerWidth - ansi.StringWidth(heading) - ansi.StringWidth(sorted); gap >= 2 {
		heading += strings.Repeat(" ", gap) + sorted
	}
	lines := []string{ansi.Truncate(heading, innerWidth, "…"), ""}
	columns := ui.Column("TASK: DETAILS", taskWidth) + "  " + ui.Column("CATEGORY / BRANCH", scopeWidth)
	lines = append(lines, ui.MutedStyle.Render(columns))
	visible := max(1, height-5)
	start, end := ui.VisibleRange(m.indexCursor, len(tasks), visible)
	if len(tasks) == 0 {
		lines = append(lines, ui.MutedStyle.Render("No tasks in the Markdown file"))
	}
	for i := start; i < end; i++ {
		t := tasks[i]
		scope := "-"
		if t.Category != "" {
			scope = "@" + t.Category
		}
		missing := m.branchMissing(t.Branch)
		if t.Branch != "" {
			scope = branchIcon + " " + t.Branch
		}
		if missing {
			scope = "⚠ " + t.Branch
		}
		mark, markStyle := priorityMark(t.Priority)+" ", ui.MutedStyle
		if t.Priority != "" {
			markStyle = priorityStyle(t.Priority)
		}
		taskStyle := lipgloss.NewStyle().Foreground(ui.ColorStrong)
		if t.Done {
			mark, markStyle, taskStyle = "✓ ", ui.MutedStyle, ui.MutedStyle
		}
		title := ui.CleanDisplay(t.Text)
		if details := strings.Join(strings.Fields(ui.CleanDisplay(t.Details)), " "); details != "" {
			title += ": " + details
		}
		selected := i == m.indexCursor
		scopeStyle := ui.MutedStyle
		switch {
		case missing:
			scopeStyle = lipgloss.NewStyle().Foreground(ui.ColorHigh)
		case t.Branch != "":
			scopeStyle = lipgloss.NewStyle().Foreground(ui.ColorGreen)
		case t.Category != "":
			scopeStyle = lipgloss.NewStyle().Foreground(ui.ColorPurple)
		}
		if selected {
			markStyle = markStyle.Background(ui.ColorSelection)
			taskStyle = taskStyle.Background(ui.ColorSelection)
			scopeStyle = scopeStyle.Background(ui.ColorSelection)
		}
		gap := "  "
		if selected {
			gap = lipgloss.NewStyle().Background(ui.ColorSelection).Render(gap)
		}
		row := markStyle.Render(mark) + taskStyle.Render(ui.Column(title, taskWidth-ansi.StringWidth(mark))) + gap +
			scopeStyle.Render(ui.Column(scope, scopeWidth))
		lines = append(lines, row)
	}
	return ui.Panel(width, height, lines, "")
}
