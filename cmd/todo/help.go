package main

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
)

type helpSection struct {
	title string
	keys  []keyHint
}

var (
	helpNavigate = helpSection{"Navigate", []keyHint{
		{"1 2 3", "jump to tab"},
		{"tab ⇧tab", "cycle tabs"},
		{"↑↓ j k", "move"},
		{"→ enter", "open"},
		{"← esc", "back"},
		{"i", "all tasks"},
		{"v", "branch list"},
	}}
	helpTasks = helpSection{"Tasks", []keyHint{
		{"d space", "toggle done"},
		{"e enter", "edit"},
		{"p", "cycle priority"},
		{"c", "set category"},
		{"a", "add task"},
		{"b", "add branch task"},
		{"s", "sort all tasks"},
	}}
	helpApp = []keyHint{{"r", "reload"}, {"?", "help"}, {"q", "quit"}}
)

func renderHelp() string {
	keyWidth := 0
	for _, section := range []helpSection{helpNavigate, helpTasks} {
		for _, hint := range section.keys {
			keyWidth = max(keyWidth, ansi.StringWidth(hint.key))
		}
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, helpNavigate.render(keyWidth), "    ", helpTasks.render(keyWidth))
	title := titleStyle.Render("Keys")
	closeHint := mutedStyle.Render("any key closes")
	gap := strings.Repeat(" ", max(2, lipgloss.Width(body)-ansi.StringWidth(title)-ansi.StringWidth(closeHint)))
	content := title + gap + closeHint + "\n\n" + body + "\n\n" + renderHints(helpApp) + "\n\n" + priorityLegend()
	return lipgloss.NewStyle().Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorFocus).BorderBackground(colorModal).
		Background(colorModal).Render(onBackground(content, colorModal))
}

func (s helpSection) render(keyWidth int) string {
	lines := []string{helpHeadingStyle().Render(s.title)}
	for _, hint := range s.keys {
		padding := strings.Repeat(" ", keyWidth-ansi.StringWidth(hint.key))
		lines = append(lines, keyStyle.Render(hint.key)+padding+"  "+mutedStyle.Render(hint.label))
	}
	return strings.Join(lines, "\n")
}

func priorityLegend() string {
	parts := make([]string, 0, 3)
	for _, p := range []store.Priority{store.PriorityHigh, store.PriorityMedium, store.PriorityLow} {
		parts = append(parts, priorityStyle(p).Render(priorityMark(p))+" "+mutedStyle.Render(strings.ToLower(p.Title())))
	}
	return helpHeadingStyle().Render("Priority") + "  " + strings.Join(parts, "  ")
}

func helpHeadingStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorText)
}
