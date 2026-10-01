package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// KeyHint is a key and what it does, as shown in a footer.
type KeyHint struct{ Key, Label string }

// RenderHints lays hints out in one line.
func RenderHints(hints []KeyHint) string {
	parts := make([]string, len(hints))
	for i, hint := range hints {
		parts[i] = KeyStyle.Render(hint.Key) + " " + MutedStyle.Render(hint.Label)
	}
	return strings.Join(parts, "  ")
}

// FitHints keeps the leading hints that fit, so each list is ordered by importance.
func FitHints(hints []KeyHint, width int) string {
	for n := len(hints); n > 0; n-- {
		if line := RenderHints(hints[:n]); ansi.StringWidth(line) <= width {
			return line
		}
	}
	return ""
}
