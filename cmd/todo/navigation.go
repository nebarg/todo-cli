package main

import (
	"cmp"
	"math"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
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

// group is a category, a branch or the README.md group, opened from the top
// of a list. The zero group is the top level.
type group struct {
	kind navigationKind // rowCategory, rowBranch or rowReadme
	name string
}

func categoryGroup(name string) group {
	if name == "" {
		return group{}
	}
	return group{kind: rowCategory, name: name}
}

func branchGroup(name string) group {
	if name == "" {
		return group{}
	}
	return group{kind: rowBranch, name: name}
}

// openedBy reports whether row opens g from the top of its list. Category
// names ignore case; branch names don't.
func (g group) openedBy(row navigationRow) bool {
	return row.kind == g.kind && (row.name == g.name || g.kind == rowCategory && strings.EqualFold(row.name, g.name))
}

// groupList is where a tab is in its list: at the top, among its groups and
// loose tasks, or in the group opened from there.
type groupList struct {
	open       group
	cursor     int
	rootCursor int // the cursor at the top, to return to
}

// enter opens g, remembering where the cursor was at the top.
func (l *groupList) enter(g group) {
	l.rootCursor, l.open, l.cursor = l.cursor, g, 0
}

// leave goes back to the top, reporting false when the list was already there.
func (l *groupList) leave() bool {
	if l.open == (group{}) {
		return false
	}
	l.open, l.cursor = group{}, l.rootCursor
	return true
}

// list is the General or Branches tab's list, or nil for another pane.
func (m *model) list(p pane) *groupList {
	switch p {
	case generalPane:
		return &m.general
	case branchPane:
		return &m.branch
	}
	return nil
}

// rows is what the General or Branches tab lists where it is.
func (m *model) rows(p pane) []navigationRow {
	if l := m.list(p); l != nil {
		return m.rowsIn(p, l.open)
	}
	return nil
}

// rowsIn is what the General or Branches tab lists with open opened.
func (m *model) rowsIn(p pane, open group) []navigationRow {
	if p == branchPane {
		return branchRows(m.tasks.branches, open, m.branchMissing)
	}
	return generalRows(m.tasks.general, m.tasks.readme, open)
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

// generalRows lists General: at the top, its categories, the README.md group
// and its tasks without a category; or the tasks of the group open.
func generalRows(general, readme []store.Task, open group) []navigationRow {
	switch open.kind {
	case rowReadme:
		return taskRows(readme, func(store.Task) bool { return true })
	case rowCategory:
		return taskRows(general, func(t store.Task) bool { return taskInCategory(t, open.name) })
	}
	counts := make(map[string]int)
	completed := make(map[string]int)
	display := make(map[string]string)
	for _, t := range general {
		if t.Category == "" {
			continue
		}
		key := strings.ToLower(t.Category)
		counts[key]++
		if t.Done {
			completed[key]++
		}
		if display[key] == "" {
			display[key] = t.Category
		}
	}
	var rows []navigationRow
	for _, key := range sortedNames(counts) {
		rows = append(rows, navigationRow{kind: rowCategory, name: display[key], count: counts[key], completed: completed[key]})
	}
	if len(readme) > 0 {
		rows = append(rows, navigationRow{kind: rowReadme, name: readmeGroup, count: len(readme), completed: completedCount(readme)})
	}
	return append(rows, taskRows(general, func(t store.Task) bool { return t.Category == "" })...)
}

// branchRows lists Branches: a row for each branch at the top, flagging
// those missing reports gone; or the tasks of the branch open.
func branchRows(branches []store.Task, open group, missing func(branch string) bool) []navigationRow {
	if open != (group{}) {
		return taskRows(branches, func(t store.Task) bool { return t.Branch == open.name })
	}
	counts := make(map[string]int)
	completed := make(map[string]int)
	for _, t := range branches {
		counts[t.Branch]++
		if t.Done {
			completed[t.Branch]++
		}
	}
	var rows []navigationRow
	for _, name := range sortedNames(counts) {
		rows = append(rows, navigationRow{kind: rowBranch, name: name, count: counts[name], completed: completed[name], missingGitBranch: missing(name)})
	}
	return rows
}

// taskRows is a row for each task keep accepts, open tasks first.
func taskRows(tasks []store.Task, keep func(store.Task) bool) []navigationRow {
	var rows []navigationRow
	for _, t := range tasks {
		if keep(t) {
			rows = append(rows, navigationRow{kind: rowTask, todo: t})
		}
	}
	openTasksFirst(rows)
	return rows
}

func (m *model) branchMissing(name string) bool {
	_, listed := slices.BinarySearch(m.localBranches, name)
	return name != "" && m.branchesVerified && !listed
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
	p := m.activePane()
	if l := m.list(p); l != nil {
		if i := nearestTask(rowTasks(m.rows(p)), selected); i >= 0 {
			l.cursor = i
		}
	}
}

// reveal opens the category or branch t is filed under, in its own tab, and
// selects it there.
func (m *model) reveal(t store.Task) {
	p, g := generalPane, categoryGroup(t.Category)
	if t.Branch != "" {
		p, g = branchPane, branchGroup(t.Branch)
	}
	l := m.list(p)
	if g != (group{}) {
		l.rootCursor = max(0, slices.IndexFunc(m.rowsIn(p, group{}), g.openedBy))
	}
	l.open = g
	l.cursor = max(0, nearestTask(rowTasks(m.rowsIn(p, g)), t))
}

// nearestTask finds t in tasks after a write, which can move it in the file:
// the task with its title, category and branch nearest its old line, or -1.
func nearestTask(tasks []store.Task, t store.Task) int {
	best, distance := -1, math.MaxInt
	for i, candidate := range tasks {
		if candidate.Text != t.Text || !candidate.Same(t.Section) {
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
	p := m.activePane()
	l := m.list(p)
	if l == nil {
		return navigationRow{}, false
	}
	rows := m.rows(p)
	if l.cursor < 0 || l.cursor >= len(rows) {
		return navigationRow{}, false
	}
	return rows[l.cursor], true
}

// enterSelectedGroup opens the selected category, branch or README.md
// group, reporting false when the selection isn't one.
func (m *model) enterSelectedGroup() (tea.Cmd, bool) {
	if m.focus == detailPane {
		return nil, false
	}
	row, ok := m.selectedNavigationRow()
	if !ok || row.kind == rowTask {
		return nil, false
	}
	var cmd tea.Cmd
	if row.kind == rowBranch {
		cmd = m.enterBranch(row.name)
	} else {
		m.general.enter(group{kind: row.kind, name: row.name})
	}
	m.detailScroll = 0
	m.status = ""
	return cmd, true
}

// jumpToTab focuses a tab, or leaves its opened category or branch when it
// already has focus. At the top of Branches it opens the current branch.
func (m *model) jumpToTab(p pane) tea.Cmd {
	switch {
	case m.focus != p:
		m.focus = p
		m.detailScroll = 0
		m.files.CloseDetails()
	case p == branchPane && m.branch.open == (group{}):
		if m.openCurrentBranch() {
			return m.checkBranches()
		}
	default:
		m.leaveGroup()
	}
	return nil
}

// enterBranch opens a branch from the top of Branches, and asks Git in the
// background whether it still has it.
func (m *model) enterBranch(name string) tea.Cmd {
	m.branch.enter(branchGroup(name))
	return m.checkBranches()
}

func (m *model) leaveGroup() {
	if l := m.list(m.focus); l == nil || !l.leave() {
		return
	}
	m.detailScroll = 0
	m.status = ""
}

// openCurrentBranch opens the current Git branch's tasks, if it has any,
// reporting whether it did.
func (m *model) openCurrentBranch() bool {
	if m.project.branch == "" || m.branch.open != (group{}) {
		return false
	}
	i := slices.IndexFunc(m.rows(branchPane), branchGroup(m.project.branch).openedBy)
	if i < 0 {
		return false
	}
	m.branch.cursor = i
	m.branch.enter(branchGroup(m.project.branch))
	m.detailScroll = 0
	m.status = ""
	return true
}
