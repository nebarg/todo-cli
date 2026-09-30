package main

import (
	"errors"
	"strings"
)

type priority string

const (
	priorityNone   priority = ""
	priorityHigh   priority = "high"
	priorityMedium priority = "medium"
	priorityLow    priority = "low"
)

var errInvalidPriority = errors.New("priority must be high, medium, or low")

func parsePriority(value string) (priority, error) {
	switch p := priority(strings.ToLower(strings.TrimSpace(value))); p {
	case priorityNone, priorityHigh, priorityMedium, priorityLow:
		return p, nil
	}
	return priorityNone, errInvalidPriority
}

func (p priority) rank() int {
	switch p {
	case priorityHigh:
		return 0
	case priorityMedium:
		return 1
	case priorityLow:
		return 2
	default:
		return 3
	}
}

func (p priority) next() priority {
	switch p {
	case priorityNone:
		return priorityHigh
	case priorityHigh:
		return priorityMedium
	case priorityMedium:
		return priorityLow
	default:
		return priorityNone
	}
}

func (p priority) title() string {
	if p == priorityNone {
		return ""
	}
	return strings.ToUpper(string(p[:1])) + string(p[1:])
}
