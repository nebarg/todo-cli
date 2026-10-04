package store

import (
	"slices"
	"strings"
)

// sortSection orders the tasks of section s by priority, keeping the file's
// order among equals. Being done doesn't move a task, so one reopened is back
// where it was. Each task moves with its details and subtasks, which keep
// their order; notes above the first task and nested headings stay where
// they are.
func sortSection(content string, s Section) string {
	lines := strings.Split(content, "\n")
	var runs [][]Task
	for _, t := range parseTasks(lines) {
		if t.Subtask || !t.Same(s) {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1][len(runs[n-1])-1].bodyEnd == t.Line {
			runs[n-1] = append(runs[n-1], t)
		} else {
			runs = append(runs, []Task{t})
		}
	}
	// Later runs first, so earlier runs' line numbers stay valid.
	for _, run := range slices.Backward(runs) {
		lines = sortRun(lines, run)
	}
	return strings.Join(lines, "\n")
}

// sortRun reorders tasks that follow one another directly, separating them
// with blank lines when most of them were. A tie keeps them compact, as a
// newly added task arrives after a blank line of its own.
func sortRun(lines []string, run []Task) []string {
	if len(run) < 2 {
		return lines
	}
	blocks := make([][]string, len(run))
	blank, spaced := "", 0
	for i, t := range run {
		block := lines[t.Line:t.bodyEnd]
		end := trimBlankEnd(block, len(block), 1)
		if end < len(block) && i < len(run)-1 {
			spaced++
			blank = block[end]
		}
		blocks[i] = block[:end]
	}
	last := run[len(run)-1]
	trailer := lines[last.Line+len(blocks[len(run)-1]) : last.bodyEnd]

	order := make([]int, len(run))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return run[a].Priority.Rank() - run[b].Priority.Rank() })

	result := slices.Clone(lines[:run[0].Line])
	for i, index := range order {
		if i > 0 && spaced*2 > len(run)-1 {
			result = append(result, blank)
		}
		result = append(result, blocks[index]...)
	}
	result = append(result, trailer...)
	return append(result, lines[last.bodyEnd:]...)
}
