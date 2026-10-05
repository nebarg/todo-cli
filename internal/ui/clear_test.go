package ui

import (
	"testing"

	"github.com/nebarg/todo-cli/internal/store"
)

func TestClearSummary(t *testing.T) {
	for _, item := range []struct {
		name    string
		targets store.ClearTargets
		want    string
	}{
		{"nothing", store.ClearTargets{}, ""},
		{"one done task", store.ClearTargets{Tasks: make([]store.Task, 1), Done: 1}, "1 done task"},
		{"done tasks", store.ClearTargets{Tasks: make([]store.Task, 3), Done: 3}, "3 done tasks"},
		{"an unknown branch's task", store.ClearTargets{Tasks: make([]store.Task, 1), Branches: []string{"a"}}, "unknown branch a with its 1 task"},
		{"an unknown branch's tasks", store.ClearTargets{Tasks: make([]store.Task, 3), Branches: []string{"a"}}, "unknown branch a with its 3 tasks"},
		{"unknown branches", store.ClearTargets{Tasks: make([]store.Task, 2), Branches: []string{"a", "b"}}, "unknown branches a, b with their 2 tasks"},
		{"both", store.ClearTargets{Tasks: make([]store.Task, 5), Done: 3, Branches: []string{"a", "b"}}, "3 done tasks and unknown branches a, b with their 2 tasks"},
		{"one of each", store.ClearTargets{Tasks: make([]store.Task, 2), Done: 1, Branches: []string{"a"}}, "1 done task and unknown branch a with its 1 task"},
		{"a done subtask", store.ClearTargets{Tasks: make([]store.Task, 1), Done: 1, DoneSubtasks: 1}, "1 done subtask"},
		{"done tasks and subtasks", store.ClearTargets{Tasks: make([]store.Task, 3), Done: 3, DoneSubtasks: 1}, "2 done tasks and 1 done subtask"},
		{"all three", store.ClearTargets{Tasks: make([]store.Task, 4), Done: 2, DoneSubtasks: 1, Branches: []string{"a"}}, "1 done task, 1 done subtask and unknown branch a with its 2 tasks"},
		// The targets picked from a file with a done task and its subtasks, as the store tests check.
		{"a done task with done subtasks", store.ClearTargets{Tasks: make([]store.Task, 3), Done: 3, DoneSubtasks: 2}, "1 done task and 2 done subtasks"},
		{"a done subtask and a missing branch", store.ClearTargets{Tasks: make([]store.Task, 3), Done: 2, DoneSubtasks: 1, Branches: []string{"feature/gone"}},
			"1 done task, 1 done subtask and unknown branch feature/gone with its 1 task"},
	} {
		t.Run(item.name, func(t *testing.T) {
			if got := ClearSummary(item.targets); got != item.want {
				t.Fatalf("ClearSummary() = %q, want %q", got, item.want)
			}
		})
	}
}

func TestSubtaskNote(t *testing.T) {
	for _, item := range []struct {
		subtasks, open int
		want           string
	}{
		{0, 0, ""},
		{1, 1, "1 open subtask goes with its task."},
		{1, 0, "1 subtask goes with its task."},
		{3, 3, "3 open subtasks go with their tasks."},
		{3, 0, "3 subtasks go with their tasks."},
		{3, 1, "3 subtasks go with their tasks, 1 of them open."},
		{2, 1, "2 subtasks go with their tasks, 1 of them open."},
	} {
		if got := SubtaskNote(store.ClearTargets{Subtasks: item.subtasks, OpenSubtasks: item.open}); got != item.want {
			t.Errorf("%d subtasks, %d open = %q, want %q", item.subtasks, item.open, got, item.want)
		}
	}
}
