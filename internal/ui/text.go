package ui

import (
	"fmt"
	"strings"
	"unicode"

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

// WrapLines wraps each line to width, keeping blank lines.
func WrapLines(lines []string, width int) []string {
	var result []string
	for _, line := range lines {
		if line == "" {
			result = append(result, "")
			continue
		}
		wrapped := ansi.Wrap(line, width, "")
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

// Plural reads like "1 task" or "3 tasks".
func Plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
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
