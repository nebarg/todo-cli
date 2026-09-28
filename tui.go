package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

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

type model struct {
	file                  string
	project               projectContext
	allTasks              []task
	indexMode             bool
	indexSort             string
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
	branchLabel           string
	branchLabelRootCursor int
	sourceCursor          int
	sourceLoading         bool
	sourceScanned         bool
	sourceError           string
	preview               []previewLine
	previewPath           string
	previewLine           int
	previewError          string
	input                 textinput.Model
	inputMode             string
	editTask              task
	modal                 *taskModal
	status                string
	width                 int
	height                int
}

func newModel(file string, project projectContext) (model, error) {
	input := textinput.New()
	input.Prompt = "New task: "
	input.Placeholder = "What needs doing?"
	input.SetWidth(72)
	m := model{file: file, project: project, input: input, width: 100, height: 30, sourceLoading: true, indexSort: "priority"}
	if err := m.reload(); err != nil {
		return model{}, err
	}
	m.preselectCurrentBranch()
	return m, nil
}

func (m model) Init() tea.Cmd { return m.scanCmd() }

func (m model) scanDir() string {
	if m.project.root != "" {
		return m.project.root
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func (m model) scanCmd() tea.Cmd {
	dir := m.scanDir()
	return func() tea.Msg {
		matches, err := scanSource(dir, 1000, false)
		return sourceScanMsg{matches: matches, err: err}
	}
}

func (m model) previewCmd() tea.Cmd {
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.modal != nil {
			return m.updateTaskModal(msg)
		}
		if m.inputMode != "" {
			return m.updateInput(msg)
		}
		if m.indexMode {
			return m.updateIndex(msg)
		}
		switch key {
		case "q":
			return m, tea.Quit
		case "i":
			m.indexMode = true
			m.indexSort = "priority"
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
					if _, ok := m.selectedTask(); ok || m.focus == sourcePane {
						m.detailFrom = m.focus
						m.focus = detailPane
						m.detailScroll = 0
					}
				}
			}
		case "left":
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
			return m.startTaskModal("add-general")
		case "b":
			return m.startTaskModal("add-branch")
		case "l":
			return m.startInput("labels")
		case "e":
			if m.activePane() == sourcePane {
				return m, m.openSource()
			}
			return m.startTaskModal("edit")
		case "p":
			m.cyclePriority()
		case "space":
			m.toggleSelected()
		case "enter":
			if m.activePane() == sourcePane {
				return m, m.openSource()
			}
			if !m.enterSelectedGroup() {
				m.toggleSelected()
			}
		case "v":
			m.focus = branchPane
			m.branchFilter = ""
			m.branchLabel = ""
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
		return moveIndexCursor(m, delta)
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

func (m model) selectedTask() (task, bool) {
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

func (m model) activePane() pane {
	if m.focus == detailPane {
		return m.detailFrom
	}
	return m.focus
}

func (m model) startInput(mode string) (tea.Model, tea.Cmd) {
	if mode == "labels" {
		selected, ok := m.selectedTask()
		if !ok {
			m.status = "Select a Markdown task to edit its label"
			return m, nil
		}
		m.editTask = selected
		m.input.Prompt = "Label: "
		m.input.SetValue(taskLabel(selected))
	}
	m.inputMode = mode
	m.status = ""
	return m, m.input.Focus()
}

func (m model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = ""
		m.input.Blur()
		m.input.SetValue("")
		m.status = ""
		return m, nil
	case "enter":
		mode := m.inputMode
		oldTask := m.editTask
		newLabel := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(m.input.Value()), "#@"))
		var err error
		switch mode {
		case "labels":
			err = setTaskLabel(m.file, m.editTask, m.input.Value())
		}
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.inputMode = ""
		m.input.Blur()
		m.input.SetValue("")
		if mode == "labels" && !m.indexMode {
			switch m.activePane() {
			case generalPane:
				m.generalLabel = newLabel
				m.generalCursor = 0
			case branchPane:
				m.branchLabel = newLabel
				m.branchCursor = 0
			}
		}
		if err := m.reload(); err != nil {
			m.status = err.Error()
			return m, nil
		}
		if mode == "labels" && m.indexMode {
			m.selectIndexTask(oldTask)
		} else if mode == "labels" {
			if m.activePane() == generalPane {
				if newLabel != "" {
					root := m
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
			} else if m.activePane() == branchPane {
				if newLabel != "" {
					root := m
					root.branchLabel = ""
					for i, row := range root.branchRows() {
						if row.kind == rowBranchLabel && strings.EqualFold(row.name, newLabel) {
							m.branchLabelRootCursor = i
							break
						}
					}
				}
				for i, row := range m.branchRows() {
					if row.kind == rowTask && row.todo.text == oldTask.text && row.todo.branch == oldTask.branch && strings.EqualFold(taskLabel(row.todo), newLabel) {
						m.branchCursor = i
						break
					}
				}
			}
		}
		m.status = ""
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) toggleSelected() {
	selected, ok := m.selectedTask()
	if !ok {
		if m.activePane() == sourcePane {
			m.status = "File TODOs are read only"
		} else {
			m.status = "Open a label or branch to select a task"
		}
		return
	}
	if err := toggleTask(m.file, selected); err != nil {
		m.status = err.Error()
		return
	}
	if err := m.reload(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = ""
}

func (m *model) cyclePriority() {
	selected, ok := m.selectedTask()
	if !ok {
		m.status = "Select a Markdown task to set priority"
		return
	}
	next := map[string]string{"": "high", "high": "medium", "medium": "low", "low": ""}[selected.priority]
	if err := setTaskPriority(m.file, selected, next); err != nil {
		m.status = err.Error()
		return
	}
	if err := m.reload(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = ""
}

func (m model) openSource() tea.Cmd {
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

func (m *model) reload() error {
	previous, hadSelection := m.selectedTask()
	tasks, err := loadTasks(m.file)
	if err != nil {
		return err
	}
	m.allTasks = tasks
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
			m.branchLabel = ""
			m.branchCursor = m.branchRootCursor
		}
	}
	if m.branchLabel != "" {
		found := false
		for _, t := range m.branches {
			if t.branch == m.branchFilter && taskHasLabel(t, m.branchLabel) {
				found = true
				break
			}
		}
		if !found {
			m.branchLabel = ""
			m.branchCursor = m.branchLabelRootCursor
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

func (m model) indexTasks() []task {
	tasks := append([]task(nil), m.allTasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		switch m.indexSort {
		case "branch":
			if c := compareIndexGroup(a.branch, b.branch); c != 0 {
				return c < 0
			}
		case "label":
			if c := compareIndexGroup(taskLabel(a), taskLabel(b)); c != 0 {
				return c < 0
			}
		}
		return priorityRank(a.priority) < priorityRank(b.priority)
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

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
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

func moveIndexCursor(m *model, delta int) bool {
	next := max(0, min(m.indexCursor+delta, len(m.allTasks)-1))
	if next == m.indexCursor {
		return false
	}
	m.indexCursor = next
	return true
}

func (m model) updateIndex(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case "p", "b", "l":
		selected, ok := m.selectedTask()
		m.indexSort = map[string]string{"p": "priority", "b": "branch", "l": "label"}[msg.String()]
		if ok {
			m.selectIndexTask(selected)
		}
	case "space", "enter":
		m.toggleSelected()
	case "e":
		return m.startTaskModal("edit")
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
