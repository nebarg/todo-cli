package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// KeyHint is a key and what it does, as shown in a footer.
type KeyHint struct{ Key, Label string }

// RenderHints lays hints out in one line.
func (t Theme) RenderHints(hints []KeyHint) string {
	parts := make([]string, len(hints))
	for i, hint := range hints {
		parts[i] = t.KeyStyle.Render(hint.Key) + " " + t.MutedStyle.Render(hint.Label)
	}
	return strings.Join(parts, "  ")
}

// FitHints keeps the leading hints that fit, so each list is ordered by importance.
func (t Theme) FitHints(hints []KeyHint, width int) string {
	for n := len(hints); n > 0; n-- {
		if line := t.RenderHints(hints[:n]); ansi.StringWidth(line) <= width {
			return line
		}
	}
	return ""
}

// Footer lays out a status message and the keys for where the user is on
// the left, dropping the least important keys first, and pinned keys on the
// right.
func (t Theme) Footer(status string, hints, pinned []KeyHint, width int) string {
	right := t.RenderHints(pinned)
	available := max(0, width-ansi.StringWidth(right)-3)
	left := t.FitHints(hints, available)
	if status != "" {
		left = t.StatusStyle.Render(ansi.Truncate(status, max(1, available), "…"))
		if keys := t.FitHints(hints, available-ansi.StringWidth(left)-3); keys != "" {
			left += "   " + keys
		}
	}
	return left + strings.Repeat(" ", max(0, width-ansi.StringWidth(left)-ansi.StringWidth(right))) + right
}
