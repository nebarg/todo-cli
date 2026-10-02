package dashboard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

type helpSection struct {
	title string
	keys  []ui.KeyHint
}

var (
	helpNavigate = helpSection{"Navigate", []ui.KeyHint{
		{Key: "1 2 3", Label: "jump to tab"},
		{Key: "tab ⇧tab", Label: "cycle tabs"},
		{Key: "↑↓ j k", Label: "move"},
		{Key: "→ enter", Label: "open"},
		{Key: "← esc", Label: "back"},
		{Key: "i", Label: "all tasks"},
	}}
	helpTasks = helpSection{"Tasks", []ui.KeyHint{
		{Key: "d space", Label: "toggle done"},
		{Key: "e enter", Label: "edit"},
		{Key: "p", Label: "cycle priority"},
		{Key: "c", Label: "set category"},
		{Key: "a", Label: "add task"},
		{Key: "b", Label: "add branch task"},
		{Key: "s", Label: "sort all tasks"},
	}}
	helpApp = []ui.KeyHint{{Key: "X / u", Label: "clear done / undo"}, {Key: "r", Label: "reload"}, {Key: "?", Label: "help"}, {Key: "q", Label: "quit"}}
)

// helpOverlay lists the keys until any key closes it.
type helpOverlay struct{}

func (h helpOverlay) update(msg tea.Msg) (overlay, tea.Msg, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return nil, nil, nil
	}
	return h, nil, nil
}

func (helpOverlay) view(int, int) string { return renderHelp() }

func renderHelp() string {
	keyWidth := 0
	for _, section := range []helpSection{helpNavigate, helpTasks} {
		for _, hint := range section.keys {
			keyWidth = max(keyWidth, ansi.StringWidth(hint.Key))
		}
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, helpNavigate.render(keyWidth), "    ", helpTasks.render(keyWidth))
	title := ui.TitleStyle.Render("Keys")
	closeHint := ui.MutedStyle.Render("any key closes")
	gap := strings.Repeat(" ", max(2, lipgloss.Width(body)-ansi.StringWidth(title)-ansi.StringWidth(closeHint)))
	content := title + gap + closeHint + "\n\n" + body + "\n\n" + ui.RenderHints(helpApp) + "\n\n" + priorityLegend()
	return lipgloss.NewStyle().Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(ui.ColorFocus).BorderBackground(ui.ColorModal).
		Background(ui.ColorModal).Render(ui.OnBackground(content, ui.ColorModal))
}

func (s helpSection) render(keyWidth int) string {
	lines := []string{helpHeadingStyle().Render(s.title)}
	for _, hint := range s.keys {
		padding := strings.Repeat(" ", keyWidth-ansi.StringWidth(hint.Key))
		lines = append(lines, ui.KeyStyle.Render(hint.Key)+padding+"  "+ui.MutedStyle.Render(hint.Label))
	}
	return strings.Join(lines, "\n")
}

func priorityLegend() string {
	parts := make([]string, 0, 3)
	for _, p := range []store.Priority{store.PriorityHigh, store.PriorityMedium, store.PriorityLow} {
		parts = append(parts, priorityStyle(p).Render(priorityMark(p))+" "+ui.MutedStyle.Render(strings.ToLower(p.Title())))
	}
	return helpHeadingStyle().Render("Priority") + "  " + strings.Join(parts, "  ")
}

func helpHeadingStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText)
}
