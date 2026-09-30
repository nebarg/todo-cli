package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Nerd Font glyphs: the Octicons branch and the Git logo.
const (
	branchIcon = "\uf418"
	gitIcon    = "\ue702"
)

// The palette is package-level so the render helpers can share it. It starts
// dark and applyTheme swaps it once the terminal reports its background.
var (
	colorText         color.Color
	colorStrong       color.Color
	colorMuted        color.Color
	colorBorder       color.Color
	colorFocus        color.Color
	colorSelection    color.Color
	colorBar          color.Color
	colorModal        color.Color
	colorField        color.Color
	colorGreen        color.Color
	colorGit          color.Color
	colorPurple       color.Color
	colorHigh         color.Color
	colorMedium       color.Color
	colorLow          color.Color
	titleStyle        lipgloss.Style
	taskTitleStyle    lipgloss.Style
	mutedStyle        lipgloss.Style
	selectedStyle     lipgloss.Style
	selectedDoneStyle lipgloss.Style
	statusBarStyle    lipgloss.Style
	statusStyle       lipgloss.Style
	keyStyle          lipgloss.Style
)

func init() {
	applyTheme(true)
}

func applyTheme(dark bool) {
	pick := lipgloss.LightDark(dark)
	hex := func(light, dark string) color.Color {
		return pick(lipgloss.Color(light), lipgloss.Color(dark))
	}
	colorText = hex("#1F2933", "#DCE4EF")
	colorStrong = hex("#111827", "#FFFFFF")
	colorMuted = hex("#5B6778", "#8190A5")
	colorBorder = hex("#A0AEC0", "#526177")
	colorFocus = hex("#B7791F", "#F4D35E")
	colorSelection = hex("#C3DAFE", "#2457A6")
	colorBar = hex("#E2E8F0", "#17253A")
	colorModal = hex("#F7FAFC", "#111E2F")
	colorField = hex("#E6ECF3", "#1D2C42")
	colorGreen = hex("#2F855A", "#80C99B")
	colorGit = hex("#C2410C", "#F05032")
	colorPurple = hex("#6B46C1", "#B7A4EB")
	colorHigh = hex("#C53030", "#F07777")
	colorMedium = hex("#B7791F", "#F4D35E")
	colorLow = hex("#0E7490", "#7DCFDF")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorFocus)
	taskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorStrong)
	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)
	selectedStyle = lipgloss.NewStyle().Foreground(colorStrong).Background(colorSelection)
	selectedDoneStyle = lipgloss.NewStyle().Foreground(hex("#4A5568", "#B6C2D3")).Background(colorSelection)
	statusBarStyle = lipgloss.NewStyle().Foreground(colorHigh).Background(colorBar)
	statusStyle = lipgloss.NewStyle().Foreground(colorStrong)
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorFocus)
}

// onBackground paints s onto bg. Nested styles end with a full reset, which
// would otherwise let the terminal background show through the rest of the
// line, so bg is re-applied after every reset.
func onBackground(s string, bg color.Color) string {
	seq := ansi.NewStyle().BackgroundColor(bg).String()
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+seq)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+seq)
	return seq + strings.ReplaceAll(s, "\x1b[49m", seq)
}
