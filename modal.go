package main

import (
	"errors"
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
	title     textarea.Model
	scope     textinput.Model
	details   textarea.Model
	field     int // add: title, scope, details; edit: title, details
	compact   bool
	err       string
}

func (m model) startTaskModal(mode string) (tea.Model, tea.Cmd) {
	modal := &taskModal{mode: mode}
	if mode == "add-general" && m.activePane() == generalPane {
		modal.addLabel = m.generalLabel
	}
	if mode == "add-branch" {
		modal.addBranch = m.project.branch
		if m.activePane() == branchPane {
			if m.branchFilter != "" {
				modal.addBranch = m.branchFilter
			} else if row, ok := m.selectedNavigationRow(); ok && row.kind == rowBranch {
				modal.addBranch = row.name
			}
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
	modal.title = textarea.New()
	modal.title.Prompt = ""
	modal.title.ShowLineNumbers = false
	modal.title.Placeholder = "What needs doing?"
	modal.title.SetHeight(2)
	modal.scope = textinput.New()
	modal.scope.Prompt = ""
	if mode == "add-general" {
		modal.scope.Placeholder = "Optional category"
		modal.scope.SetValue(modal.addLabel)
	} else if mode == "add-branch" {
		modal.scope.Placeholder = "Branch name"
		modal.scope.SetValue(modal.addBranch)
	}
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
		modal.scope.Blur()
		modal.details.Blur()
		m.modal = nil
		return m, nil
	case "ctrl+enter":
		if err := modal.save(m.file); err != nil {
			modal.err = err.Error()
			return m, nil
		}
		mode := modal.mode
		m.modal = nil
		if err := m.refresh(); err != nil {
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
				if row.kind == rowTask && row.todo.branch == "" && row.todo.text == modal.taskTitle() && strings.EqualFold(taskLabel(row.todo), modal.addLabel) {
					m.generalCursor = i
				}
			}
		} else if mode == "add-branch" {
			m.indexMode = false
			m.focus = branchPane
			branchRoot := m
			branchRoot.branchFilter = ""
			for i, row := range branchRoot.branchRows() {
				if row.name == modal.addBranch {
					m.branchRootCursor = i
					break
				}
			}
			m.branchFilter = modal.addBranch
			for i, row := range m.branchRows() {
				if row.kind == rowTask && row.todo.text == modal.taskTitle() {
					m.branchCursor = i
				}
			}
		}
		m.detailScroll = 0
		m.status = ""
		return m, nil
	case "tab":
		return m, modal.focusField((modal.field + 1) % (modal.detailsField() + 1))
	case "shift+tab":
		return m, modal.focusField((modal.field + modal.detailsField()) % (modal.detailsField() + 1))
	case "down":
		if modal.field == 0 && modal.title.Line() < strings.Count(modal.title.Value(), "\n") {
			break
		}
		if modal.field < modal.detailsField() {
			return m, modal.focusField(modal.field + 1)
		}
	case "enter":
		if modal.field == 1 && modal.mode != "edit" {
			return m, modal.focusField(modal.detailsField())
		}
	case "up":
		if modal.field == 0 && modal.title.Line() == 0 {
			return m, nil
		}
		if modal.field == 1 && modal.mode != "edit" {
			return m, modal.focusField(0)
		}
		if modal.field == modal.detailsField() && modal.details.Line() == 0 {
			return m, modal.focusField(modal.field - 1)
		}
	}
	modal.err = ""
	var cmd tea.Cmd
	if modal.field == 0 {
		modal.title, cmd = modal.title.Update(msg)
	} else if modal.field == 1 && modal.mode != "edit" {
		if modal.mode == "add-general" {
			if msg.Code == tea.KeySpace {
				return m, nil
			}
			msg.Text = stripLabelSpaces(msg.Text)
		}
		modal.scope, cmd = modal.scope.Update(msg)
	} else {
		modal.details, cmd = modal.details.Update(msg)
	}
	return m, cmd
}

func (m model) updateTaskModalPaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	modal := m.modal
	modal.err = ""
	var cmd tea.Cmd
	if modal.field == 0 {
		modal.title, cmd = modal.title.Update(msg)
	} else if modal.field == 1 && modal.mode != "edit" {
		if modal.mode == "add-general" {
			msg.Content = stripLabelSpaces(msg.Content)
		}
		modal.scope, cmd = modal.scope.Update(msg)
	} else {
		modal.details, cmd = modal.details.Update(msg)
	}
	return m, cmd
}

func (f *taskModal) detailsField() int {
	if f.mode == "edit" {
		return 1
	}
	return 2
}

func (f *taskModal) focusField(field int) tea.Cmd {
	f.field = field
	f.title.Blur()
	f.scope.Blur()
	f.details.Blur()
	if field == 0 {
		return f.title.Focus()
	}
	if field == 1 && f.mode != "edit" {
		return f.scope.Focus()
	}
	return f.details.Focus()
}

func (f *taskModal) save(path string) error {
	if f.mode == "edit" {
		return editTaskContent(path, f.selected, f.taskTitle(), f.details.Value())
	}
	var labels []string
	branch := ""
	if f.mode == "add-general" {
		f.addLabel = normalizeLabelInput(f.scope.Value())
		if f.addLabel != "" {
			labels = []string{f.addLabel}
		}
	} else {
		f.addBranch = strings.TrimSpace(f.scope.Value())
		if f.addBranch == "" {
			return errors.New("enter a branch name")
		}
		branch = f.addBranch
	}
	return addTaskWithDetails(path, f.taskTitle(), f.details.Value(), "", labels, branch)
}

func (f *taskModal) taskTitle() string {
	return strings.Join(strings.Fields(f.title.Value()), " ")
}

func (f *taskModal) dimensions(width, height int) (int, int) {
	return min(width-2, 76), min(height-4, 20)
}

func (f *taskModal) resize(width, height int) {
	modalWidth, modalHeight := f.dimensions(width, height)
	f.compact = f.mode != "edit" && modalHeight < 15
	innerWidth := max(1, modalWidth-6)
	f.title.SetWidth(innerWidth)
	f.title.SetHeight(2)
	if f.mode != "edit" {
		f.scope.SetWidth(max(1, innerWidth/2))
	}
	f.details.SetWidth(innerWidth)
	detailsSpace := modalHeight - 11
	if f.mode != "edit" && modalHeight >= 15 {
		detailsSpace = modalHeight - 13
	} else if f.mode != "edit" {
		detailsSpace = modalHeight - 10
	}
	f.details.SetHeight(max(2, detailsSpace))
}

func (f *taskModal) render(width, height int) string {
	innerWidth := max(1, width-6)
	heading := "Add general task"
	if f.mode == "add-branch" {
		heading = "Add branch task"
	} else if f.mode == "edit" {
		heading = "Edit task"
	}
	headingStyle := titleStyle
	if f.err != "" {
		heading = f.err
		headingStyle = lipgloss.NewStyle().Bold(true).Foreground(colorHigh)
	}
	detailsLabel := mutedStyle.Render("Details")
	if f.field == f.detailsField() {
		detailsLabel = titleStyle.Render("Details")
	}
	lines := []string{headingStyle.Render(ansi.Truncate(heading, innerWidth, "…"))}
	compact := f.mode != "edit" && height < 15
	if f.mode == "edit" {
		lines = append(lines, f.editLocation(innerWidth))
	}
	if f.mode == "edit" || !compact {
		lines = append(lines, "")
	}
	lines = append(lines, f.title.View())
	if f.mode != "edit" {
		lines = append(lines, "")
		scopeName := "Category"
		if f.mode == "add-branch" {
			scopeName = "Branch"
		}
		scopeStyle := mutedStyle
		if f.field == 1 {
			scopeStyle = titleStyle
		}
		lines = append(lines, scopeStyle.Render(scopeName), f.scope.View())
		if !compact {
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, "")
	}
	lines = append(lines, detailsLabel, f.details.View())
	if !compact && !(f.mode == "edit" && height < 13) {
		lines = append(lines, "")
	}
	lines = append(lines, mutedStyle.Render(ansi.Truncate("↑/↓/tab navigate · ctrl+enter save · esc cancel", innerWidth, "…")))
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorFocus).
		Background(lipgloss.Color("#111E2F")).Render(strings.Join(lines, "\n"))
}

func (f *taskModal) editLocation(width int) string {
	if f.selected.branch != "" {
		return ansi.Truncate(mutedStyle.Render("Branch    ")+lipgloss.NewStyle().Foreground(colorGreen).Render(" "+f.selected.branch), width, "…")
	}
	if category := taskLabel(f.selected); category != "" {
		return ansi.Truncate(mutedStyle.Render("Category  ")+lipgloss.NewStyle().Foreground(colorPurple).Render("@"+category), width, "…")
	}
	return mutedStyle.Render("General task")
}

func (f *taskModal) cursor(x, y int) *tea.Cursor {
	var cursor *tea.Cursor
	if f.field == 0 {
		cursor = f.title.Cursor()
		if cursor != nil {
			cursor.X += x + 3
			if f.mode != "edit" && f.compact {
				cursor.Y += y + 2
			} else if f.mode == "edit" {
				cursor.Y += y + 4
			} else {
				cursor.Y += y + 3
			}
		}
	} else if f.field == 1 && f.mode != "edit" {
		cursor = f.scope.Cursor()
		if cursor != nil {
			cursor.X += x + 3
			if f.compact {
				cursor.Y += y + 6
			} else {
				cursor.Y += y + 7
			}
		}
	} else {
		cursor = f.details.Cursor()
		if cursor != nil {
			cursor.X += x + 3
			if f.compact {
				cursor.Y += y + 8
			} else if f.mode == "edit" {
				cursor.Y += y + 8
			} else {
				cursor.Y += y + 10
			}
		}
	}
	return cursor
}
