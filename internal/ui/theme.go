// Package ui holds the look shared by the dashboard and the file scanner:
// the theme, panels, key hints and list rows.
package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Theme is the palette for a dark or light terminal background, and the
// styles built from it. Each model keeps its own, and draws with it.
type Theme struct {
	// The palette.
	ColorText      color.Color
	ColorStrong    color.Color
	ColorMuted     color.Color
	ColorBorder    color.Color
	ColorFocus     color.Color
	ColorSelection color.Color
	ColorBar       color.Color
	ColorModal     color.Color
	ColorField     color.Color
	ColorGreen     color.Color
	ColorGit       color.Color
	ColorPurple    color.Color
	ColorHigh      color.Color
	ColorMedium    color.Color
	ColorLow       color.Color

	// Styles for the text the views share.
	TitleStyle        lipgloss.Style
	TaskTitleStyle    lipgloss.Style
	MutedStyle        lipgloss.Style
	SelectedStyle     lipgloss.Style
	SelectedDoneStyle lipgloss.Style
	StatusBarStyle    lipgloss.Style
	StatusStyle       lipgloss.Style
	KeyStyle          lipgloss.Style
}

// NewTheme is the theme for a dark or light terminal background. Until the
// terminal reports its background, a dark one is the best guess.
func NewTheme(dark bool) Theme {
	pick := lipgloss.LightDark(dark)
	hex := func(light, dark string) color.Color {
		return pick(lipgloss.Color(light), lipgloss.Color(dark))
	}
	t := Theme{
		ColorText:      hex("#1F2933", "#DCE4EF"),
		ColorStrong:    hex("#111827", "#FFFFFF"),
		ColorMuted:     hex("#5B6778", "#8190A5"),
		ColorBorder:    hex("#A0AEC0", "#526177"),
		ColorFocus:     hex("#B7791F", "#F4D35E"),
		ColorSelection: hex("#C3DAFE", "#2457A6"),
		ColorBar:       hex("#E2E8F0", "#17253A"),
		ColorModal:     hex("#F7FAFC", "#111E2F"),
		ColorField:     hex("#E6ECF3", "#1D2C42"),
		ColorGreen:     hex("#2F855A", "#80C99B"),
		ColorGit:       hex("#C2410C", "#F05032"),
		ColorPurple:    hex("#6B46C1", "#B7A4EB"),
		ColorHigh:      hex("#C53030", "#F07777"),
		ColorMedium:    hex("#B7791F", "#F4D35E"),
		ColorLow:       hex("#0E7490", "#7DCFDF"),
	}
	t.TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFocus)
	t.TaskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorStrong)
	t.MutedStyle = lipgloss.NewStyle().Foreground(t.ColorMuted)
	t.SelectedStyle = lipgloss.NewStyle().Foreground(t.ColorStrong).Background(t.ColorSelection)
	t.SelectedDoneStyle = lipgloss.NewStyle().Foreground(hex("#4A5568", "#B6C2D3")).Background(t.ColorSelection)
	t.StatusBarStyle = lipgloss.NewStyle().Foreground(t.ColorHigh).Background(t.ColorBar)
	t.StatusStyle = lipgloss.NewStyle().Foreground(t.ColorStrong)
	t.KeyStyle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorFocus)
	return t
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
