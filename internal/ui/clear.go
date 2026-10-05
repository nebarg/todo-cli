package ui

import (
	"fmt"
	"strings"

	"github.com/nebarg/todo-cli/internal/store"
)

// ClearSummary reads like "3 done tasks, 1 done subtask and unknown branch
// feature/x with its 2 tasks", naming the branches Git doesn't have.
func ClearSummary(c store.ClearTargets) string {
	var parts []string
	if n := c.Done - c.DoneSubtasks; n > 0 {
		parts = append(parts, Plural(n, "done task", "done tasks"))
	}
	if c.DoneSubtasks > 0 {
		parts = append(parts, Plural(c.DoneSubtasks, "done subtask", "done subtasks"))
	}
	tasks := Plural(len(c.Tasks)-c.Done, "task", "tasks")
	switch len(c.Branches) {
	case 0:
	case 1:
		parts = append(parts, fmt.Sprintf("unknown branch %s with its %s", c.Branches[0], tasks))
	default:
		parts = append(parts, fmt.Sprintf("unknown branches %s with their %s", strings.Join(c.Branches, ", "), tasks))
	}
	return JoinNames(parts)
}

// SubtaskNote says how many subtasks go with the tasks they're under, as in
// "3 subtasks go with their tasks, 1 of them open.", or is "" when none do.
func SubtaskNote(c store.ClearTargets) string {
	subtasks := Plural(c.Subtasks, "subtask", "subtasks")
	verb, theirs := PluralWord(c.Subtasks, "goes", "go"), PluralWord(c.Subtasks, "its task", "their tasks")
	switch {
	case c.Subtasks == 0:
		return ""
	case c.OpenSubtasks == c.Subtasks:
		subtasks = Plural(c.Subtasks, "open subtask", "open subtasks")
	case c.OpenSubtasks > 0:
		return fmt.Sprintf("%s %s with %s, %d of them open.", subtasks, verb, theirs, c.OpenSubtasks)
	}
	return fmt.Sprintf("%s %s with %s.", subtasks, verb, theirs)
}
