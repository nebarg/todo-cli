package main

import (
	"sort"
	"strings"

	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
)

type navigationKind int

const (
	rowTask navigationKind = iota
	rowCategory
	rowBranch
	rowFileCategory
	rowFile
	rowReadme
)

// readmeGroup names the General group of README.md's tasks.
const readmeGroup = "README.md"

type navigationRow struct {
	kind             navigationKind
	name             string
	count            int
	completed        int
	missingGitBranch bool
	todo             store.Task
	match            scan.Match
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
	if m.readmeOpen {
		for _, t := range m.readme {
			rows = append(rows, navigationRow{kind: rowTask, todo: t})
		}
		openTasksFirst(rows)
		return rows
	}
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
		if len(m.readme) > 0 {
			rows = append(rows, navigationRow{kind: rowReadme, name: readmeGroup, count: len(m.readme), completed: completedCount(m.readme)})
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

// sourceRows lists file TODOs as General lists tasks: todo@category rows
// first, then the rest, most urgent first as the scan returned them.
func (m *model) sourceRows() []navigationRow {
	var rows []navigationRow
	if m.sourceCategory != "" {
		for _, match := range m.source {
			if strings.EqualFold(match.Category, m.sourceCategory) {
				rows = append(rows, navigationRow{kind: rowFile, match: match})
			}
		}
		return rows
	}
	counts := make(map[string]int)
	display := make(map[string]string)
	for _, match := range m.source {
		if match.Category != "" {
			key := strings.ToLower(match.Category)
			counts[key]++
			if display[key] == "" {
				display[key] = match.Category
			}
		}
	}
	for _, key := range sortedNames(counts) {
		rows = append(rows, navigationRow{kind: rowFileCategory, name: display[key], count: counts[key]})
	}
	for _, match := range m.source {
		if match.Category == "" {
			rows = append(rows, navigationRow{kind: rowFile, match: match})
		}
	}
	return rows
}

func (m *model) selectedSource() (scan.Match, bool) {
	rows := m.sourceRows()
	if m.sourceCursor < 0 || m.sourceCursor >= len(rows) || rows[m.sourceCursor].kind != rowFile {
		return scan.Match{}, false
	}
	return rows[m.sourceCursor].match, true
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
	if m.focus == sourcePane {
		rows := m.sourceRows()
		if m.sourceCursor >= len(rows) || rows[m.sourceCursor].kind != rowFileCategory {
			return false
		}
		m.sourceRootCursor = m.sourceCursor
		m.sourceCategory = rows[m.sourceCursor].name
		m.sourceCursor = 0
		m.detailScroll = 0
		m.status = ""
		return true
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
		m.openBranch(m.branchCursor, row.name)
	case rowReadme:
		m.generalRootCursor = m.generalCursor
		m.readmeOpen = true
		m.generalCursor = 0
	default:
		return false
	}
	m.detailScroll = 0
	m.status = ""
	return true
}

// jumpToTab focuses a tab, or leaves its opened category or branch when it
// already has focus. At the top of Branches it opens the current branch.
func (m *model) jumpToTab(p pane) {
	switch {
	case m.focus != p:
		m.focus = p
		m.detailScroll = 0
	case p == branchPane && m.branchFilter == "":
		m.openCurrentBranch()
	default:
		m.leaveGroup()
	}
}

func (m *model) openBranch(rootCursor int, name string) {
	m.branchRootCursor = rootCursor
	m.branchFilter = name
	m.branchCursor = 0
	m.recheckBranch(name)
}

func (m *model) leaveGroup() {
	switch m.focus {
	case generalPane:
		if m.generalCategory == "" && !m.readmeOpen {
			return
		}
		m.generalCategory, m.readmeOpen = "", false
		m.generalCursor = m.generalRootCursor
	case branchPane:
		if m.branchFilter == "" {
			return
		}
		m.branchFilter = ""
		m.branchCursor = m.branchRootCursor
	case sourcePane:
		if m.sourceCategory == "" {
			return
		}
		m.sourceCategory = ""
		m.sourceCursor = m.sourceRootCursor
	default:
		return
	}
	m.detailScroll = 0
	m.status = ""
}

// openCurrentBranch opens the current Git branch's tasks, if it has any.
func (m *model) openCurrentBranch() {
	if m.project.branch == "" || m.branchFilter != "" {
		return
	}
	for i, row := range m.branchRows() {
		if row.name == m.project.branch {
			m.openBranch(i, row.name)
			m.detailScroll = 0
			m.status = ""
			return
		}
	}
}
