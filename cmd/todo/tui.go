package main

import (
	"cmp"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/editor"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/level"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

type pane int

const (
	generalPane pane = iota
	branchPane
	sourcePane
	detailPane
)

type model struct {
	file             string
	project          projectContext
	tasks            taskSet
	all              *allTasksView // nil unless the All tasks view is open
	general          groupList
	branch           groupList
	focus            pane
	detailFrom       pane
	detailScroll     int
	localBranchNames map[string]bool
	branchesVerified bool
	files            filesui.Model
	overlay          overlay // nil when nothing is open over the dashboard
	lastClear        *store.Removal
	status           string
	width            int
	height           int
}

func newModel(file string, project projectContext, files filesui.Model) (*model, error) {
	m := &model{file: file, project: project, width: 100, height: 30, files: files}
	if err := m.reload(); err != nil {
		return nil, err
	}
	m.openCurrentBranch()
	return m, nil
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.files.Scan(), tea.RequestBackgroundColor) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.overlay != nil {
			return m, m.updateOverlay(msg)
		}
	case tea.BackgroundColorMsg:
		ui.ApplyTheme(msg.IsDark())
	case filesui.ScannedMsg:
		cmd := m.files.Update(msg)
		m.status = ""
		if err := m.files.Err(); err != "" {
			m.status = "File scan failed: " + err
		}
		return m, cmd
	case editor.ClosedMsg:
		if msg.Err != nil {
			m.status = msg.Err.Error()
		} else {
			m.status = ""
		}
		if err := m.refresh(); err != nil {
			m.status = err.Error()
		}
		return m, m.files.Scan()
	case taskSavedMsg:
		return m.taskSaved(msg)
	case categorySetMsg:
		return m.categorySet(msg)
	case clearConfirmedMsg:
		return m.applyClear(msg)
	case tea.PasteMsg:
		if m.overlay != nil {
			return m, m.updateOverlay(msg)
		}
	case tea.KeyPressMsg:
		return m, m.updateKey(msg)
	default:
		return m, m.files.Update(msg)
	}
	return m, nil
}

// updateKey sends a key to the open overlay, or else to the current view,
// and then to the task keys both views share.
func (m *model) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case msg.String() == "ctrl+c":
		return tea.Quit
	case m.overlay != nil:
		return m.updateOverlay(msg)
	}
	var cmd tea.Cmd
	var handled bool
	if m.all != nil {
		cmd, handled = m.allTasksKey(msg.String())
	} else {
		cmd, handled = m.dashboardKey(msg)
	}
	if handled {
		return cmd
	}
	return m.taskKey(msg.String())
}

// taskKey handles the keys that work the same in the dashboard and the All
// tasks view.
func (m *model) taskKey(key string) tea.Cmd {
	switch key {
	case "q":
		return tea.Quit
	case "?":
		m.overlay = helpOverlay{}
	case "a":
		if m.all == nil && m.activePane() == branchPane {
			return m.startTaskModal(modalAddBranch)
		}
		return m.startTaskModal(modalAddGeneral)
	case "b":
		return m.startTaskModal(modalAddBranch)
	case "c":
		return m.startCategoryInput()
	case "e":
		return m.editSelected()
	case "p":
		m.cyclePriority()
	case "space", "d":
		m.toggleSelected()
	case "X":
		m.startClearDone()
	case "u":
		m.undoClear()
	case "r":
		m.status = ""
		if err := m.refreshProject(); err != nil {
			m.status = err.Error()
		}
		return m.files.Scan()
	}
	return nil
}

// dashboardKey handles the keys for moving around the dashboard's tabs,
// reporting false for any other.
func (m *model) dashboardKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()
	if m.focus == sourcePane {
		switch key {
		case "right", "left", "esc", "up", "k", "down", "j", "e", "enter":
			return m.files.Update(msg), true
		}
	}
	switch key {
	case "i":
		m.openAllTasks()
	case "tab":
		if m.focus == detailPane {
			m.focus = (m.detailFrom + 1) % 3
		} else {
			m.focus = (m.focus + 1) % 3
		}
		m.detailScroll = 0
		m.files.CloseDetails()
	case "shift+tab":
		if m.focus == detailPane {
			m.focus = m.detailFrom
		} else {
			m.focus = (m.focus + 2) % 3
		}
		m.detailScroll = 0
		m.files.CloseDetails()
	case "1":
		m.jumpToTab(generalPane)
	case "2":
		m.jumpToTab(branchPane)
	case "3":
		if m.focus == sourcePane {
			m.files.Top()
		} else {
			m.focus = sourcePane
			m.detailScroll = 0
		}
		return m.files.PreviewCmd(), true
	case "right":
		if m.focus != detailPane && !m.enterSelectedGroup() {
			if _, ok := m.selectedTask(); ok {
				m.detailFrom = m.focus
				m.focus = detailPane
				m.detailScroll = 0
			}
		}
	case "left", "esc":
		if m.focus == detailPane {
			m.focus = m.detailFrom
			m.detailScroll = 0
		} else {
			m.leaveGroup()
		}
	case "up", "k":
		if m.focus == detailPane {
			m.detailScroll = max(0, m.detailScroll-1)
		} else {
			m.moveCursor(-1)
		}
	case "down", "j":
		if m.focus == detailPane {
			m.detailScroll++
		} else {
			m.moveCursor(1)
		}
	case "enter":
		if !m.enterSelectedGroup() {
			return m.editSelected(), true
		}
	default:
		return nil, false
	}
	return nil, true
}

// editSelected opens the selected task in the form, or a README task in the
// editor.
func (m *model) editSelected() tea.Cmd {
	if m.readmeSelected() {
		return m.openReadme()
	}
	return m.startTaskModal(modalEdit)
}

func (m *model) moveCursor(delta int) {
	if m.all != nil {
		m.all.cursor = max(0, min(m.all.cursor+delta, len(m.tasks.all)-1))
	} else if l := m.list(m.focus); l != nil {
		l.cursor = max(0, min(l.cursor+delta, len(m.rows(m.focus))-1))
	}
}

func (m *model) selectedTask() (store.Task, bool) {
	if m.all != nil {
		tasks := m.all.sorted(m.tasks.all)
		if m.all.cursor >= 0 && m.all.cursor < len(tasks) {
			return tasks[m.all.cursor], true
		}
		return store.Task{}, false
	}
	row, ok := m.selectedNavigationRow()
	if ok && row.kind == rowTask {
		return row.todo, true
	}
	return store.Task{}, false
}

func (m *model) activePane() pane {
	if m.focus == detailPane {
		return m.detailFrom
	}
	return m.focus
}

// readmeSelected is true while the README.md group is open, where every
// row is a README task.
func (m *model) readmeSelected() bool {
	return m.general.open.kind == rowReadme && m.all == nil && m.activePane() == generalPane
}

// readmeReadOnly is the status for anything but done and reopen on a README
// task; the README is edited in an editor.
const readmeReadOnly = "README.md tasks can only be marked done or reopened; e opens the file"

func (m *model) readmeFile() string {
	return filepath.Join(filepath.Dir(m.file), "README.md")
}

func (m *model) openReadme() tea.Cmd {
	selected, ok := m.selectedTask()
	if !ok {
		return nil
	}
	return editor.Open(m.readmeFile(), selected.Line+1)
}

func (m *model) toggleSelected() {
	selected, ok := m.selectedTask()
	if !ok {
		if m.activePane() == sourcePane {
			m.status = "File TODOs are read only"
		} else {
			m.status = "Open a category or branch to select a task"
		}
		return
	}
	if m.blockMissingBranch(selected) {
		return
	}
	toggle := func() error { return store.Toggle(m.file, selected) }
	if m.readmeSelected() {
		toggle = func() error { return store.ToggleReadme(m.readmeFile(), selected) }
	}
	if err := toggle(); err != nil {
		m.status = errorStatus(err)
		return
	}
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return
	}
	if m.all == nil {
		m.selectNavigationTask(selected)
	}
	m.status = ""
}

func errorStatus(err error) string {
	if errors.Is(err, store.ErrTaskChanged) {
		return "Task changed on disk; press r to reload"
	}
	return err.Error()
}

func (m *model) blockMissingBranch(t store.Task) bool {
	if !m.branchMissing(t.Branch) {
		return false
	}
	m.status = missingBranchStatus
	return true
}

func (m *model) cyclePriority() {
	if m.readmeSelected() {
		m.status = readmeReadOnly
		return
	}
	selected, ok := m.selectedTask()
	if !ok {
		m.status = "Select a Markdown task to set priority"
		return
	}
	if m.blockMissingBranch(selected) {
		return
	}
	if err := store.SetPriority(m.file, selected, selected.Priority.Next()); err != nil {
		m.status = errorStatus(err)
		return
	}
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = ""
}

// refreshProject re-reads the Git context and the task file. When the
// Branches tab was showing the current branch and Git has since switched,
// it follows to the new current branch, as at startup; a branch opened by
// hand stays open.
func (m *model) refreshProject() error {
	previous := m.project.branch
	m.project = currentProject()
	err := m.reload()
	if m.project.branch != previous && previous != "" && m.branch.open == branchGroup(previous) {
		m.branch.leave()
		if m.focus == detailPane && m.detailFrom == branchPane {
			m.focus = branchPane
		}
		m.openCurrentBranch()
	}
	return err
}

// reload is for startup and explicit refreshes. Otherwise Git is only asked
// for branches when a branch task modal opens, keeping quick edits free of
// subprocess calls.
func (m *model) reload() error {
	m.checkLocalBranches()
	return m.readTasks(true)
}

func (m *model) checkLocalBranches() (branches []string, current string) {
	branches, current, verified := m.project.localBranchState()
	m.branchesVerified = verified
	m.localBranchNames = make(map[string]bool, len(branches))
	for _, branch := range branches {
		m.localBranchNames[branch] = true
	}
	return branches, current
}

// recheckBranch updates one branch's cached state, so a branch deleted since
// the last refresh is caught before its tasks are shown.
func (m *model) recheckBranch(name string) {
	if !m.branchesVerified {
		return
	}
	if exists, verified := m.project.branchExists(name); verified {
		m.localBranchNames[name] = exists
	}
}

func (m *model) refresh() error {
	return m.readTasks(false)
}

// readTasks also drops any pending undo of a clear: whatever caused the
// reload may have changed the file since.
func (m *model) readTasks(sortByPriority bool) error {
	m.lastClear = nil
	previous, hadSelection := m.selectedTask()
	tasks, err := store.Load(m.file)
	if err != nil {
		return err
	}
	if sortByPriority {
		m.tasks.setAll(sortedTasksByPriority(tasks))
		if m.all != nil {
			m.all.byPriority = false
		}
	} else {
		m.tasks.setAll(preserveTaskOrder(m.tasks.all, tasks))
	}
	if m.tasks.readme, err = loadReadme(m.readmeFile()); err != nil {
		return err
	}
	// A group whose tasks have all gone closes.
	for _, p := range []pane{generalPane, branchPane} {
		l := m.list(p)
		if len(m.rows(p)) == 0 {
			l.leave()
		}
		l.cursor = min(l.cursor, max(0, len(m.rows(p))-1))
	}
	if m.all != nil {
		m.all.cursor = min(m.all.cursor, max(0, len(m.tasks.all)-1))
		if hadSelection {
			m.selectInAllTasks(previous)
		}
	}
	return nil
}

func preserveTaskOrder(previous, loaded []store.Task) []store.Task {
	if len(previous) == 0 || len(loaded) == 0 {
		return loaded
	}
	type key struct{ text, branch, category string }
	identity := func(item store.Task) key {
		return key{item.Text, item.Branch, strings.ToLower(item.Category)}
	}
	positions := make(map[key][]int, len(loaded))
	for i, item := range loaded {
		positions[identity(item)] = append(positions[identity(item)], i)
	}
	assigned := make([]int, len(previous))
	for i := range assigned {
		assigned[i] = -1
	}
	used := make([]bool, len(loaded))
	for oldIndex, old := range previous {
		candidates := positions[identity(old)]
		best, bestOffset, distance := -1, -1, math.MaxInt
		for offset, i := range candidates {
			if d := abs(loaded[i].Line - old.Line); d < distance {
				best, bestOffset, distance = i, offset, d
			}
		}
		if best >= 0 {
			assigned[oldIndex] = best
			used[best] = true
			positions[identity(old)] = append(candidates[:bestOffset], candidates[bestOffset+1:]...)
		}
	}
	for oldIndex, old := range previous {
		if assigned[oldIndex] >= 0 {
			continue
		}
		best, distance := -1, math.MaxInt
		for i, candidate := range loaded {
			if used[i] || candidate.Branch != old.Branch || !strings.EqualFold(candidate.Category, old.Category) {
				continue
			}
			if d := abs(candidate.Line - old.Line); d < distance {
				best, distance = i, d
			}
		}
		if best >= 0 {
			assigned[oldIndex] = best
			used[best] = true
		}
	}
	ordered := make([]store.Task, 0, len(loaded))
	for _, index := range assigned {
		if index >= 0 {
			ordered = append(ordered, loaded[index])
		}
	}
	for i, item := range loaded {
		if !used[i] {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

// loadReadme reads the README's tasks with todo-system levels first, most
// urgent at the top, as the Files tab lists them.
func loadReadme(path string) ([]store.Task, error) {
	tasks, err := store.LoadReadme(path)
	slices.SortStableFunc(tasks, func(a, b store.Task) int {
		return cmp.Compare(level.Rank(a.Level), level.Rank(b.Level))
	})
	return tasks, err
}

// taskSet is the tasks the dashboard shows, as last read.
type taskSet struct {
	all      []store.Task // todo.md's tasks, in the dashboard's order
	general  []store.Task // all's tasks without a branch
	branches []store.Task // all's tasks with one
	readme   []store.Task // README.md's tasks, most urgent first
}

func (s *taskSet) setAll(tasks []store.Task) {
	s.all, s.general, s.branches = tasks, nil, nil
	for _, t := range tasks {
		if t.Branch == "" {
			s.general = append(s.general, t)
		} else {
			s.branches = append(s.branches, t)
		}
	}
}
