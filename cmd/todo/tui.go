package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type pane int

const (
	generalPane pane = iota
	branchPane
	sourcePane
	detailPane
)

type sourceScanMsg struct {
	matches []sourceTodo
	err     error
}
type sourcePreviewMsg struct {
	path  string
	line  int
	lines []previewLine
	err   error
}
type editorFinishedMsg struct{ err error }

type sortOrder string

const (
	sortPriority sortOrder = "priority"
	sortBranch   sortOrder = "branch"
	sortCategory sortOrder = "category"
)

func (s sortOrder) next() sortOrder {
	switch s {
	case sortPriority:
		return sortBranch
	case sortBranch:
		return sortCategory
	default:
		return sortPriority
	}
}

type model struct {
	file                  string
	project               projectContext
	allTasks              []task
	indexMode             bool
	indexSort             sortOrder
	indexPriorityExplicit bool
	indexCursor           int
	general               []task
	branches              []task
	source                []sourceTodo
	focus                 pane
	detailFrom            pane
	detailScroll          int
	generalCursor         int
	generalRootCursor     int
	generalLabel          string
	branchCursor          int
	branchRootCursor      int
	branchFilter          string
	localBranchNames      map[string]bool
	branchesVerified      bool
	sourceCursor          int
	sourceLoading         bool
	sourceScanned         bool
	sourceError           string
	preview               []previewLine
	previewPath           string
	previewLine           int
	previewError          string
	input                 textinput.Model
	categoryInput         bool
	editTask              task
	modal                 *taskModal
	status                string
	width                 int
	height                int
}

func newModel(file string, project projectContext) (*model, error) {
	input := textinput.New()
	input.Prompt = "New task: "
	input.Placeholder = "What needs doing?"
	input.SetWidth(72)
	m := &model{file: file, project: project, input: input, width: 100, height: 30, sourceLoading: true, indexSort: sortPriority}
	if err := m.reload(); err != nil {
		return nil, err
	}
	m.preselectCurrentBranch()
	return m, nil
}

func (m *model) Init() tea.Cmd { return m.scanCmd() }

func (m *model) scanDir() string {
	if m.project.root != "" {
		return m.project.root
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func (m *model) scanCmd() tea.Cmd {
	dir := m.scanDir()
	return func() tea.Msg {
		matches, err := scanSource(dir, 1000, false)
		return sourceScanMsg{matches: matches, err: err}
	}
}

func (m *model) previewCmd() tea.Cmd {
	if m.sourceCursor >= len(m.source) {
		return nil
	}
	selected := m.source[m.sourceCursor]
	path := filepath.Join(m.scanDir(), selected.path)
	return func() tea.Msg {
		lines, err := readSourceContext(path, selected.line)
		return sourcePreviewMsg{path: selected.path, line: selected.line, lines: lines, err: err}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.modal != nil {
			m.modal.resize(msg.Width, msg.Height)
		}
		inputWidth := msg.Width - 4
		if inputWidth < 20 {
			inputWidth = 20
		}
		m.input.SetWidth(inputWidth)
	case sourceScanMsg:
		m.sourceLoading = false
		m.sourceScanned = true
		if msg.err != nil {
			m.sourceError = msg.err.Error()
			m.status = "File scan failed: " + m.sourceError
		} else {
			m.sourceError = ""
			m.source = msg.matches
			if m.sourceCursor >= len(m.source) {
				m.sourceCursor = 0
			}
			m.status = ""
			return m, m.previewCmd()
		}
	case sourcePreviewMsg:
		if m.sourceCursor < len(m.source) {
			selected := m.source[m.sourceCursor]
			if selected.path == msg.path && selected.line == msg.line {
				m.previewPath, m.previewLine = msg.path, msg.line
				m.preview = msg.lines
				m.previewError = ""
				if msg.err != nil {
					m.previewError = msg.err.Error()
				}
			}
		}
	case editorFinishedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = ""
		}
		m.sourceLoading = true
		return m, m.scanCmd()
	case tea.PasteMsg:
		if m.modal != nil {
			return m.updateTaskModalPaste(msg)
		}
		if m.categoryInput {
			msg.Content = stripLabelSpaces(msg.Content)
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.modal != nil {
			return m.updateTaskModal(msg)
		}
		if m.categoryInput {
			return m.updateCategoryInput(msg)
		}
		if m.indexMode {
			return m.updateIndex(msg)
		}
		switch key {
		case "q":
			return m, tea.Quit
		case "i":
			m.indexMode = true
			m.indexSort = sortPriority
			m.indexPriorityExplicit = false
			m.indexCursor = 0
			m.status = ""
		case "tab":
			if m.focus == detailPane {
				m.focus = (m.detailFrom + 1) % 3
			} else {
				m.focus = (m.focus + 1) % 3
			}
			m.detailScroll = 0
		case "shift+tab":
			if m.focus == detailPane {
				m.focus = m.detailFrom
			} else {
				m.focus = (m.focus + 2) % 3
			}
			m.detailScroll = 0
		case "1":
			m.focus = generalPane
			m.detailScroll = 0
		case "2":
			m.focus = branchPane
			m.detailScroll = 0
		case "3":
			m.focus = sourcePane
			m.detailScroll = 0
		case "right":
			if m.focus != detailPane {
				if !m.enterSelectedGroup() {
					_, taskSelected := m.selectedTask()
					if taskSelected || (m.focus == sourcePane && m.sourceCursor < len(m.source)) {
						m.detailFrom = m.focus
						m.focus = detailPane
						m.detailScroll = 0
					}
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
			} else if m.moveCursor(-1) && m.focus == sourcePane {
				return m, m.previewCmd()
			}
		case "down", "j":
			if m.focus == detailPane {
				m.detailScroll++
			} else if m.moveCursor(1) && m.focus == sourcePane {
				return m, m.previewCmd()
			}
		case "a":
			if m.activePane() == branchPane {
				return m.startTaskModal(modalAddBranch)
			}
			return m.startTaskModal(modalAddGeneral)
		case "b":
			return m.startTaskModal(modalAddBranch)
		case "c", "l":
			return m.startCategoryInput()
		case "e":
			if m.activePane() == sourcePane {
				return m, m.openSource()
			}
			return m.startTaskModal(modalEdit)
		case "p":
			m.cyclePriority()
		case "space", "d":
			m.toggleSelected()
		case "enter":
			if m.activePane() == sourcePane {
				return m, m.openSource()
			}
			if !m.enterSelectedGroup() {
				return m.startTaskModal(modalEdit)
			}
		case "v":
			m.focus = branchPane
			m.branchFilter = ""
			m.branchCursor = m.branchRootCursor
			m.status = ""
		case "r":
			m.project = currentProject()
			if err := m.reload(); err != nil {
				m.status = err.Error()
			} else {
				m.status = ""
			}
			m.sourceLoading = true
			return m, m.scanCmd()
		}
	}
	return m, nil
}

func (m *model) moveCursor(delta int) bool {
	if m.indexMode {
		return m.moveIndexCursor(delta)
	}
	cursor, length := &m.generalCursor, len(m.generalRows())
	switch m.focus {
	case branchPane:
		cursor, length = &m.branchCursor, len(m.branchRows())
	case sourcePane:
		cursor, length = &m.sourceCursor, len(m.source)
	}
	next := *cursor + delta
	if next < 0 {
		next = 0
	}
	if next >= length {
		next = length - 1
	}
	if next < 0 {
		next = 0
	}
	if next == *cursor {
		return false
	}
	*cursor = next
	if m.focus == sourcePane {
		m.previewPath = ""
		m.preview = nil
	}
	return true
}

func (m *model) selectedTask() (task, bool) {
	if m.indexMode {
		tasks := m.indexTasks()
		if m.indexCursor >= 0 && m.indexCursor < len(tasks) {
			return tasks[m.indexCursor], true
		}
		return task{}, false
	}
	row, ok := m.selectedNavigationRow()
	if ok && row.kind == rowTask {
		return row.todo, true
	}
	return task{}, false
}

func (m *model) activePane() pane {
	if m.focus == detailPane {
		return m.detailFrom
	}
	return m.focus
}

func (m *model) startCategoryInput() (tea.Model, tea.Cmd) {
	selected, ok := m.selectedTask()
	if !ok {
		m.status = "Select a Markdown task to edit its category"
		return m, nil
	}
	if selected.branch != "" {
		m.status = "Categories are only for general tasks"
		return m, nil
	}
	m.editTask = selected
	m.input.Prompt = "Category: "
	m.input.SetValue(taskLabel(selected))
	m.categoryInput = true
	m.status = ""
	return m, m.input.Focus()
}

func (m *model) updateCategoryInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.categoryInput = false
		m.input.Blur()
		m.input.SetValue("")
		m.status = ""
		return m, nil
	case "enter":
		oldTask := m.editTask
		newLabel := normalizeLabelInput(m.input.Value())
		if err := setTaskLabel(m.file, m.editTask, m.input.Value()); err != nil {
			m.status = errorStatus(err)
			return m, nil
		}
		m.categoryInput = false
		m.input.Blur()
		m.input.SetValue("")
		if !m.indexMode && m.activePane() == generalPane {
			m.generalLabel = newLabel
			m.generalCursor = 0
		}
		if err := m.refresh(); err != nil {
			m.status = err.Error()
			return m, nil
		}
		if m.indexMode {
			m.selectIndexTask(oldTask)
		} else if m.activePane() == generalPane {
			if newLabel != "" {
				root := *m
				root.generalLabel = ""
				for i, row := range root.generalRows() {
					if row.kind == rowLabel && strings.EqualFold(row.name, newLabel) {
						m.generalRootCursor = i
						break
					}
				}
			}
			for i, row := range m.generalRows() {
				if row.kind == rowTask && row.todo.text == oldTask.text && row.todo.branch == oldTask.branch && strings.EqualFold(taskLabel(row.todo), newLabel) {
					m.generalCursor = i
					break
				}
			}
		}
		m.status = ""
		return m, nil
	}
	if msg.Code == tea.KeySpace {
		return m, nil
	}
	msg.Text = stripLabelSpaces(msg.Text)
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func stripLabelSpaces(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
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
	if err := toggleTask(m.file, selected); err != nil {
		m.status = errorStatus(err)
		return
	}
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return
	}
	if !m.indexMode {
		m.selectNavigationTask(selected)
	}
	m.status = ""
}

func errorStatus(err error) string {
	if errors.Is(err, errTaskChanged) {
		return "Task changed on disk; press r to reload"
	}
	return err.Error()
}

func (m *model) blockMissingBranch(t task) bool {
	if !m.branchMissing(t.branch) {
		return false
	}
	m.status = missingBranchStatus
	return true
}

func (m *model) cyclePriority() {
	selected, ok := m.selectedTask()
	if !ok {
		m.status = "Select a Markdown task to set priority"
		return
	}
	if m.blockMissingBranch(selected) {
		return
	}
	if err := setTaskPriority(m.file, selected, selected.priority.next()); err != nil {
		m.status = errorStatus(err)
		return
	}
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = ""
}

func (m *model) openSource() tea.Cmd {
	if m.sourceCursor >= len(m.source) {
		return nil
	}
	item := m.source[m.sourceCursor]
	return openEditor(filepath.Join(m.scanDir(), item.path), item.line)
}

func openEditor(path string, line int) tea.Cmd {
	cmd := editorProcess(path, line)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editorFinishedMsg{err: err} })
}

func editorProcess(path string, line int) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append([]string{}, parts[1:]...)
	switch filepath.Base(parts[0]) {
	case "code", "codium", "cursor":
		args = append(args, "--wait", "--goto", fmt.Sprintf("%s:%d", path, line))
	case "vi", "vim", "nvim", "view":
		args = append(args, fmt.Sprintf("+%d", line), path)
	default:
		args = append(args, path)
	}
	return exec.Command(parts[0], args...)
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

func (m *model) refresh() error {
	return m.readTasks(false)
}

func (m *model) readTasks(sortByPriority bool) error {
	previous, hadSelection := m.selectedTask()
	previousTasks := m.allTasks
	tasks, err := loadTasks(m.file)
	if err != nil {
		return err
	}
	if sortByPriority {
		m.allTasks = sortedTasksByPriority(tasks)
		m.indexPriorityExplicit = false
	} else {
		m.allTasks = preserveTaskOrder(previousTasks, tasks)
	}
	m.partitionTasks()
	if m.generalLabel != "" {
		found := false
		for _, t := range m.general {
			if taskHasLabel(t, m.generalLabel) {
				found = true
				break
			}
		}
		if !found {
			m.generalLabel = ""
			m.generalCursor = m.generalRootCursor
		}
	}
	if m.branchFilter != "" {
		found := false
		for _, t := range m.branches {
			if t.branch == m.branchFilter {
				found = true
				break
			}
		}
		if !found {
			m.branchFilter = ""
			m.branchCursor = m.branchRootCursor
		}
	}
	m.generalCursor = min(m.generalCursor, max(0, len(m.generalRows())-1))
	m.branchCursor = min(m.branchCursor, max(0, len(m.branchRows())-1))
	if m.indexMode {
		m.indexCursor = min(m.indexCursor, max(0, len(m.allTasks)-1))
		if hadSelection {
			m.selectIndexTask(previous)
		}
	}
	return nil
}

func preserveTaskOrder(previous, loaded []task) []task {
	if len(previous) == 0 || len(loaded) == 0 {
		return loaded
	}
	type key struct{ text, branch, label string }
	identity := func(item task) key {
		return key{item.text, item.branch, strings.ToLower(taskLabel(item))}
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
		best, bestOffset, distance := -1, -1, int(^uint(0)>>1)
		for offset, i := range candidates {
			d := loaded[i].line - old.line
			if d < 0 {
				d = -d
			}
			if d < distance {
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
		best, distance := -1, int(^uint(0)>>1)
		for i, candidate := range loaded {
			if used[i] || candidate.branch != old.branch || !strings.EqualFold(taskLabel(candidate), taskLabel(old)) {
				continue
			}
			d := candidate.line - old.line
			if d < 0 {
				d = -d
			}
			if d < distance {
				best, distance = i, d
			}
		}
		if best >= 0 {
			assigned[oldIndex] = best
			used[best] = true
		}
	}
	ordered := make([]task, 0, len(loaded))
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

func (m *model) indexTasks() []task {
	tasks := append([]task(nil), m.allTasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		switch m.indexSort {
		case sortBranch:
			if c := compareIndexGroup(a.branch, b.branch); c != 0 {
				return c < 0
			}
			if a.branch == "" {
				if c := compareIndexGroup(taskLabel(a), taskLabel(b)); c != 0 {
					return c < 0
				}
			}
		case sortCategory:
			if c := compareIndexGroup(taskLabel(a), taskLabel(b)); c != 0 {
				return c < 0
			}
			if taskLabel(a) == "" {
				if c := compareIndexGroup(a.branch, b.branch); c != 0 {
					return c < 0
				}
			}
		}
		if a.done != b.done {
			return !a.done
		}
		if m.indexSort == sortPriority && m.indexPriorityExplicit {
			return a.priority.rank() < b.priority.rank()
		}
		return false
	})
	return tasks
}

func compareIndexGroup(a, b string) int {
	if a == "" && b != "" {
		return 1
	}
	if b == "" && a != "" {
		return -1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func (m *model) selectIndexTask(selected task) {
	tasks := m.indexTasks()
	best, distance := -1, int(^uint(0)>>1)
	for i, t := range tasks {
		if t.text == selected.text && t.branch == selected.branch {
			d := t.line - selected.line
			if d < 0 {
				d = -d
			}
			if d < distance {
				best, distance = i, d
			}
		}
	}
	if best >= 0 {
		m.indexCursor = best
	}
}

func (m *model) moveIndexCursor(delta int) bool {
	next := max(0, min(m.indexCursor+delta, len(m.allTasks)-1))
	if next == m.indexCursor {
		return false
	}
	m.indexCursor = next
	return true
}

func (m *model) updateIndex(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i", "esc", "left":
		m.indexMode = false
		m.status = ""
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "s":
		selected, ok := m.selectedTask()
		m.indexSort = m.indexSort.next()
		m.indexPriorityExplicit = m.indexSort == sortPriority
		if ok {
			m.selectIndexTask(selected)
		}
	case "p":
		m.cyclePriority()
	case "c", "l":
		return m.startCategoryInput()
	case "b":
		return m.startTaskModal(modalAddBranch)
	case "space", "d":
		m.toggleSelected()
	case "enter", "e":
		return m.startTaskModal(modalEdit)
	case "r":
		m.project = currentProject()
		if err := m.reload(); err != nil {
			m.status = err.Error()
		} else {
			m.status = ""
		}
		m.sourceLoading = true
		return m, m.scanCmd()
	}
	return m, nil
}

func (m *model) partitionTasks() {
	m.general = nil
	m.branches = nil
	for _, t := range m.allTasks {
		if t.branch == "" {
			m.general = append(m.general, t)
		} else {
			m.branches = append(m.branches, t)
		}
	}
}
