// Package ui holds the look shared by the dashboard and the file scanner:
// the palette, panels, key hints and list rows.
package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The palette is package-level so every view can share it. It starts dark
// and ApplyTheme swaps it once the terminal reports its background.
var (
	ColorText         color.Color
	ColorStrong       color.Color
	ColorMuted        color.Color
	ColorBorder       color.Color
	ColorFocus        color.Color
	ColorSelection    color.Color
	ColorBar          color.Color
	ColorModal        color.Color
	ColorField        color.Color
	ColorGreen        color.Color
	ColorGit          color.Color
	ColorPurple       color.Color
	ColorHigh         color.Color
	ColorMedium       color.Color
	ColorLow          color.Color
	TitleStyle        lipgloss.Style
	TaskTitleStyle    lipgloss.Style
	MutedStyle        lipgloss.Style
	SelectedStyle     lipgloss.Style
	SelectedDoneStyle lipgloss.Style
	StatusBarStyle    lipgloss.Style
	StatusStyle       lipgloss.Style
	KeyStyle          lipgloss.Style
)

func init() {
	ApplyTheme(true)
}

// ApplyTheme sets the palette for a dark or light terminal background.
func ApplyTheme(dark bool) {
	pick := lipgloss.LightDark(dark)
	hex := func(light, dark string) color.Color {
		return pick(lipgloss.Color(light), lipgloss.Color(dark))
	}
	ColorText = hex("#1F2933", "#DCE4EF")
	ColorStrong = hex("#111827", "#FFFFFF")
	ColorMuted = hex("#5B6778", "#8190A5")
	ColorBorder = hex("#A0AEC0", "#526177")
	ColorFocus = hex("#B7791F", "#F4D35E")
	ColorSelection = hex("#C3DAFE", "#2457A6")
	ColorBar = hex("#E2E8F0", "#17253A")
	ColorModal = hex("#F7FAFC", "#111E2F")
	ColorField = hex("#E6ECF3", "#1D2C42")
	ColorGreen = hex("#2F855A", "#80C99B")
	ColorGit = hex("#C2410C", "#F05032")
	ColorPurple = hex("#6B46C1", "#B7A4EB")
	ColorHigh = hex("#C53030", "#F07777")
	ColorMedium = hex("#B7791F", "#F4D35E")
	ColorLow = hex("#0E7490", "#7DCFDF")

	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorFocus)
	TaskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorStrong)
	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	SelectedStyle = lipgloss.NewStyle().Foreground(ColorStrong).Background(ColorSelection)
	SelectedDoneStyle = lipgloss.NewStyle().Foreground(hex("#4A5568", "#B6C2D3")).Background(ColorSelection)
	StatusBarStyle = lipgloss.NewStyle().Foreground(ColorHigh).Background(ColorBar)
	StatusStyle = lipgloss.NewStyle().Foreground(ColorStrong)
	KeyStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorFocus)
}

// OnBackground paints s onto bg. Nested styles end with a full reset, which
// would otherwise let the terminal background show through the rest of the
// line, so bg is re-applied after every reset.
func OnBackground(s string, bg color.Color) string {
	seq := ansi.NewStyle().BackgroundColor(bg).String()
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+seq)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+seq)
	return seq + strings.ReplaceAll(s, "\x1b[49m", seq)
}
