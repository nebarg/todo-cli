package store

import (
	"errors"
	"strings"
)

// Priority orders tasks; PriorityNone sorts after the others.
type Priority string

const (
	PriorityNone   Priority = ""
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

var errInvalidPriority = errors.New("priority must be high, medium, or low")

// ParsePriority accepts high, medium, low or blank, ignoring case and spaces.
func ParsePriority(value string) (Priority, error) {
	switch p := Priority(strings.ToLower(strings.TrimSpace(value))); p {
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

// Title is the capitalised name written to Markdown, or blank for none.
func (p Priority) Title() string {
	if p == PriorityNone {
		return ""
	}
	return strings.ToUpper(string(p[:1])) + string(p[1:])
}
