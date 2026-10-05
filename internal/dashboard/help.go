package dashboard

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

type helpSection struct {
	title string
	rows  []ui.KeyHint
}

// helpRow is a row of the help: what bindings do together, with their keys
// shown as they are.
func helpRow(label string, bindings ...key.Binding) ui.KeyHint {
	return ui.KeyHint{Key: ui.KeyText(bindings...), Label: label}
}

var (
	helpNavigate = helpSection{"Navigate", []ui.KeyHint{
		helpRow("jump to tab", keys.Tab1, keys.Tab2, keys.Tab3),
		helpRow("cycle tabs", keys.NextTab, keys.PrevTab),
		{Key: "↑↓ j k", Label: "move"},
		helpRow("open", keys.Open, keys.Enter),
		helpRow("back", keys.Back),
		helpRow("all tasks", keys.AllTasks),
		helpRow("sort tasks", keys.Sort),
	}}
	helpTasks = helpSection{"Tasks", []ui.KeyHint{
		helpRow("toggle done", keys.Done),
		helpRow("edit", keys.Edit, keys.Enter),
		helpRow("cycle priority", keys.Priority),
		helpRow("set category", keys.Category),
		helpRow("delete", keys.Delete),
		helpRow("add task/branch", keys.Add, keys.AddBranch),
		helpRow("add subtask", keys.Open, keys.Add),
	}}
	helpApp = []ui.KeyHint{{Key: "X / u", Label: "clear done / undo"}, helpRow("reload", keys.Reload), helpRow("help", keys.Help), helpRow("quit", keys.Quit)}
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
		for _, hint := range section.rows {
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
	for _, hint := range s.rows {
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
