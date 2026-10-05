package store

import (
	"cmp"
	"errors"
	"os"
	"slices"
)

// ErrFileChanged means the file is no longer the one a removal was planned
// from, or undone to, so it was left alone.
var ErrFileChanged = errors.New("file changed on disk")

// ClearTargets is what a clear removes: every done task, and every task of a
// branch Git doesn't have, each with its subtasks.
type ClearTargets struct {
	// Tasks are the tasks to remove, in the order given, leaving out
	// subtasks that go with their task.
	Tasks []Task
	// Done is how many of Tasks are done tasks of branches that still exist,
	// and DoneSubtasks how many of those are subtasks of open tasks.
	Done, DoneSubtasks int
	// Subtasks is how many subtasks go with the tasks they're under, and
	// OpenSubtasks how many of those are open.
	Subtasks, OpenSubtasks int
	// Branches are the missing branches whose tasks are removed, sorted.
	Branches []string
}

// PickClearTargets picks out of tasks the done tasks, and every task of a
// branch missing reports gone. A task's subtasks go with it.
func PickClearTargets(tasks []Task, missing func(branch string) bool) ClearTargets {
	picked := slices.DeleteFunc(slices.Clone(tasks), func(t Task) bool { return !t.Done && !missing(t.Branch) })
	var c ClearTargets
	for _, t := range tasks {
		switch {
		case slices.ContainsFunc(picked, func(p Task) bool { return p.Holds(t) }):
			c.Subtasks++
			if !t.Done {
				c.OpenSubtasks++
			}
		case missing(t.Branch):
			c.Tasks = append(c.Tasks, t)
			if !slices.Contains(c.Branches, t.Branch) {
				c.Branches = append(c.Branches, t.Branch)
			}
		case t.Done:
			c.Tasks = append(c.Tasks, t)
			c.Done++
			if t.Subtask {
				c.DoneSubtasks++
			}
		}
	}
	slices.Sort(c.Branches)
	return c
}

// Removal is a planned deletion of tasks. Apply writes it, and Undo puts
// the file back as long as nothing else has changed it in between.
type Removal struct {
	// Tasks are the tasks removed, leaving out subtasks that go with their
	// task.
	Tasks      []Task
	Categories []string
	Branches   []string

	path, before, after string
	mode                os.FileMode
}

// PlanRemove works out the file without tasks, and which category and branch
// headings they leave empty, without writing it.
func PlanRemove(path string, tasks []Task) (Removal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Removal{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Removal{}, err
	}
	r := Removal{Tasks: withoutNested(tasks), path: path, before: string(data), mode: info.Mode().Perm()}
	// Removing from the bottom up keeps the line numbers of the tasks and
	// headings still to come valid.
	slices.SortFunc(r.Tasks, func(a, b Task) int { return cmp.Compare(b.Line, a.Line) })
	lines, format := decode(data)
	for _, t := range r.Tasks {
		if !taskUnchanged(lines, t) {
			return Removal{}, ErrTaskChanged
		}
		lines = append(lines[:t.Line:t.Line], lines[t.bodyEnd:]...)
		if kept := removeEmptyCategoryHeading(lines, t); len(kept) < len(lines) {
			lines = kept
			r.Categories = append(r.Categories, t.Category)
		}
		if t.Branch == "" {
			continue
		}
		if kept := removeEmptyBranchHeading(lines, t.Branch); len(kept) < len(lines) {
			lines = removeEmptyBranchesHeading(kept)
			r.Branches = append(r.Branches, t.Branch)
		}
	}
	slices.Sort(r.Categories)
	slices.Sort(r.Branches)
	r.after = format.encode(lines)
	return r, nil
}

// Apply writes the planned file, refusing if it changed since it was planned.
func (r Removal) Apply() error {
	return r.replace(r.before, r.after)
}

// Undo restores the file from before Apply, refusing if it changed since.
func (r Removal) Undo() error {
	return r.replace(r.after, r.before)
}

func (r Removal) replace(expected, updated string) error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}
	if string(data) != expected {
		return ErrFileChanged
	}
	return replaceFile(r.path, []byte(updated), r.mode)
}

// withoutNested is tasks without those another of them holds, such as a
// subtask of a task also given, which goes with it.
func withoutNested(tasks []Task) []Task {
	return slices.DeleteFunc(slices.Clone(tasks), func(t Task) bool {
		return slices.ContainsFunc(tasks, func(other Task) bool { return other.Holds(t) })
	})
}

func removeEmptyBranchesHeading(lines []string) []string {
	start, end, _ := findBranchesSection(lines)
	if start < 0 {
		return lines
	}
	if !allBlank(lines[start+1 : end]) {
		return lines
	}
	return slices.Concat(lines[:start], lines[end:])
}
