package store

import (
	"errors"
	"strings"
	"unicode"
)

// Priority orders tasks; PriorityNone sorts after the others.
type Priority string

// The priorities a task can have, written in todo.md as a trailing !high,
// !medium or !low. PriorityNone is a task without one.
const (
	PriorityNone   Priority = ""
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

var errInvalidPriority = errors.New("priority must be h, high, m, medium, l, or low")

// ParsePriority reads a priority as typed for the -p flag: high, medium, low,
// their first letters, or blank, ignoring case and spaces.
func ParsePriority(value string) (Priority, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, p := range []Priority{PriorityHigh, PriorityMedium, PriorityLow} {
		if value == string(p[:1]) {
			return p, nil
		}
	}
	return parsePriorityName(value)
}

// parsePriorityName accepts only the full names, ignoring case, so that in
// a task file a shorter trailing word such as "!h" stays part of the title.
func parsePriorityName(value string) (Priority, error) {
	switch p := Priority(strings.ToLower(value)); p {
	case PriorityNone, PriorityHigh, PriorityMedium, PriorityLow:
		return p, nil
	}
	return PriorityNone, errInvalidPriority
}

// Rank sorts priorities from high (0) to none (3).
func (p Priority) Rank() int {
	switch p {
	case PriorityHigh:
		return 0
	case PriorityMedium:
		return 1
	case PriorityLow:
		return 2
	default:
		return 3
	}
}

// Next cycles none → high → medium → low → none.
func (p Priority) Next() Priority {
	switch p {
	case PriorityNone:
		return PriorityHigh
	case PriorityHigh:
		return PriorityMedium
	case PriorityMedium:
		return PriorityLow
	default:
		return PriorityNone
	}
}

// SplitPriority separates a trailing !high, !medium or !low from a task
// title. Any other trailing !word, or a title that is only a priority, stays
// part of the title.
func SplitPriority(text string) (string, Priority) {
	trimmed := strings.TrimRightFunc(text, unicode.IsSpace)
	cut := strings.LastIndexFunc(trimmed, unicode.IsSpace)
	if cut < 0 || !strings.HasPrefix(trimmed[cut+1:], "!") {
		return text, PriorityNone
	}
	p, err := parsePriorityName(trimmed[cut+2:])
	title := strings.TrimRightFunc(trimmed[:cut], unicode.IsSpace)
	if err != nil || p == PriorityNone || title == "" {
		return text, PriorityNone
	}
	return title, p
}

func priorityToken(p Priority) string {
	if p == PriorityNone {
		return ""
	}
	return " !" + string(p)
}

// Title is the capitalised name shown for a priority, or blank for none.
func (p Priority) Title() string {
	if p == PriorityNone {
		return ""
	}
	return strings.ToUpper(string(p[:1])) + string(p[1:])
}
