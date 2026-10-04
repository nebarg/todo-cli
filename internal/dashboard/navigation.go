package dashboard

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
	rowReadmeHeading
	rowReadmeTask
)

// readmeGroup names the General group of README.md's tasks.
const readmeGroup = "README.md"

type navigationRow struct {
	kind             navigationKind
	name             string
	count            int
	completed        int
	missingGitBranch bool
	todo             store.Task       // a rowTask's task
	readme           store.ReadmeTask // a rowReadmeTask's task
	parentDone       bool             // a README subtask's parent is done, so it shows greyed out
}

// isTask reports whether row is a task of todo.md or README.md, not a group.
func (r navigationRow) isTask() bool {
	return r.kind == rowTask || r.kind == rowReadmeTask
}

// done reports whether a task row's task is done.
func (r navigationRow) done() bool {
	if r.kind == rowReadmeTask {
		return r.readme.Done
	}
	return r.todo.Done
}

// group is a category, a branch or the README.md group, opened from the top
// of a list, or a heading opened from the README.md group. The zero group is
// the top level.
type group struct {
	kind navigationKind // rowCategory, rowBranch, rowReadme or rowReadmeHeading
	name string
}

// inReadme reports whether g is the README.md group or one of its headings.
func (g group) inReadme() bool {
	return g.kind == rowReadme || g.kind == rowReadmeHeading
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
// loose tasks, in the group opened from there, or in a README.md heading.
type groupList struct {
	open         group
	cursor       int
	rootCursor   int // the cursor at the top, to return to
	readmeCursor int // the cursor in the README.md group, to return to from a heading
}

// enter opens g, remembering where the cursor was in the list it leaves.
func (l *groupList) enter(g group) {
	if g.kind == rowReadmeHeading {
		l.readmeCursor = l.cursor
	} else {
		l.rootCursor = l.cursor
	}
	l.open, l.cursor = g, 0
}

// leave goes back one level, reporting false when the list was already at
// the top.
func (l *groupList) leave() bool {
	switch {
	case l.open.kind == rowReadmeHeading:
		l.open, l.cursor = group{kind: rowReadme, name: readmeGroup}, l.readmeCursor
	case l.open != (group{}):
		l.open, l.cursor = group{}, l.rootCursor
	default:
		return false
	}
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

func completedReadmeCount(tasks []store.ReadmeTask) int {
	count := 0
	for _, t := range tasks {
		if t.Done {
			count++
		}
	}
	return count
}

// generalRows lists General: at the top, its categories, the README.md group
// and its tasks without a category; or what the group open lists.
func generalRows(general []store.Task, readme []store.ReadmeTask, open group) []navigationRow {
	switch open.kind {
	case rowReadme:
		return readmeRows(readme)
	case rowReadmeHeading:
		return readmeTaskRows(readme, open.name)
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
		rows = append(rows, navigationRow{kind: rowReadme, name: readmeGroup, count: len(readme), completed: completedReadmeCount(readme)})
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

// taskRows is a row for each task keep accepts, in the dashboard's order.
func taskRows(tasks []store.Task, keep func(store.Task) bool) []navigationRow {
	var rows []navigationRow
	for _, t := range tasks {
		if keep(t) {
			rows = append(rows, navigationRow{kind: rowTask, todo: t})
		}
	}
	return rows
}

// readmeRows lists the README.md group: a row for each heading inside its
// TODO sections, in the README's order, then the tasks under no heading.
func readmeRows(tasks []store.ReadmeTask) []navigationRow {
	var rows []navigationRow
	index := make(map[string]int)     // each heading's row
	firstLine := make(map[string]int) // each heading's first task, to order the rows by
	for _, t := range tasks {
		if t.Heading == "" {
			continue
		}
		i, ok := index[t.Heading]
		if !ok {
			i, index[t.Heading], firstLine[t.Heading] = len(rows), len(rows), t.Line
			rows = append(rows, navigationRow{kind: rowReadmeHeading, name: t.Heading})
		}
		rows[i].count++
		if t.Done {
			rows[i].completed++
		}
		firstLine[t.Heading] = min(firstLine[t.Heading], t.Line)
	}
	slices.SortFunc(rows, func(a, b navigationRow) int { return cmp.Compare(firstLine[a.name], firstLine[b.name]) })
	return append(rows, readmeTaskRows(tasks, "")...)
}

// readmeTaskRows is a row for each README task under heading, in the
// dashboard's order, with each task's subtasks after it.
func readmeTaskRows(tasks []store.ReadmeTask, heading string) []navigationRow {
	parents := readmeParents(tasks)
	subtasks := make(map[int][]store.ReadmeTask) // by their parent's line
	for _, t := range tasks {
		if t.Heading == heading && t.Subtask {
			subtasks[parents[t.Line]] = append(subtasks[parents[t.Line]], t)
		}
	}
	var rows []navigationRow
	for _, t := range tasks {
		if t.Heading != heading || t.Subtask {
			continue
		}
		rows = append(rows, navigationRow{kind: rowReadmeTask, readme: t})
		for _, sub := range subtasks[t.Line] {
			rows = append(rows, navigationRow{kind: rowReadmeTask, readme: sub, parentDone: t.Done})
		}
	}
	return rows
}

// readmeParents maps each subtask's line to its parent's line: the last task
// before it in the README that isn't a subtask.
func readmeParents(tasks []store.ReadmeTask) map[int]int {
	inFile := slices.SortedFunc(slices.Values(tasks), func(a, b store.ReadmeTask) int { return cmp.Compare(a.Line, b.Line) })
	parents := make(map[int]int)
	parent := -1
	for _, t := range inFile {
		if t.Subtask {
			parents[t.Line] = parent
		} else {
			parent = t.Line
		}
	}
	return parents
}

func (m *model) branchMissing(name string) bool {
	_, listed := slices.BinarySearch(m.localBranches, name)
	return name != "" && m.branchesVerified && !listed
}

// compareDone orders open tasks before done ones.
func compareDone(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
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

// selectReadmeTask puts the cursor on t after a write, which can move it in
// the open README.md group or heading: the task with its text nearest its
// old line.
func (m *model) selectReadmeTask(t store.ReadmeTask) {
	best, distance := -1, math.MaxInt
	for i, row := range m.rows(generalPane) {
		if row.kind != rowReadmeTask || row.readme.Text != t.Text {
			continue
		}
		if d := abs(row.readme.Line - t.Line); d < distance {
			best, distance = i, d
		}
	}
	if best >= 0 {
		m.general.cursor = best
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
// Of identical tasks, those with t's done state and priority come first.
func nearestTask(tasks []store.Task, t store.Task) int {
	best, bestDiffers, distance := -1, math.MaxInt, math.MaxInt
	for i, candidate := range tasks {
		if candidate.Text != t.Text || !candidate.Same(t.Section) {
			continue
		}
		differs := 0
		if candidate.Done != t.Done {
			differs++
		}
		if candidate.Priority != t.Priority {
			differs++
		}
		d := abs(candidate.Line - t.Line)
		if differs < bestDiffers || differs == bestDiffers && d < distance {
			best, bestDiffers, distance = i, differs, d
		}
	}
	return best
}

// rowTasks is each row's todo.md task; a group or README task row's is empty,
// so it matches none.
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

// sortedTasks is tasks as the dashboard shows them after loading: open
// tasks by priority, then done tasks by priority.
func sortedTasks(tasks []store.Task) []store.Task {
	sorted := slices.Clone(tasks)
	slices.SortStableFunc(sorted, func(a, b store.Task) int {
		return cmp.Or(compareDone(a.Done, b.Done), cmp.Compare(a.Priority.Rank(), b.Priority.Rank()))
	})
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

// enterSelectedGroup opens the selected category, branch, README.md group
// or README heading, reporting false when the selection isn't one.
func (m *model) enterSelectedGroup() bool {
	if m.focus == detailPane {
		return false
	}
	row, ok := m.selectedNavigationRow()
	if !ok || row.isTask() {
		return false
	}
	if row.kind == rowBranch {
		m.branch.enter(branchGroup(row.name))
	} else {
		m.general.enter(group{kind: row.kind, name: row.name})
	}
	m.detailScroll = 0
	m.status = ""
	return true
}

// jumpToTab focuses a tab, or goes back to its top when it already has
// focus. At the top of Branches it opens the current branch.
func (m *model) jumpToTab(p pane) {
	switch {
	case m.focus != p:
		m.focus = p
		m.detailScroll = 0
		m.files.CloseDetails()
	case p == branchPane && m.branch.open == (group{}):
		m.openCurrentBranch()
	default:
		for m.leaveGroup() {
		}
	}
}

// leaveGroup goes back one level in the focused list, reporting false when
// it was already at the top.
func (m *model) leaveGroup() bool {
	if l := m.list(m.focus); l == nil || !l.leave() {
		return false
	}
	m.detailScroll = 0
	m.status = ""
	return true
}

// openCurrentBranch opens the current Git branch's tasks, if it has any.
func (m *model) openCurrentBranch() {
	if m.project.Branch == "" || m.branch.open != (group{}) {
		return
	}
	i := slices.IndexFunc(m.rows(branchPane), branchGroup(m.project.Branch).openedBy)
	if i < 0 {
		return
	}
	m.branch.cursor = i
	m.branch.enter(branchGroup(m.project.Branch))
	m.detailScroll = 0
	m.status = ""
}
