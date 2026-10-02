package dashboard

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

func TestThemeFollowsTerminalBackground(t *testing.T) {
	t.Cleanup(func() { ui.ApplyTheme(true) })
	m := &model{}
	dark := ui.ColorText
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if ui.ColorText == dark || ui.ColorStrong == color.Color(lipgloss.Color("#FFFFFF")) {
		t.Fatal("light background kept the dark palette")
	}
	if got := priorityStyle(store.PriorityHigh).GetForeground(); got != ui.ColorHigh {
		t.Fatalf("priority style did not use the light palette: %v", got)
	}
	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if ui.ColorText != dark {
		t.Fatal("dark background did not restore the dark palette")
	}
}
