package main

import (
	"sort"
	"strings"
)

type navigationKind int

const (
	rowTask navigationKind = iota
	rowLabel
	rowBranch
)

type navigationRow struct {
	kind             navigationKind
	name             string
	count            int
	completed        int
	missingGitBranch bool
	todo             task
}

func completedCount(tasks []task) int {
	count := 0
	for _, t := range tasks {
		if t.done {
			count++
		}
	}
	return count
}

func (m *model) generalRows() []navigationRow {
	var rows []navigationRow
	if m.generalLabel == "" {
		counts := make(map[string]int)
		completed := make(map[string]int)
		display := make(map[string]string)
		for _, t := range m.general {
			if label := taskLabel(t); label != "" {
				key := strings.ToLower(label)
				counts[key]++
				if t.done {
					completed[key]++
				}
				if display[key] == "" {
					display[key] = label
				}
			}
		}
		for _, key := range sortedNames(counts) {
			label := display[key]
			rows = append(rows, navigationRow{kind: rowLabel, name: label, count: counts[key], completed: completed[key]})
		}
		taskStart := len(rows)
		for _, t := range m.general {
			if taskLabel(t) == "" {
				rows = append(rows, navigationRow{kind: rowTask, todo: t})
			}
		}
		openTasksFirst(rows[taskStart:])
		return rows
	}
	for _, t := range m.general {
		if taskHasLabel(t, m.generalLabel) {
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
			if t.branch == m.branchFilter {
				rows = append(rows, navigationRow{kind: rowTask, todo: t})
			}
		}
		openTasksFirst(rows)
		return rows
	}
	counts := make(map[string]int)
	completed := make(map[string]int)
	for _, t := range m.branches {
		counts[t.branch]++
		if t.done {
			completed[t.branch]++
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
		return !rows[i].todo.done && rows[j].todo.done
	})
}

func (m *model) selectNavigationTask(selected task) {
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
	for i, row := range rows {
		if row.kind == rowTask && row.todo.line == selected.line && row.todo.text == selected.text &&
			row.todo.branch == selected.branch && strings.EqualFold(taskLabel(row.todo), taskLabel(selected)) {
			*cursor = i
			return
		}
	}
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

func sortedTasksByPriority(tasks []task) []task {
	sorted := append([]task(nil), tasks...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].priority.rank() < sorted[j].priority.rank()
	})
	return sorted
}

func taskHasLabel(t task, label string) bool {
	return strings.EqualFold(taskLabel(t), label)
}

func taskLabel(t task) string {
	if len(t.labels) == 0 {
		return ""
	}
	return t.labels[0]
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
	case rowLabel:
		m.generalRootCursor = m.generalCursor
		m.generalLabel = row.name
		m.generalCursor = 0
	case rowBranch:
		m.branchRootCursor = m.branchCursor
		m.branchFilter = row.name
		m.branchCursor = 0
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
		if m.generalLabel == "" {
			return false
		}
		m.generalLabel = ""
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
