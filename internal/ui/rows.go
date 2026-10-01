package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
func (g GroupRow) Render(width int, selected bool) string {
	nameStyle, noteStyle := g.NameStyle, g.NoteStyle
	countStyle, fillStyle := MutedStyle, lipgloss.NewStyle()
	if selected {
		if !g.KeepColour {
			nameStyle = nameStyle.Foreground(ColorStrong)
		}
		nameStyle = nameStyle.Background(ColorSelection)
		noteStyle = noteStyle.Background(ColorSelection)
		countStyle, fillStyle = SelectedDoneStyle, SelectedStyle
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
func LevelMark(level string) (string, lipgloss.Style) {
	switch {
	case level == "":
		return "·", MutedStyle
	case strings.Trim(level, "0") == "":
		return LevelLabel(level), lipgloss.NewStyle().Foreground(ColorHigh)
	}
	return LevelLabel(level), lipgloss.NewStyle().Foreground(ColorMedium)
}

// LevelLabel keeps a level short: four or more zeros are written as 0x4 up
// to 0x9, and 0x9+ for anything longer; lists still sort by the real count.
func LevelLabel(level string) string {
	switch {
	case len(level) < 4 || strings.Trim(level, "0") != "":
		return level
	case len(level) > 9:
		return "0x9+"
	}
	return fmt.Sprintf("0x%d", len(level))
}

// LeveledTitle is a detail page's title, led by its todo-system level.
func LeveledTitle(text, level string) string {
	title := TaskTitleStyle.Render(CleanDisplay(text))
	if level == "" {
		return title
	}
	label, style := LevelMark(level)
	return style.Bold(true).Render(label) + " " + title
}
