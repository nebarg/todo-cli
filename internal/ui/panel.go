package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Breadcrumb shows where a panel is, with the last part highlighted and a
// muted suffix such as a count after it.
func (t Theme) Breadcrumb(parts []string, suffix string, width int) string {
	last := len(parts) - 1
	var line strings.Builder
	for _, part := range parts[:last] {
		line.WriteString(t.MutedStyle.Render(part + " › "))
	}
	line.WriteString(t.TitleStyle.Render(parts[last]))
	if suffix != "" {
		line.WriteString(t.MutedStyle.Render("  " + suffix))
	}
	return ansi.Truncate(line.String(), width, "…")
}

// ContentHeight is how many lines fit inside a panel of height, leaving
// room for its status bar when it has one.
func ContentHeight(height int, status string) int {
	lines := height - 2
	if status != "" {
		lines--
	}
	return max(1, lines)
}

// Panel draws lines in a bordered box, with status in a bar along the
// bottom when it isn't empty.
func (t Theme) Panel(width, height int, lines []string, status string) string {
	if status != "" {
		contentHeight := ContentHeight(height, status)
		lines = lines[:min(len(lines), contentHeight)]
		for len(lines) < contentHeight {
			lines = append(lines, "")
		}
		innerWidth := max(1, width-4)
		status = ansi.Truncate(status, max(1, innerWidth-2), "…")
		lines = append(lines, lipgloss.NewStyle().Background(t.ColorBar).Width(innerWidth).Padding(0, 1).Render(OnBackground(status, t.ColorBar)))
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder).
		Render(strings.Join(lines, "\n"))
}
