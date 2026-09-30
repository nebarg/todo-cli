package main

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type modalMode int

const (
	modalAddGeneral modalMode = iota
	modalAddBranch
	modalEdit
)

type taskModal struct {
	mode         modalMode
	selected     task
	project      projectContext
	addBranch    string
	addCategory  string
	branches     []string
	branchCursor int
	branchFresh  bool
	title        textarea.Model
	scope        textinput.Model
	details      textarea.Model
	field        int // add: title, scope, details; edit: title, details
	err          string
}

func (m *model) startTaskModal(mode modalMode) (tea.Model, tea.Cmd) {
	modal := &taskModal{mode: mode, project: m.project}
	if mode == modalAddGeneral && m.activePane() == generalPane {
		modal.addCategory = m.generalCategory
	}
	if mode == modalAddBranch {
		modal.branches, modal.addBranch = m.checkLocalBranches()
		if !m.indexMode && m.activePane() == branchPane && m.branchFilter != "" {
			modal.addBranch = m.branchFilter
		}
		if !slices.Contains(modal.branches, modal.addBranch) {
			modal.branchCursor = -1
		}
	}
	if mode == modalEdit {
		selected, ok := m.selectedTask()
		if !ok {
			m.status = "Select a Markdown task to edit"
			return m, nil
		}
		if selected.branch != "" {
			m.checkLocalBranches()
		}
		if m.blockMissingBranch(selected) {
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
	switch mode {
	case modalAddGeneral:
		modal.scope.Placeholder = "Optional category"
		modal.scope.SetValue(modal.addCategory)
	case modalAddBranch:
		modal.scope.Placeholder = "Search local branches"
		modal.scope.SetValue(modal.addBranch)
	}
	modal.details = textarea.New()
	modal.details.Prompt = ""
	modal.details.ShowLineNumbers = false
	modal.details.Placeholder = "Add context, steps, or links…"
	if mode == modalEdit {
		modal.title.SetValue(modal.selected.text)
		modal.details.SetValue(modal.selected.details)
	}
	modal.resize(m.width, m.height)
	m.modal = modal
	m.status = ""
	return m, modal.title.Focus()
}

func (m *model) updateTaskModal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
			modal.err = errorStatus(err)
			return m, nil
		}
		mode := modal.mode
		m.modal = nil
		if err := m.refresh(); err != nil {
			m.status = err.Error()
			return m, nil
		}
		if mode == modalAddGeneral {
			m.focus = generalPane
			m.generalCategory = modal.addCategory
			if modal.addCategory != "" {
				root := *m
				root.generalCategory = ""
				for i, row := range root.generalRows() {
					if row.kind == rowCategory && strings.EqualFold(row.name, modal.addCategory) {
						m.generalRootCursor = i
						break
					}
				}
			}
			for i, row := range m.generalRows() {
				if row.kind == rowTask && row.todo.branch == "" && row.todo.text == modal.taskTitle() && strings.EqualFold(row.todo.category, modal.addCategory) {
					m.generalCursor = i
				}
			}
		} else if mode == modalAddBranch {
			m.indexMode = false
			m.focus = branchPane
			branchRoot := *m
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
		if modal.mode == modalAddBranch && modal.field == 1 {
			modal.acceptBranch()
		}
		return m, modal.focusField((modal.field + 1) % (modal.detailsField() + 1))
	case "shift+tab":
		return m, modal.focusField((modal.field + modal.detailsField()) % (modal.detailsField() + 1))
	case "down":
		if modal.mode == modalAddBranch && modal.field == 1 {
			if count := len(modal.matchingBranches()); count > 0 {
				modal.branchCursor = (modal.branchCursor + 1) % count
			}
			return m, nil
		}
		if modal.field == 0 && modal.title.Line() < strings.Count(modal.title.Value(), "\n") {
			break
		}
		if modal.field < modal.detailsField() {
			return m, modal.focusField(modal.field + 1)
		}
	case "enter":
		if modal.field == 1 && modal.mode != modalEdit {
			if modal.mode == modalAddBranch {
				modal.acceptBranch()
			}
			return m, modal.focusField(modal.detailsField())
		}
	case "up":
		if modal.field == 0 && modal.title.Line() == 0 {
			return m, nil
		}
		if modal.field == 1 && modal.mode != modalEdit {
			if modal.mode == modalAddBranch {
				if count := len(modal.matchingBranches()); count > 0 {
					modal.branchCursor = (modal.branchCursor - 1 + count) % count
				}
				return m, nil
			}
			return m, modal.focusField(0)
		}
		if modal.field == modal.detailsField() && modal.details.Line() == 0 {
			return m, modal.focusField(modal.field - 1)
		}
	}
	modal.err = ""
	var cmd tea.Cmd
	switch {
	case modal.field == 0:
		modal.title, cmd = modal.title.Update(msg)
	case modal.field == 1 && modal.mode != modalEdit:
		if modal.mode == modalAddGeneral {
			if msg.Code == tea.KeySpace {
				return m, nil
			}
			msg.Text = stripCategorySpaces(msg.Text)
		} else if modal.branchFresh && msg.Text != "" {
			modal.scope.SetValue("")
		}
		modal.scope, cmd = modal.scope.Update(msg)
		if modal.mode == modalAddBranch {
			modal.branchFresh = false
			modal.resetBranchCursor()
		}
	default:
		modal.details, cmd = modal.details.Update(msg)
	}
	return m, cmd
}

func (m *model) updateTaskModalPaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	modal := m.modal
	modal.err = ""
	var cmd tea.Cmd
	switch {
	case modal.field == 0:
		modal.title, cmd = modal.title.Update(msg)
	case modal.field == 1 && modal.mode != modalEdit:
		if modal.mode == modalAddGeneral {
			msg.Content = stripCategorySpaces(msg.Content)
		} else if modal.branchFresh {
			modal.scope.SetValue("")
		}
		modal.scope, cmd = modal.scope.Update(msg)
		if modal.mode == modalAddBranch {
			modal.branchFresh = false
			modal.resetBranchCursor()
		}
	default:
		modal.details, cmd = modal.details.Update(msg)
	}
	return m, cmd
}

func (f *taskModal) detailsField() int {
	if f.mode == modalEdit {
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
	if field == 1 && f.mode != modalEdit {
		if f.mode == modalAddBranch {
			f.branchFresh = true
		}
		return f.scope.Focus()
	}
	return f.details.Focus()
}

func (f *taskModal) save(path string) error {
	if f.mode == modalEdit {
		return editTaskContent(path, f.selected, f.taskTitle(), f.details.Value())
	}
	category, branch := "", ""
	if f.mode == modalAddGeneral {
		f.addCategory = normalizeCategoryInput(f.scope.Value())
		category = f.addCategory
	} else {
		f.addBranch = f.chosenBranch()
		if f.addBranch == "" {
			if len(f.branches) == 0 {
				return errors.New("no local Git branches found")
			}
			return errors.New("choose an existing local Git branch")
		}
		if !f.project.hasLocalBranch(f.addBranch) {
			return fmt.Errorf("branch %q no longer exists locally", f.addBranch)
		}
		branch = f.addBranch
	}
	return addTaskWithDetails(path, f.taskTitle(), f.details.Value(), priorityNone, category, branch)
}

func (f *taskModal) taskTitle() string {
	return strings.Join(strings.Fields(f.title.Value()), " ")
}

func (f *taskModal) matchingBranches() []string {
	query := strings.ToLower(strings.TrimSpace(f.scope.Value()))
	var matches []string
	for _, branch := range f.branches {
		if strings.Contains(strings.ToLower(branch), query) {
			matches = append(matches, branch)
		}
	}
	if query != "" {
		rank := func(branch string) int {
			name := strings.ToLower(branch)
			if name == query {
				return 0
			}
			if strings.HasPrefix(name, query) {
				return 1
			}
			return 2
		}
		sort.SliceStable(matches, func(i, j int) bool {
			return rank(matches[i]) < rank(matches[j])
		})
	}
	return matches
}

func (f *taskModal) resetBranchCursor() {
	if strings.TrimSpace(f.scope.Value()) == "" || len(f.matchingBranches()) == 0 {
		f.branchCursor = -1
	} else {
		f.branchCursor = 0
	}
}

func (f *taskModal) chosenBranch() string {
	query := strings.TrimSpace(f.scope.Value())
	matches := f.matchingBranches()
	if f.branchCursor >= 0 && f.branchCursor < len(matches) {
		return matches[f.branchCursor]
	}
	for _, branch := range f.branches {
		if branch == query && query != "" {
			return branch
		}
	}
	return ""
}

func (f *taskModal) acceptBranch() {
	if branch := f.chosenBranch(); branch != "" {
		f.scope.SetValue(branch)
		f.branchCursor = 0
		f.branchFresh = true
	}
}

func (f *taskModal) branchSuggestions(width int) []string {
	rows := 2
	matches := f.matchingBranches()
	lines := make([]string, 0, rows)
	if len(matches) == 0 {
		message := "No matching local branches"
		if len(f.branches) == 0 {
			message = "No local Git branches"
		}
		lines = append(lines, mutedStyle.Render(ansi.Truncate(message, width, "…")))
	} else {
		start := max(0, f.branchCursor-rows+1)
		for i := start; i < len(matches) && len(lines) < rows; i++ {
			mark := "  "
			style := mutedStyle
			if i == f.branchCursor {
				mark = "› "
				style = lipgloss.NewStyle().Foreground(colorGreen)
				if f.field == 1 {
					style = style.Background(colorSelection)
				}
			}
			lineWidth := width
			if len(matches) > rows {
				lineWidth = max(1, width-2)
			}
			line := ansi.Truncate(mark+" "+matches[i], lineWidth, "…")
			if len(matches) > rows {
				line += strings.Repeat(" ", max(0, lineWidth-ansi.StringWidth(line)))
				thumb := min(rows-1, max(0, f.branchCursor)*rows/len(matches))
				bar := "│"
				if i-start == thumb {
					bar = "┃"
				}
				lines = append(lines, style.Render(line)+mutedStyle.Render(" "+bar))
			} else {
				lines = append(lines, style.Render(line))
			}
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines
}

func (f *taskModal) dimensions(width, height int) (int, int) {
	modalHeight := min(height-4, 20)
	if f.mode == modalAddBranch {
		modalHeight = min(height, max(modalHeight, 13))
	}
	return min(width-2, 76), modalHeight
}

// isCompact drops spacer lines so add forms still fit short terminals.
func (f *taskModal) isCompact(height int) bool {
	return f.mode != modalEdit && (height < 15 || f.mode == modalAddBranch && height < 18)
}

func (f *taskModal) resize(width, height int) {
	modalWidth, modalHeight := f.dimensions(width, height)
	compact := f.isCompact(modalHeight)
	innerWidth := max(1, modalWidth-6)
	f.title.SetWidth(innerWidth)
	f.title.SetHeight(2)
	if f.mode != modalEdit {
		f.scope.SetWidth(max(1, innerWidth/2))
	}
	f.details.SetWidth(innerWidth)
	var detailsSpace int
	switch {
	case f.mode == modalEdit:
		detailsSpace = modalHeight - 11
	case !compact && f.mode == modalAddBranch:
		detailsSpace = modalHeight - 15
	case !compact:
		detailsSpace = modalHeight - 13
	case f.mode == modalAddBranch:
		detailsSpace = modalHeight - 11
	default:
		detailsSpace = modalHeight - 10
	}
	f.details.SetHeight(max(2, detailsSpace))
}

func (f *taskModal) render(width, height int) string {
	innerWidth := max(1, width-6)
	heading := "Add general task"
	switch f.mode {
	case modalAddBranch:
		heading = "Add branch task"
	case modalEdit:
		heading = "Edit task"
	}
	headingStyle := titleStyle
	if f.err != "" {
		heading = f.err
		headingStyle = lipgloss.NewStyle().Bold(true).Foreground(colorHigh)
	}
	detailsHeading := mutedStyle.Render("Details")
	if f.field == f.detailsField() {
		detailsHeading = titleStyle.Render("Details")
	}
	lines := []string{headingStyle.Render(ansi.Truncate(heading, innerWidth, "…"))}
	compact := f.isCompact(height)
	if f.mode == modalEdit {
		lines = append(lines, f.editLocation(innerWidth))
	}
	if f.mode == modalEdit || !compact {
		lines = append(lines, "")
	}
	lines = append(lines, f.title.View())
	if f.mode != modalEdit {
		if f.mode != modalAddBranch || !compact {
			lines = append(lines, "")
		}
		scopeName := "Category"
		if f.mode == modalAddBranch {
			scopeName = "Branch"
		}
		scopeStyle := mutedStyle
		if f.field == 1 {
			scopeStyle = titleStyle
		}
		lines = append(lines, scopeStyle.Render(scopeName), f.scope.View())
		if f.mode == modalAddBranch {
			lines = append(lines, f.branchSuggestions(max(1, innerWidth/2))...)
		}
		if !compact {
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, "")
	}
	lines = append(lines, detailsHeading, f.details.View())
	if !compact && (f.mode != modalEdit || height >= 13) {
		lines = append(lines, "")
	}
	help := "↑/↓/tab navigate · ctrl+enter save · esc cancel"
	if f.mode == modalAddBranch && f.field == 1 {
		help = "↑/↓ cycle · tab next · ctrl+enter save · esc cancel"
		if compact {
			help = "↑/↓ cycle · ctrl+enter save · esc cancel"
		}
	}
	lines = append(lines, mutedStyle.Render(ansi.Truncate(help, innerWidth, "…")))
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorFocus).
		Background(colorModal).Render(strings.Join(lines, "\n"))
}

func (f *taskModal) editLocation(width int) string {
	if f.selected.branch != "" {
		return ansi.Truncate(mutedStyle.Render("Branch    ")+lipgloss.NewStyle().Foreground(colorGreen).Render(" "+f.selected.branch), width, "…")
	}
	if category := f.selected.category; category != "" {
		return ansi.Truncate(mutedStyle.Render("Category  ")+lipgloss.NewStyle().Foreground(colorPurple).Render("@"+category), width, "…")
	}
	return mutedStyle.Render("General task")
}
