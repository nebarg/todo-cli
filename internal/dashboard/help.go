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

func (helpOverlay) view(theme ui.Theme, _, _ int) string { return renderHelp(theme) }

func renderHelp(theme ui.Theme) string {
	keyWidth := 0
	for _, section := range []helpSection{helpNavigate, helpTasks} {
		for _, hint := range section.keys {
			keyWidth = max(keyWidth, ansi.StringWidth(hint.Key))
		}
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, helpNavigate.render(theme, keyWidth), "    ", helpTasks.render(theme, keyWidth))
	title := theme.TitleStyle.Render("Keys")
	closeHint := theme.MutedStyle.Render("any key closes")
	gap := strings.Repeat(" ", max(2, lipgloss.Width(body)-ansi.StringWidth(title)-ansi.StringWidth(closeHint)))
	content := title + gap + closeHint + "\n\n" + body + "\n\n" + theme.RenderHints(helpApp) + "\n\n" + priorityLegend(theme)
	return lipgloss.NewStyle().Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorFocus).BorderBackground(theme.ColorModal).
		Background(theme.ColorModal).Render(ui.OnBackground(content, theme.ColorModal))
}

func (s helpSection) render(theme ui.Theme, keyWidth int) string {
	lines := []string{helpHeadingStyle(theme).Render(s.title)}
	for _, hint := range s.keys {
		padding := strings.Repeat(" ", keyWidth-ansi.StringWidth(hint.Key))
		lines = append(lines, theme.KeyStyle.Render(hint.Key)+padding+"  "+theme.MutedStyle.Render(hint.Label))
	}
	return strings.Join(lines, "\n")
}

func priorityLegend(theme ui.Theme) string {
	parts := make([]string, 0, 3)
	for _, p := range []store.Priority{store.PriorityHigh, store.PriorityMedium, store.PriorityLow} {
		parts = append(parts, priorityStyle(theme, p).Render(priorityMark(p))+" "+theme.MutedStyle.Render(strings.ToLower(p.Title())))
	}
	return helpHeadingStyle(theme).Render("Priority") + "  " + strings.Join(parts, "  ")
}

func helpHeadingStyle(theme ui.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorText)
}
