package main

import (
	"sort"
	"strings"

	"github.com/nebarg/todo-cli/internal/store"
)

type navigationKind int

const (
	rowTask navigationKind = iota
	rowCategory
	rowBranch
)

type navigationRow struct {
	kind             navigationKind
	name             string
	count            int
	completed        int
	missingGitBranch bool
	todo             store.Task
}

func completedCount(tasks []store.Task) int {
	count := 0
	for _, t := range tasks {
		if t.Done {
			count++
		}
	}
	return count
}

func (m *model) generalRows() []navigationRow {
	var rows []navigationRow
	if m.generalCategory == "" {
		counts := make(map[string]int)
		completed := make(map[string]int)
		display := make(map[string]string)
		for _, t := range m.general {
			if t.Category != "" {
				key := strings.ToLower(t.Category)
				counts[key]++
				if t.Done {
					completed[key]++
				}
				if display[key] == "" {
					display[key] = t.Category
				}
			}
		}
		for _, key := range sortedNames(counts) {
			rows = append(rows, navigationRow{kind: rowCategory, name: display[key], count: counts[key], completed: completed[key]})
		}
		taskStart := len(rows)
		for _, t := range m.general {
			if t.Category == "" {
				rows = append(rows, navigationRow{kind: rowTask, todo: t})
			}
		}
		openTasksFirst(rows[taskStart:])
		return rows
	}
	for _, t := range m.general {
		if taskInCategory(t, m.generalCategory) {
			rows = append(rows, navigationRow{kind: rowTask, todo: t})
		}
	}
	openTasksFirst(rows)
	return rows
}

func (m *model) branchRows() []navigationRow {
	var rows []navigationRow
	if m.branchFilter != "" {
		for _, t := range m.branches {
			if t.Branch == m.branchFilter {
				rows = append(rows, navigationRow{kind: rowTask, todo: t})
			}
		}
		openTasksFirst(rows)
		return rows
	}
	counts := make(map[string]int)
	completed := make(map[string]int)
	for _, t := range m.branches {
		counts[t.Branch]++
		if t.Done {
			completed[t.Branch]++
		}
	}
	for _, name := range sortedNames(counts) {
		rows = append(rows, navigationRow{kind: rowBranch, name: name, count: counts[name], completed: completed[name], missingGitBranch: m.branchMissing(name)})
	}
	return rows
}

func (m *model) branchMissing(name string) bool {
	return name != "" && m.branchesVerified && !m.localBranchNames[name]
}

func openTasksFirst(rows []navigationRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return !rows[i].todo.Done && rows[j].todo.Done
	})
}

func (m *model) selectNavigationTask(selected store.Task) {
	var rows []navigationRow
	var cursor *int
	switch m.activePane() {
	case generalPane:
		rows, cursor = m.generalRows(), &m.generalCursor
	case branchPane:
		rows, cursor = m.branchRows(), &m.branchCursor
	default:
		return
	}
	// Writes can move the task in the file, so the nearest task with the same
	// title in the same place is taken to be it.
	best, distance := -1, int(^uint(0)>>1)
	for i, row := range rows {
		if row.kind != rowTask || row.todo.Text != selected.Text || row.todo.Branch != selected.Branch || !strings.EqualFold(row.todo.Category, selected.Category) {
			continue
		}
		if d := abs(row.todo.Line - selected.Line); d < distance {
			best, distance = i, d
		}
	}
	if best >= 0 {
		*cursor = best
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sortedNames(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := strings.ToLower(names[i]), strings.ToLower(names[j])
		if a == b {
			return names[i] < names[j]
		}
		return a < b
	})
	return names
}

func sortedTasksByPriority(tasks []store.Task) []store.Task {
	sorted := append([]store.Task(nil), tasks...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority.Rank() < sorted[j].Priority.Rank()
	})
	return sorted
}

func taskInCategory(t store.Task, category string) bool {
	return strings.EqualFold(t.Category, category)
}

func (m *model) selectedNavigationRow() (navigationRow, bool) {
	var rows []navigationRow
	var cursor int
	switch m.activePane() {
	case generalPane:
		rows, cursor = m.generalRows(), m.generalCursor
	case branchPane:
		rows, cursor = m.branchRows(), m.branchCursor
	default:
		return navigationRow{}, false
	}
	if cursor < 0 || cursor >= len(rows) {
		return navigationRow{}, false
	}
	return rows[cursor], true
}

func (m *model) enterSelectedGroup() bool {
	if m.focus == detailPane {
		return false
	}
	row, ok := m.selectedNavigationRow()
	if !ok {
		return false
	}
	switch row.kind {
	case rowCategory:
		m.generalRootCursor = m.generalCursor
		m.generalCategory = row.name
		m.generalCursor = 0
	case rowBranch:
		m.branchRootCursor = m.branchCursor
		m.branchFilter = row.name
		m.branchCursor = 0
		m.recheckBranch(row.name)
	default:
		return false
	}
	m.detailScroll = 0
	m.status = ""
	return true
}

func (m *model) leaveGroup() bool {
	switch m.focus {
	case generalPane:
		if m.generalCategory == "" {
			return false
		}
		m.generalCategory = ""
		m.generalCursor = m.generalRootCursor
	case branchPane:
		if m.branchFilter == "" {
			return false
		}
		m.branchFilter = ""
		m.branchCursor = m.branchRootCursor
	default:
		return false
	}
	m.detailScroll = 0
	m.status = ""
	return true
}

func (m *model) preselectCurrentBranch() {
	if m.project.branch == "" || m.branchFilter != "" {
		return
	}
	for i, row := range m.branchRows() {
		if row.name == m.project.branch {
			m.branchCursor = i
			return
		}
	}
}
