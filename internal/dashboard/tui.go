// Package dashboard is the terminal dashboard of todo. Its tabs show the
// general tasks, the tasks of each Git branch and the TODO comments in source
// files. Tasks can be added, edited, marked done, deleted and cleared from it.
package dashboard

import (
	"cmp"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/editor"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/level"
	"github.com/nebarg/todo-cli/internal/project"
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

// tabCount is how many panes are tabs: those before detailPane, which opens
// over whichever of them has focus. tab and shift+tab cycle through them.
const tabCount = detailPane

type model struct {
	file             string
	project          project.Repo
	tasks            taskSet
	all              *allTasksView // nil unless the All tasks view is open
	general          groupList
	branch           groupList
	theme            ui.Theme
	focus            pane
	detailFrom       pane
	detailScroll     int
	localBranches    []string // as Git last listed them, sorted
	branchesVerified bool     // whether Git answered, so a branch not in localBranches is gone
	files            filesui.Model
	overlay          overlay      // nil when nothing is open over the dashboard
	lastRemoval      *removalUndo // the last clear or delete, for u to undo
	contents         [][]byte     // the watched files as the tasks were last read from them
	status           string
	width            int
	height           int
}

// New makes the dashboard for the task file in repo, its Files tab showing
// files. It fails if the task file or README.md can't be read.
func New(file string, repo project.Repo, files filesui.Model) (tea.Model, error) {
	m, err := newModel(file, repo, files)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func newModel(file string, repo project.Repo, files filesui.Model) (*model, error) {
	m := &model{file: file, project: repo, theme: ui.NewTheme(true), width: 100, height: 30, files: files}
	// Before the dashboard is drawn, Git is asked directly.
	m.setBranches(branchState(repo))
	if err := m.readTasks(true, nil); err != nil {
		return nil, err
	}
	m.openCurrentBranch()
	return m, nil
}

// Init starts watching without stats, so the first check compares the files
// with what newModel read, catching an edit made since.
func (m *model) Init() tea.Cmd {
	return tea.Batch(m.files.Scan(), tea.RequestBackgroundColor, m.watch(nil))
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Laying the task form or the category prompt out needs the theme,
		// which only the model holds, so they are resized here rather than
		// through their updates.
		switch o := m.overlay.(type) {
		case *taskModal:
			o.resize(m.theme, msg.Width, msg.Height)
		case *categoryPrompt:
			o.resize(m.theme, msg.Width)
		}
	case tea.BackgroundColorMsg:
		m.setTheme(ui.NewTheme(msg.IsDark()))
	case filesui.ScannedMsg:
		cmd := m.files.Update(msg)
		m.status = ""
		if err := m.files.Err(); err != nil {
			m.status = "File scan failed: " + err.Error()
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
	case branchStateMsg:
		m.setBranches(msg)
	case projectStateMsg:
		m.setProject(msg)
	case filesChangedMsg:
		return m, m.filesChanged(msg)
	case taskSavedMsg:
		return m.taskSaved(msg)
	case categorySetMsg:
		return m.categorySet(msg)
	case removalConfirmedMsg:
		return m.applyRemoval(msg)
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
		cmd, handled = m.allTasksKey(msg)
	} else {
		cmd, handled = m.dashboardKey(msg)
	}
	if handled {
		return cmd
	}
	return m.taskKey(msg)
}

// taskKey handles the keys that work the same in the dashboard and the All
// tasks view.
func (m *model) taskKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Quit):
		return tea.Quit
	case key.Matches(msg, keys.Help):
		m.overlay = helpOverlay{}
	case key.Matches(msg, keys.Add):
		switch {
		case m.readmeSelected():
			m.status = readmeReadOnly
			return nil
		case m.focus == detailPane:
			return m.startTaskModal(modalAddSubtask)
		case m.all == nil && m.activePane() == branchPane:
			return m.startTaskModal(modalAddBranch)
		}
		return m.startTaskModal(modalAddGeneral)
	case key.Matches(msg, keys.AddBranch):
		return m.startTaskModal(modalAddBranch)
	case key.Matches(msg, keys.Category):
		return m.startCategoryInput()
	case key.Matches(msg, keys.Edit):
		return m.editSelected()
	case key.Matches(msg, keys.Priority):
		m.cyclePriority()
	case key.Matches(msg, keys.Done):
		m.toggleSelected()
	case key.Matches(msg, keys.Delete):
		m.startDelete()
	case key.Matches(msg, keys.Clear):
		m.startClearDone()
	case key.Matches(msg, keys.Undo):
		m.undoRemoval()
	case key.Matches(msg, keys.Reload):
		m.status = ""
		if err := m.readTasks(true, nil); err != nil {
			m.status = err.Error()
		}
		return tea.Batch(reloadProject(), m.files.Scan())
	}
	return nil
}

// dashboardKey handles the keys for moving around the dashboard's tabs,
// reporting false for any other.
func (m *model) dashboardKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.focus == sourcePane && filesui.Handles(msg) {
		return m.files.Update(msg), true
	}
	switch {
	case key.Matches(msg, keys.AllTasks):
		m.openAllTasks()
	case key.Matches(msg, keys.NextTab):
		if m.focus == detailPane {
			m.focus = (m.detailFrom + 1) % tabCount
		} else {
			m.focus = (m.focus + 1) % tabCount
		}
		m.detailScroll = 0
		m.files.CloseDetails()
	case key.Matches(msg, keys.PrevTab):
		if m.focus == detailPane {
			m.focus = m.detailFrom
		} else {
			m.focus = (m.focus + tabCount - 1) % tabCount
		}
		m.detailScroll = 0
		m.files.CloseDetails()
	case key.Matches(msg, keys.Tab1):
		m.jumpToTab(generalPane)
	case key.Matches(msg, keys.Tab2):
		m.jumpToTab(branchPane)
	case key.Matches(msg, keys.Tab3):
		if m.focus == sourcePane {
			m.files.Top()
		} else {
			m.focus = sourcePane
			m.detailScroll = 0
		}
		return m.files.PreviewCmd(), true
	case key.Matches(msg, keys.Open):
		if m.enterSelectedGroup() {
			return nil, true
		}
		if row, ok := m.selectedNavigationRow(); ok && row.isTask() && m.focus != detailPane {
			m.detailFrom = m.focus
			m.focus = detailPane
			m.detailScroll = 0
		}
	case key.Matches(msg, keys.Back):
		if m.focus == detailPane {
			m.focus = m.detailFrom
			m.detailScroll = 0
		} else {
			m.leaveGroup()
		}
	case key.Matches(msg, keys.Up):
		if m.focus == detailPane {
			m.detailScroll = max(0, m.detailScroll-1)
		} else {
			m.moveCursor(-1)
		}
	case key.Matches(msg, keys.Down):
		if m.focus == detailPane {
			m.detailScroll++
		} else {
			m.moveCursor(1)
		}
	case key.Matches(msg, keys.Enter):
		if m.enterSelectedGroup() {
			return nil, true
		}
		return m.editSelected(), true
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

// moveCursor moves the selection a row up or down, wrapping past either end.
func (m *model) moveCursor(delta int) {
	if m.all != nil {
		m.all.cursor = ui.Wrap(m.all.cursor, delta, len(m.tasks.all))
	} else if l := m.list(m.focus); l != nil {
		l.cursor = ui.Wrap(l.cursor, delta, len(m.rows(m.focus)))
	}
}

// selectedTask is the selected task of todo.md; a README task is not one.
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

// selectedReadmeTask is the selected task while the README.md group or one
// of its headings is open.
func (m *model) selectedReadmeTask() (store.ReadmeTask, bool) {
	if m.all != nil {
		return store.ReadmeTask{}, false
	}
	row, ok := m.selectedNavigationRow()
	if ok && row.kind == rowReadmeTask {
		return row.readme, true
	}
	return store.ReadmeTask{}, false
}

func (m *model) activePane() pane {
	if m.focus == detailPane {
		return m.detailFrom
	}
	return m.focus
}

// readmeSelected is true while the README.md group or one of its headings
// is open, where every row is a README task or heading.
func (m *model) readmeSelected() bool {
	return m.general.open.inReadme() && m.all == nil && m.activePane() == generalPane
}

// subtaskStaysPut is the status for c on a subtask, which goes wherever its
// task does.
const subtaskStaysPut = "A subtask keeps its task's category; c on the task moves both"

// readmeReadOnly is the status for anything but done and reopen on a README
// task; the README is edited in an editor.
const readmeReadOnly = "README.md tasks can only be marked done or reopened; e opens the file"

func (m *model) readmeFile() string {
	return filepath.Join(filepath.Dir(m.file), "README.md")
}

func (m *model) openReadme() tea.Cmd {
	selected, ok := m.selectedReadmeTask()
	if !ok {
		return nil
	}
	return editor.Open(m.readmeFile(), selected.Line+1)
}

func (m *model) toggleSelected() {
	var toggle func() error
	var reselect func()
	var changed []store.Task
	if readme, ok := m.selectedReadmeTask(); ok {
		toggle = func() error { return store.ToggleReadme(m.readmeFile(), readme) }
		reselect = func() { m.selectReadmeTask(readme) }
	} else {
		selected, ok := m.selectedTask()
		if !ok {
			if m.activePane() == sourcePane {
				m.status = "File TODOs are read only"
			} else {
				m.status = "Open a category or branch to select a task"
			}
			return
		}
		toggle = func() error { return store.Toggle(m.file, selected) }
		toggled := selected
		toggled.Done = !selected.Done
		changed = append(changed, toggled)
	}
	if err := toggle(); err != nil {
		m.status = errorStatus(err)
		return
	}
	if err := m.refresh(changed...); err != nil {
		m.status = err.Error()
		return
	}
	if reselect != nil {
		reselect()
	}
	m.status = ""
}

func errorStatus(err error) string {
	if errors.Is(err, store.ErrTaskChanged) {
		return "Task changed on disk; press r to reload"
	}
	return err.Error()
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
	changed := selected
	changed.Priority = selected.Priority.Next()
	if err := store.SetPriority(m.file, selected, changed.Priority); err != nil {
		m.status = errorStatus(err)
		return
	}
	if err := m.refresh(changed); err != nil {
		m.status = err.Error()
		return
	}
	m.status = ""
}

// branchStateMsg is the local branches and the current branch as Git
// listed them. verified is false when there was no Git repository to ask.
type branchStateMsg struct {
	branches []string
	current  string
	verified bool
}

func branchState(repo project.Repo) branchStateMsg {
	branches, current, verified := repo.LocalBranchState()
	return branchStateMsg{branches: branches, current: current, verified: verified}
}

// checkBranches asks Git for the local branches in the background, so the
// dashboard doesn't wait on it.
func (m *model) checkBranches() tea.Cmd {
	repo := m.project
	return func() tea.Msg { return branchState(repo) }
}

// setBranches takes the local branches Git listed, passing them on to an
// open branch task form.
func (m *model) setBranches(msg branchStateMsg) {
	m.localBranches, m.branchesVerified = msg.branches, msg.verified
	f, ok := m.overlay.(*taskModal)
	if !ok || !f.branchScope() {
		return
	}
	f.setBranches(msg.branches, msg.current)
}

// projectStateMsg is the repository, current branch and local branches as
// Git reported them when r reloaded.
type projectStateMsg struct {
	project  project.Repo
	branches branchStateMsg
}

// reloadProject asks Git in the background for the repository, its current
// branch and its local branches.
func reloadProject() tea.Cmd {
	return func() tea.Msg {
		repo := project.Current()
		return projectStateMsg{project: repo, branches: branchState(repo)}
	}
}

// setProject takes the Git state r asked for. When the Branches tab was
// showing the current branch and Git has since switched, it follows to the
// new current branch, as at startup; a branch opened by hand stays open.
func (m *model) setProject(msg projectStateMsg) {
	previous := m.project.Branch
	m.project = msg.project
	m.setBranches(msg.branches)
	if m.project.Branch != previous && previous != "" && m.branch.open == branchGroup(previous) {
		m.branch.leave()
		if m.focus == detailPane && m.detailFrom == branchPane {
			m.focus = branchPane
		}
		m.openCurrentBranch()
	}
}

// setTheme takes the theme for the terminal's background, passing it on to
// an open task form, whose fields keep the styles they were given.
func (m *model) setTheme(theme ui.Theme) {
	m.theme = theme
	if f, ok := m.overlay.(*taskModal); ok {
		f.setTheme(theme)
	}
}

// refresh rereads the file after a write, keeping each task in the place it
// was shown in. changed are the tasks the write changed, as they now are but
// with the lines they were read at. They are placed after the other tasks,
// so an identical task can't take their places, and the cursor follows them.
func (m *model) refresh(changed ...store.Task) error {
	return m.readTasks(false, changed)
}

// refreshFrom rereads the file as refresh does, keeping the rows in the
// order of previous rather than of the tasks shown.
func (m *model) refreshFrom(previous []store.Task) error {
	m.tasks.setAll(previous)
	return m.refresh()
}

// readTasks also drops any pending undo of a clear or delete: whatever
// caused the reload may have changed the file since. With sort, the tasks
// are sorted afresh, as at startup and on r; otherwise each keeps its place.
// changed is as refresh takes it. Each file is read once and the tasks are
// parsed from those bytes, so m.contents is exactly what they come from.
func (m *model) readTasks(sort bool, changed []store.Task) error {
	m.lastRemoval = nil
	previous, hadSelection := m.selectedTask()
	i := slices.IndexFunc(changed, func(t store.Task) bool { return t.Line == previous.Line })
	selectionChanged := hadSelection && i >= 0
	if selectionChanged {
		previous = changed[i]
	}
	contents, err := readFiles(m.watchedFiles())
	if err != nil {
		return err
	}
	m.contents = contents
	// contents are in the order watchedFiles lists them.
	tasks, readme := store.Parse(contents[0]), store.ParseReadme(contents[1])
	if sort {
		m.tasks.setAll(sortedTasks(tasks))
		m.tasks.readme = sortedReadme(readme)
	} else {
		m.tasks.setAll(preserveTaskOrder(m.tasks.all, tasks, changed))
		m.tasks.readme = keepReadmeOrder(m.tasks.readme, readme)
	}
	// A group whose tasks have all gone closes, as does a README.md group
	// left empty by its heading closing.
	for _, p := range []pane{generalPane, branchPane} {
		l := m.list(p)
		for len(m.rows(p)) == 0 && l.leave() {
		}
		l.cursor = min(l.cursor, max(0, len(m.rows(p))-1))
	}
	if m.all != nil {
		m.all.cursor = min(m.all.cursor, max(0, len(m.tasks.all)-1))
		if hadSelection {
			m.selectInAllTasks(previous)
		}
	} else if selectionChanged {
		m.selectNavigationTask(previous)
	}
	return nil
}

// preserveTaskOrder orders loaded as previous was shown. Each task takes the
// place of the previous task most like it: first one alike in text, section,
// done state and priority, then any in its section, nearest its line either
// way. Tasks without a place go in as placeNew puts them. The changed tasks,
// at their previous lines, only get places in the second round, so an
// identical task left as it was can't take theirs.
func preserveTaskOrder(previous, loaded, changed []store.Task) []store.Task {
	if len(previous) == 0 || len(loaded) == 0 {
		return loaded
	}
	type key struct {
		text, branch, category string
		done, subtask          bool
		priority               store.Priority
	}
	identity := func(item store.Task) key {
		return key{item.Text, item.Branch, strings.ToLower(item.Category), item.Done, item.Subtask, item.Priority}
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
		if slices.ContainsFunc(changed, func(t store.Task) bool { return t.Line == old.Line }) {
			continue
		}
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
			if used[i] || !candidate.Same(old.Section) || candidate.Subtask != old.Subtask {
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
			ordered = placeNew(ordered, item)
		}
	}
	return ordered
}

// placeNew adds t, a task new to the list or moved into its section, to
// tasks. An open task goes after the open tasks of its section, so it isn't
// shown among the done ones; a done task goes last.
func placeNew(tasks []store.Task, t store.Task) []store.Task {
	afterOpen, firstDone := -1, -1
	for i, other := range tasks {
		switch {
		case t.Done || !other.Same(t.Section):
		case !other.Done:
			afterOpen = i + 1
		case firstDone < 0:
			firstDone = i
		}
	}
	switch {
	case afterOpen >= 0:
		return slices.Insert(tasks, afterOpen, t)
	case firstDone >= 0:
		return slices.Insert(tasks, firstDone, t)
	}
	return append(tasks, t)
}

// sortedReadme is README tasks as the dashboard shows them after loading:
// open tasks, then done ones, each with todo-system levels first, most
// urgent at the top, as the Files tab lists them. Subtasks show under their
// parents, so a parent's place decides where its subtasks go.
func sortedReadme(tasks []store.ReadmeTask) []store.ReadmeTask {
	sorted := slices.Clone(tasks)
	slices.SortStableFunc(sorted, func(a, b store.ReadmeTask) int {
		return cmp.Or(compareDone(a.Done, b.Done), cmp.Compare(level.Rank(a.Level), level.Rank(b.Level)))
	})
	return sorted
}

// keepReadmeOrder orders loaded as previous was shown when it's the same
// tasks on the same lines, as after a toggle. A README changed in other ways,
// such as in an editor, is sorted afresh.
func keepReadmeOrder(previous, loaded []store.ReadmeTask) []store.ReadmeTask {
	if len(previous) != len(loaded) {
		return sortedReadme(loaded)
	}
	byLine := make(map[int]store.ReadmeTask, len(loaded))
	for _, t := range loaded {
		byLine[t.Line] = t
	}
	ordered := make([]store.ReadmeTask, 0, len(loaded))
	for _, old := range previous {
		t, ok := byLine[old.Line]
		if !ok || t.Text != old.Text || t.Heading != old.Heading || t.Subtask != old.Subtask {
			return sortedReadme(loaded)
		}
		ordered = append(ordered, t)
	}
	return ordered
}

// taskSet is the tasks the dashboard shows, as last read.
type taskSet struct {
	all      []store.Task       // todo.md's tasks, in the dashboard's order
	general  []store.Task       // all's tasks without a branch
	branches []store.Task       // all's tasks with one
	readme   []store.ReadmeTask // README.md's tasks, in the dashboard's order
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
