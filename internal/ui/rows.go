package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/level"
)

// GroupRow is a row that opens a group, such as a category or a branch:
// "▸ name  note", with a count on the right.
type GroupRow struct {
	Marker, Name, Note, Count string
	NameStyle, NoteStyle      lipgloss.Style
	// KeepColour keeps NameStyle's colour when selected, for names whose
	// colour means something, such as the current branch.
	KeepColour bool
}

// Render draws the row in width cells.
func (g GroupRow) Render(theme Theme, width int, selected bool) string {
	nameStyle, noteStyle := g.NameStyle, g.NoteStyle
	countStyle, fillStyle := theme.MutedStyle, lipgloss.NewStyle()
	if selected {
		if !g.KeepColour {
			nameStyle = nameStyle.Foreground(theme.ColorStrong)
		}
		nameStyle = nameStyle.Background(theme.ColorSelection)
		noteStyle = noteStyle.Background(theme.ColorSelection)
		countStyle, fillStyle = theme.SelectedDoneStyle, theme.SelectedStyle
	}
	note := g.Note
	if note != "" {
		note = "  " + note
	}
	fixed := ansi.StringWidth(g.Marker) + ansi.StringWidth(note) + ansi.StringWidth(g.Count) + 2
	name := ansi.Truncate(g.Name, max(0, width-fixed), "…")
	gap := max(2, width-fixed+2-ansi.StringWidth(name))
	return nameStyle.Render(g.Marker+name) + noteStyle.Render(note) + fillStyle.Render(strings.Repeat(" ", gap)) + countStyle.Render(g.Count)
}

// LevelMark is a todo-system level's label and colour: red for levels of
// zeros, yellow for the rest, and a dim dot without a level.
func (t Theme) LevelMark(l string) (string, lipgloss.Style) {
	switch {
	case l == "":
		return "·", t.MutedStyle
	case level.Zeros(l):
		return LevelLabel(l), lipgloss.NewStyle().Foreground(t.ColorHigh)
	}
	return LevelLabel(l), lipgloss.NewStyle().Foreground(t.ColorMedium)
}

// LevelLabel keeps a level short: four or more zeros are written as 0x4 up
// to 0x9, and 0x9+ for anything longer; lists still sort by the real count.
func LevelLabel(l string) string {
	switch {
	case len(l) < 4 || !level.Zeros(l):
		return l
	case len(l) > 9:
		return "0x9+"
	}
	return fmt.Sprintf("0x%d", len(l))
}

// LeveledTitle leads a detail page's title, already styled, with its
// todo-system level.
func (t Theme) LeveledTitle(title, level string) string {
	if level == "" {
		return title
	}
	label, style := t.LevelMark(level)
	return style.Bold(true).Render(label) + " " + title
}
