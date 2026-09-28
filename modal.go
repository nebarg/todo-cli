package main

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type taskModal struct {
	mode      string
	selected  task
	addBranch string
	addLabel  string
	title     textinput.Model
	details   textarea.Model
	field     int // 0: title, 1: details
	err       string
}

func (m model) startTaskModal(mode string) (tea.Model, tea.Cmd) {
	modal := &taskModal{mode: mode}
	if mode == "add-general" && m.activePane() == generalPane {
		modal.addLabel = m.generalLabel
		if modal.addLabel == "" {
			if row, ok := m.selectedNavigationRow(); ok {
				if row.kind == rowLabel {
					modal.addLabel = row.name
				} else if row.kind == rowTask {
					modal.addLabel = taskLabel(row.todo)
				}
			}
		}
	}
	if mode == "add-branch" {
		modal.addBranch = m.project.branch
		if m.activePane() == branchPane {
			if m.branchFilter != "" {
				modal.addBranch = m.branchFilter
				modal.addLabel = m.branchLabel
				if row, ok := m.selectedNavigationRow(); ok {
					if row.kind == rowBranchLabel {
						modal.addLabel = row.name
					} else if row.kind == rowTask {
						modal.addLabel = taskLabel(row.todo)
					}
				}
			} else if row, ok := m.selectedNavigationRow(); ok && row.kind == rowBranch {
				modal.addBranch = row.name
			}
		}
		if modal.addBranch == "" {
			m.status = "Select a branch or check out a Git branch"
			return m, nil
		}
	}
	if mode == "edit" {
		selected, ok := m.selectedTask()
		if !ok {
			m.status = "Select a Markdown task to edit"
			return m, nil
		}
		modal.selected = selected
	}
	modal.title = textinput.New()
	modal.title.Prompt = ""
	modal.title.Placeholder = "What needs doing?"
	modal.details = textarea.New()
	modal.details.Prompt = ""
	modal.details.ShowLineNumbers = false
	modal.details.Placeholder = "Add context, steps, or links…"
	if mode == "edit" {
		modal.title.SetValue(modal.selected.text)
		modal.details.SetValue(modal.selected.details)
	}
	modal.resize(m.width, m.height)
	m.modal = modal
	m.status = ""
	return m, modal.title.Focus()
}

func (m model) updateTaskModal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	modal := m.modal
	switch msg.String() {
	case "esc":
		modal.title.Blur()
		modal.details.Blur()
		m.modal = nil
		return m, nil
	case "ctrl+s", "ctrl+enter":
		if err := modal.save(m.file); err != nil {
			modal.err = err.Error()
			return m, nil
		}
		mode := modal.mode
		m.modal = nil
		if err := m.reload(); err != nil {
			m.status = err.Error()
			return m, nil
		}
		if mode == "add-general" {
			m.focus = generalPane
			m.generalLabel = modal.addLabel
			if modal.addLabel != "" {
				root := m
				root.generalLabel = ""
				for i, row := range root.generalRows() {
					if row.kind == rowLabel && strings.EqualFold(row.name, modal.addLabel) {
						m.generalRootCursor = i
						break
					}
				}
			}
			for i, row := range m.generalRows() {
				if row.kind == rowTask && row.todo.branch == "" && row.todo.text == strings.TrimSpace(modal.title.Value()) && strings.EqualFold(taskLabel(row.todo), modal.addLabel) {
					m.generalCursor = i
				}
			}
		} else if mode == "add-branch" {
			m.focus = branchPane
			branchRoot := m
			branchRoot.branchFilter = ""
			branchRoot.branchLabel = ""
			for i, row := range branchRoot.branchRows() {
				if row.name == modal.addBranch {
					m.branchRootCursor = i
					break
				}
			}
			m.branchFilter = modal.addBranch
			m.branchLabel = modal.addLabel
			if modal.addLabel != "" {
				root := m
				root.branchLabel = ""
				for i, row := range root.branchRows() {
					if row.kind == rowBranchLabel && strings.EqualFold(row.name, modal.addLabel) {
						m.branchLabelRootCursor = i
						break
					}
				}
			}
			for i, row := range m.branchRows() {
				if row.kind == rowTask && row.todo.text == strings.TrimSpace(modal.title.Value()) && strings.EqualFold(taskLabel(row.todo), modal.addLabel) {
					m.branchCursor = i
				}
			}
		}
		m.detailScroll = 0
		m.status = ""
		return m, nil
	case "tab", "shift+tab":
		return m, modal.focusField(1 - modal.field)
	case "down", "enter":
		if modal.field == 0 {
			return m, modal.focusField(1)
		}
	}
	modal.err = ""
	var cmd tea.Cmd
	if modal.field == 0 {
		modal.title, cmd = modal.title.Update(msg)
	} else {
		modal.details, cmd = modal.details.Update(msg)
	}
	return m, cmd
}

func (f *taskModal) focusField(field int) tea.Cmd {
	f.field = field
	if field == 0 {
		f.details.Blur()
		return f.title.Focus()
	}
	f.title.Blur()
	return f.details.Focus()
}

func (f *taskModal) save(path string) error {
	if f.mode == "edit" {
		return editTaskContent(path, f.selected, f.title.Value(), f.details.Value())
	}
	var labels []string
	if f.addLabel != "" {
		labels = []string{f.addLabel}
	}
	return addTaskWithDetails(path, f.title.Value(), f.details.Value(), "", labels, f.addBranch)
}

func (f *taskModal) dimensions(width, height int) (int, int) {
	return min(width-4, 76), min(height-4, 20)
}

func (f *taskModal) resize(width, height int) {
	modalWidth, modalHeight := f.dimensions(width, height)
	innerWidth := max(1, modalWidth-6)
	f.title.SetWidth(innerWidth)
	f.details.SetWidth(innerWidth)
	f.details.SetHeight(max(2, modalHeight-10))
}

func (f *taskModal) render(width, height int) string {
	innerWidth := max(1, width-6)
	heading := "Add general task"
	if f.mode == "add-branch" {
		heading = "Add branch task · " + f.addBranch
	} else if f.mode == "edit" {
		heading = "Edit task"
	}
	if f.mode != "edit" && f.addLabel != "" {
		heading += " · @" + f.addLabel
	}
	titleLabel := mutedStyle.Render("Title")
	detailsLabel := mutedStyle.Render("Details")
	if f.field == 0 {
		titleLabel = titleStyle.Render("Title")
	} else {
		detailsLabel = titleStyle.Render("Details")
	}
	help := "Tab / ↓  details   Ctrl+S  save   Esc  cancel"
	if f.field == 1 {
		help = "Tab  title   Ctrl+S  save   Esc  cancel"
	}
	if f.err != "" {
		help = f.err
	}
	lines := []string{
		titleStyle.Render(ansi.Truncate(heading, innerWidth, "…")), "", titleLabel, f.title.View(), "", detailsLabel,
		f.details.View(), "", mutedStyle.Render(ansi.Truncate(help, innerWidth, "…")),
	}
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorFocus).
		Background(lipgloss.Color("#111E2F")).Render(strings.Join(lines, "\n"))
}

func (f *taskModal) cursor(x, y int) *tea.Cursor {
	var cursor *tea.Cursor
	if f.field == 0 {
		cursor = f.title.Cursor()
		if cursor != nil {
			cursor.X += x + 3
			cursor.Y += y + 4
		}
	} else {
		cursor = f.details.Cursor()
		if cursor != nil {
			cursor.X += x + 3
			cursor.Y += y + 7
		}
	}
	return cursor
}
