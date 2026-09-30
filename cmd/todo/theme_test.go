package main

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestThemeFollowsTerminalBackground(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	m := &model{}
	dark := colorText
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if colorText == dark || colorStrong == color.Color(lipgloss.Color("#FFFFFF")) {
		t.Fatal("light background kept the dark palette")
	}
	if got := priorityStyle(priorityHigh).GetForeground(); got != colorHigh {
		t.Fatalf("priority style did not use the light palette: %v", got)
	}
	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if colorText != dark {
		t.Fatal("dark background did not restore the dark palette")
	}
}
