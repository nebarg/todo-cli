package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"
)

// KeyHint is a key and what it does, as shown in a footer.
type KeyHint struct{ Key, Label string }

// Hint is the hint for a key binding: its first key, so the order of its keys
// says which one is shown, and the label for what it does where the hint is
// shown.
func Hint(b key.Binding, label string) KeyHint {
	return KeyHint{Key: glyph(b.Keys()[0]), Label: label}
}

// keyGlyphs are the keys shown as a symbol rather than by name.
var keyGlyphs = map[string]string{
	"left": "←", "right": "→", "up": "↑", "down": "↓", "backspace": "⌫", "shift+tab": "⇧tab",
}

// glyph is how a key, as a binding names it, is shown.
func glyph(name string) string {
	if symbol, ok := keyGlyphs[name]; ok {
		return symbol
	}
	return name
}

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
