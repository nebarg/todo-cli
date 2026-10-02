package main

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/nebarg/todo-cli/internal/store"
)

type navigationKind int

const (
	rowTask navigationKind = iota
	rowCategory
	rowBranch
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

func (m *model) branchMissing(name string) bool {
	return name != "" && m.branchesVerified && !m.localBranchNames[name]
}

func openTasksFirst(rows []navigationRow) {
	slices.SortStableFunc(rows, func(a, b navigationRow) int { return compareDone(a.todo, b.todo) })
}

// compareDone orders open tasks before done ones.
func compareDone(a, b store.Task) int {
	switch {
	case a.Done == b.Done:
		return 0
	case a.Done:
		return 1
	}
	return -1
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
	if i := nearestTask(rowTasks(rows), selected); i >= 0 {
		*cursor = i
	}
}

// reveal opens the category or branch t is filed under, in its own tab, and
// selects it there.
func (m *model) reveal(t store.Task) {
	if t.Branch != "" {
		m.branchFilter = ""
		m.branchRootCursor = max(0, slices.IndexFunc(m.branchRows(), func(row navigationRow) bool { return row.name == t.Branch }))
		m.branchFilter = t.Branch
		m.branchCursor = max(0, nearestTask(rowTasks(m.branchRows()), t))
		return
	}
	m.generalCategory, m.readmeOpen = "", false
	if t.Category != "" {
		m.generalRootCursor = max(0, slices.IndexFunc(m.generalRows(), func(row navigationRow) bool {
			return row.kind == rowCategory && strings.EqualFold(row.name, t.Category)
		}))
	}
	m.generalCategory = t.Category
	m.generalCursor = max(0, nearestTask(rowTasks(m.generalRows()), t))
}

// nearestTask finds t in tasks after a write, which can move it in the file:
// the task with its title, category and branch nearest its old line, or -1.
func nearestTask(tasks []store.Task, t store.Task) int {
	best, distance := -1, math.MaxInt
	for i, candidate := range tasks {
		if candidate.Text != t.Text || candidate.Branch != t.Branch || !strings.EqualFold(candidate.Category, t.Category) {
			continue
		}
		if d := abs(candidate.Line - t.Line); d < distance {
			best, distance = i, d
		}
	}
	return best
}

// rowTasks is each row's task; a group row's is empty, so it matches none.
func rowTasks(rows []navigationRow) []store.Task {
	tasks := make([]store.Task, len(rows))
	for i, row := range rows {
		tasks[i] = row.todo
	}
	return tasks
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
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Or(strings.Compare(strings.ToLower(a), strings.ToLower(b)), strings.Compare(a, b))
	})
	return names
}

func sortedTasksByPriority(tasks []store.Task) []store.Task {
	sorted := slices.Clone(tasks)
	slices.SortStableFunc(sorted, func(a, b store.Task) int { return cmp.Compare(a.Priority.Rank(), b.Priority.Rank()) })
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
		m.files.CloseDetails()
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
