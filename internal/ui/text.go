package ui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// CleanDisplay replaces control characters, which would break the layout,
// with spaces.
func CleanDisplay(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// WrapLines wraps each line to width, keeping blank lines. A style that runs
// onto the next line is carried over, so it doesn't spill into the border.
func WrapLines(lines []string, width int) []string {
	var result []string
	for _, line := range lines {
		if line == "" {
			result = append(result, "")
			continue
		}
		wrapped := lipgloss.Wrap(line, width, "")
		result = append(result, strings.Split(wrapped, "\n")...)
	}
	return result
}

// Column fits value into exactly width cells, cutting it with an ellipsis or
// padding it with spaces.
func Column(value string, width int) string {
	value = ansi.Truncate(CleanDisplay(value), width, "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}

// StyledColumn is Column for value already drawn in styles: it's padded with
// spaces in pad, so a selected row's background reaches the column's end.
func StyledColumn(value string, width int, pad lipgloss.Style) string {
	value = ansi.Truncate(value, width, "…")
	return value + pad.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(value))))
}

// Plural reads like "1 task" or "3 tasks".
func Plural(n int, one, many string) string {
	return fmt.Sprintf("%d %s", n, PluralWord(n, one, many))
}

// PluralWord is the word for a count of n, as in "task" or "tasks", without
// the count.
func PluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// JoinNames lists names as "a", "a and b" or "a, b and c", or is "" for none.
func JoinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// VisibleRange is the slice of total rows that fits in height while keeping
// cursor on screen.
func VisibleRange(cursor, total, height int) (int, int) {
	if height < 1 {
		height = 1
	}
	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	end := min(start+height, total)
	return start, end
}
