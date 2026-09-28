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
	rowBranchLabel
)

type navigationRow struct {
	kind  navigationKind
	name  string
	count int
	todo  task
}

func (m model) generalRows() []navigationRow {
	var rows []navigationRow
	if m.generalLabel == "" {
		counts := make(map[string]int)
		display := make(map[string]string)
		for _, t := range m.allTasks {
			if label := taskLabel(t); label != "" {
				key := strings.ToLower(label)
				counts[key]++
				if display[key] == "" {
					display[key] = label
				}
			}
		}
		for _, key := range sortedNames(counts) {
			label := display[key]
			rows = append(rows, navigationRow{kind: rowLabel, name: label, count: counts[key]})
			for _, t := range m.allTasks {
				if taskHasLabel(t, label) {
					rows = append(rows, navigationRow{kind: rowTask, todo: t})
				}
			}
		}
		for _, t := range m.general {
			if taskLabel(t) == "" {
				rows = append(rows, navigationRow{kind: rowTask, todo: t})
			}
		}
		return rows
	}
	for _, t := range m.allTasks {
		if taskHasLabel(t, m.generalLabel) {
			rows = append(rows, navigationRow{kind: rowTask, todo: t})
		}
	}
	return rows
}

func (m model) branchRows() []navigationRow {
	var rows []navigationRow
	if m.branchFilter != "" {
		counts := make(map[string]int)
		display := make(map[string]string)
		for _, t := range m.branches {
			if t.branch == m.branchFilter && taskLabel(t) != "" {
				label := taskLabel(t)
				key := strings.ToLower(label)
				counts[key]++
				if display[key] == "" {
					display[key] = label
				}
			}
		}
		for _, key := range sortedNames(counts) {
			label := display[key]
			rows = append(rows, navigationRow{kind: rowBranchLabel, name: label, count: counts[key]})
			for _, t := range m.branches {
				if t.branch == m.branchFilter && taskHasLabel(t, label) {
					rows = append(rows, navigationRow{kind: rowTask, todo: t})
				}
			}
		}
		for _, t := range m.branches {
			if t.branch == m.branchFilter && taskLabel(t) == "" {
				rows = append(rows, navigationRow{kind: rowTask, todo: t})
			}
		}
		return rows
	}
	counts := make(map[string]int)
	for _, t := range m.branches {
		counts[t.branch]++
	}
	for _, name := range sortedNames(counts) {
		rows = append(rows, navigationRow{kind: rowBranch, name: name, count: counts[name]})
	}
	return rows
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

func taskHasLabel(t task, label string) bool {
	return strings.EqualFold(taskLabel(t), label)
}

func taskLabel(t task) string {
	if len(t.labels) == 0 {
		return ""
	}
	return t.labels[0]
}

func (m model) selectedNavigationRow() (navigationRow, bool) {
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
	case rowBranchLabel:
		if m.branchCursor+1 < len(m.branchRows()) {
			m.branchCursor++
		}
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
