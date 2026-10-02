package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

type modalMode int

const (
	modalAddGeneral modalMode = iota
	modalAddBranch
	modalEdit
)

type taskModal struct {
	mode         modalMode
	selected     store.Task
	file         string
	project      projectContext
	target       store.Section
	branches     []string
	branchCursor int
	branchFresh  bool
	title        textarea.Model
	scope        textinput.Model
	details      textarea.Model
	field        int // titleField, scopeField or detailsField
	err          string
}

const (
	titleField = iota
	scopeField
	detailsField
)

func (m *model) startTaskModal(mode modalMode) tea.Cmd {
	modal := &taskModal{mode: mode, file: m.file, project: m.project}
	if mode == modalAddGeneral && m.all == nil && m.activePane() == generalPane && m.general.open.kind == rowCategory {
		modal.target.Category = m.general.open.name
	}
	if mode == modalAddBranch {
		modal.branches, modal.target.Branch = m.checkLocalBranches()
		if m.all == nil && m.activePane() == branchPane && m.branch.open != (group{}) {
			modal.target.Branch = m.branch.open.name
		}
		if !slices.Contains(modal.branches, modal.target.Branch) {
			modal.branchCursor = -1
		}
	}
	if mode == modalEdit {
		selected, ok := m.selectedTask()
		if !ok {
			m.status = "Select a Markdown task to edit"
			return nil
		}
		if selected.Branch != "" {
			modal.branches, _ = m.checkLocalBranches()
		}
		if m.blockMissingBranch(selected) {
			return nil
		}
		modal.selected = selected
		modal.target = selected.Section
	}
	modal.title = textarea.New()
	modal.title.SetStyles(fieldAreaStyles())
	modal.title.Prompt = ""
	modal.title.ShowLineNumbers = false
	modal.title.Placeholder = "What needs doing?"
	modal.title.SetHeight(2)
	modal.scope = textinput.New()
	modal.scope.SetStyles(fieldInputStyles())
	modal.scope.Prompt = ""
	if modal.branchScope() {
		modal.scope.Placeholder = "Search local branches"
		modal.scope.SetValue(modal.target.Branch)
	} else {
		modal.scope.Placeholder = "Optional category"
		modal.scope.SetValue(modal.target.Category)
	}
	modal.details = textarea.New()
	modal.details.SetStyles(fieldAreaStyles())
	modal.details.Prompt = ""
	modal.details.ShowLineNumbers = false
	modal.details.Placeholder = "Add context, steps, or links…"
	if mode == modalEdit {
		modal.title.SetValue(modal.selected.Text)
		modal.details.SetValue(modal.selected.Details)
	}
	modal.resize(m.width, m.height)
	m.overlay = modal
	m.status = ""
	return modal.title.Focus()
}

// taskSavedMsg is a task the form added or edited, as it now is: its title,
// category and branch, with its line before the edit.
type taskSavedMsg struct {
	task  store.Task
	added bool
	moved bool // an edit filed it under another category or branch
}

// taskSaved follows a new task, or one an edit moved to another category or
// branch, to where it now lives.
func (m *model) taskSaved(msg taskSavedMsg) (tea.Model, tea.Cmd) {
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	switch {
	case msg.added || msg.moved && m.all == nil:
		m.all = nil
		m.focus = generalPane
		if msg.task.Branch != "" {
			m.focus = branchPane
		}
		m.reveal(msg.task)
	case m.all != nil:
		m.selectInAllTasks(msg.task)
	}
	m.detailScroll = 0
	m.status = ""
	return m, nil
}

func (f *taskModal) update(msg tea.Msg) (overlay, tea.Msg, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.resize(msg.Width, msg.Height)
	case tea.PasteMsg:
		return f, nil, f.paste(msg)
	case tea.KeyPressMsg:
		return f.key(msg)
	}
	return f, nil, nil
}

func (f *taskModal) view(width, height int) string {
	return f.render(f.dimensions(width, height))
}

func (f *taskModal) key(msg tea.KeyPressMsg) (overlay, tea.Msg, tea.Cmd) {
	switch msg.String() {
	case "esc":
		f.title.Blur()
		f.scope.Blur()
		f.details.Blur()
		return nil, nil, nil
	case "ctrl+enter":
		if err := f.save(); err != nil {
			f.err = errorStatus(err)
			return f, nil, nil
		}
		return nil, f.saved(), nil
	case "tab":
		if f.branchScope() && f.field == scopeField {
			f.acceptBranch()
		}
		return f, nil, f.focusField((f.field + 1) % (detailsField + 1))
	case "shift+tab":
		return f, nil, f.focusField((f.field + detailsField) % (detailsField + 1))
	case "down":
		if f.branchScope() && f.field == scopeField {
			if count := len(f.matchingBranches()); count > 0 {
				f.branchCursor = (f.branchCursor + 1) % count
			}
			return f, nil, nil
		}
		if f.field == titleField && f.title.Line() < strings.Count(f.title.Value(), "\n") {
			break
		}
		if f.field < detailsField {
			return f, nil, f.focusField(f.field + 1)
		}
	case "enter":
		if f.field == scopeField {
			if f.branchScope() {
				f.acceptBranch()
			}
			return f, nil, f.focusField(detailsField)
		}
	case "up":
		if f.field == titleField && f.title.Line() == 0 {
			return f, nil, nil
		}
		if f.field == scopeField {
			if f.branchScope() {
				if count := len(f.matchingBranches()); count > 0 {
					f.branchCursor = (f.branchCursor - 1 + count) % count
				}
				return f, nil, nil
			}
			return f, nil, f.focusField(0)
		}
		if f.field == detailsField && f.details.Line() == 0 {
			return f, nil, f.focusField(f.field - 1)
		}
	}
	return f, nil, f.typeKey(msg)
}

// typeKey passes a key the form doesn't use itself to the focused field.
func (f *taskModal) typeKey(msg tea.KeyPressMsg) tea.Cmd {
	f.err = ""
	var cmd tea.Cmd
	switch f.field {
	case titleField:
		f.title, cmd = f.title.Update(msg)
	case scopeField:
		if !f.branchScope() {
			if msg.Code == tea.KeySpace {
				return nil
			}
			msg.Text = stripCategorySpaces(msg.Text)
		} else if f.branchFresh && msg.Text != "" {
			f.scope.SetValue("")
		}
		f.scope, cmd = f.scope.Update(msg)
		if f.branchScope() {
			f.branchFresh = false
			f.resetBranchCursor()
		}
	default:
		f.details, cmd = f.details.Update(msg)
	}
	return cmd
}

func (f *taskModal) paste(msg tea.PasteMsg) tea.Cmd {
	f.err = ""
	var cmd tea.Cmd
	switch f.field {
	case titleField:
		f.title, cmd = f.title.Update(msg)
	case scopeField:
		if !f.branchScope() {
			msg.Content = stripCategorySpaces(msg.Content)
		} else if f.branchFresh {
			f.scope.SetValue("")
		}
		f.scope, cmd = f.scope.Update(msg)
		if f.branchScope() {
			f.branchFresh = false
			f.resetBranchCursor()
		}
	default:
		f.details, cmd = f.details.Update(msg)
	}
	return cmd
}

// branchScope is true when the task is filed under a branch, so the scope
// field picks a local branch rather than taking a category.
func (f *taskModal) branchScope() bool {
	return f.mode == modalAddBranch || f.mode == modalEdit && f.selected.Branch != ""
}

func (f *taskModal) focusField(field int) tea.Cmd {
	f.field = field
	f.title.Blur()
	f.scope.Blur()
	f.details.Blur()
	if field == titleField {
		return f.title.Focus()
	}
	if field == scopeField {
		if f.branchScope() {
			f.branchFresh = true
		}
		return f.scope.Focus()
	}
	return f.details.Focus()
}

func (f *taskModal) save() error {
	if f.branchScope() {
		f.target.Branch = f.chosenBranch()
		if f.target.Branch == "" && f.mode == modalEdit && strings.TrimSpace(f.scope.Value()) == f.selected.Branch {
			f.target.Branch = f.selected.Branch
		}
		if f.target.Branch == "" {
			if len(f.branches) == 0 {
				return errors.New("no local Git branches found")
			}
			return errors.New("choose an existing local Git branch")
		}
		// The task's own branch was checked when the form opened.
		if f.target.Branch != f.selected.Branch && !f.project.hasLocalBranch(f.target.Branch) {
			return fmt.Errorf("branch %q no longer exists locally", f.target.Branch)
		}
	} else {
		f.target.Category = store.NormalizeCategory(f.scope.Value())
	}
	if f.mode == modalEdit {
		return store.Edit(f.file, f.selected, f.taskTitle(), f.details.Value(), f.target)
	}
	return store.Add(f.file, f.taskTitle(), f.details.Value(), store.PriorityNone, f.target)
}

// saved describes the task save wrote, for the model to follow.
func (f *taskModal) saved() taskSavedMsg {
	task := store.Task{Text: f.taskTitle(), Section: f.target, Line: f.selected.Line}
	moved := f.mode == modalEdit && !f.target.Same(f.selected.Section)
	return taskSavedMsg{task: task, added: f.mode != modalEdit, moved: moved}
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
		slices.SortStableFunc(matches, func(a, b string) int { return cmp.Compare(rank(a), rank(b)) })
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
		lines = append(lines, ui.MutedStyle.Render(ansi.Truncate(message, width, "…")))
	} else {
		start := max(0, f.branchCursor-rows+1)
		for i := start; i < len(matches) && len(lines) < rows; i++ {
			mark := "  "
			style := ui.MutedStyle
			if i == f.branchCursor {
				mark = "› "
				style = lipgloss.NewStyle().Foreground(ui.ColorGreen)
				if f.field == scopeField {
					style = style.Background(ui.ColorSelection)
				}
			}
			lineWidth := width
			if len(matches) > rows {
				lineWidth = max(1, width-2)
			}
			line := ansi.Truncate(mark+branchIcon+" "+matches[i], lineWidth, "…")
			if len(matches) > rows {
				line += strings.Repeat(" ", max(0, lineWidth-ansi.StringWidth(line)))
				thumb := min(rows-1, max(0, f.branchCursor)*rows/len(matches))
				bar := "│"
				if i-start == thumb {
					bar = "┃"
				}
				lines = append(lines, style.Render(line)+ui.MutedStyle.Render(" "+bar))
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
	if f.branchScope() {
		modalHeight = min(height, max(modalHeight, 13))
	}
	return min(width-2, 76), modalHeight
}

// isCompact drops spacer lines so add forms still fit short terminals.
func (f *taskModal) isCompact(height int) bool {
	return height < 15 || f.branchScope() && height < 18
}

func (f *taskModal) resize(width, height int) {
	modalWidth, modalHeight := f.dimensions(width, height)
	innerWidth := max(1, modalWidth-6)
	f.title.SetWidth(innerWidth)
	f.title.SetHeight(2)
	f.scope.SetWidth(innerWidth)
	f.details.SetWidth(innerWidth)
	// Details takes whatever height the rest of the layout leaves, measured
	// from the real content so the two can never drift apart.
	f.details.SetHeight(1)
	otherLines := lipgloss.Height(strings.Join(f.contentLines(modalWidth, modalHeight), "\n")) - 1
	f.details.SetHeight(max(2, modalHeight-2*modalBorder-otherLines))
}

const modalBorder = 1

func (f *taskModal) render(width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2, 0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(ui.ColorFocus).BorderBackground(ui.ColorModal).
		Background(ui.ColorModal).Render(strings.Join(f.contentLines(width, height), "\n"))
}

// contentLines lays out the form. Every line starts with a one-cell gutter
// that holds the focus bar beside the active field.
func (f *taskModal) contentLines(width, height int) []string {
	innerWidth := max(1, width-6)
	compact := f.isCompact(height)
	var lines []string
	add := func(focused bool, blocks ...string) {
		gutter := " "
		if focused {
			gutter = lipgloss.NewStyle().Foreground(ui.ColorFocus).Render("┃")
		}
		for _, block := range blocks {
			for line := range strings.SplitSeq(block, "\n") {
				lines = append(lines, ui.OnBackground(gutter+line, ui.ColorModal))
			}
		}
	}
	gap := func() { lines = append(lines, "") }

	add(false, ui.Breadcrumb(f.breadcrumb(), "", innerWidth))
	if !compact {
		gap()
	}
	add(f.field == titleField, ui.OnBackground(f.title.View(), ui.ColorField))
	if !f.branchScope() || !compact {
		gap()
	}
	scopeName := "Category"
	if f.branchScope() {
		scopeName = "Branch"
	}
	add(f.field == scopeField, f.label(scopeName, scopeField), ui.OnBackground(fieldStyle().Width(innerWidth).Render(f.scope.View()), ui.ColorField))
	if f.branchScope() {
		add(f.field == scopeField, f.branchSuggestions(innerWidth)...)
	}
	if !compact {
		gap()
	}
	add(f.field == detailsField, f.label("Details", detailsField), ui.OnBackground(f.details.View(), ui.ColorField))
	if !compact {
		gap()
	}
	add(false, f.footer(innerWidth))
	return lines
}

func (f *taskModal) breadcrumb() []string {
	switch {
	case f.mode == modalAddGeneral:
		return []string{"General", "New task"}
	case f.mode == modalAddBranch:
		return []string{"Branches", "New task"}
	case f.selected.Branch != "":
		return []string{"Branches", branchIcon + " " + f.selected.Branch, "Edit task"}
	case f.selected.Category != "":
		return []string{"General", "@" + f.selected.Category, "Edit task"}
	}
	return []string{"General", "Edit task"}
}

func (f *taskModal) label(name string, field int) string {
	if f.field == field {
		return ui.TitleStyle.Render(name)
	}
	return ui.MutedStyle.Render(name)
}

func (f *taskModal) footer(width int) string {
	if f.err != "" {
		return errorText(f.err, width)
	}
	hints := []ui.KeyHint{{Key: "ctrl+enter", Label: "save"}, {Key: "esc", Label: "cancel"}, {Key: "tab", Label: "next field"}}
	if f.branchScope() && f.field == scopeField {
		hints = []ui.KeyHint{{Key: "ctrl+enter", Label: "save"}, {Key: "esc", Label: "cancel"}, {Key: "↑↓", Label: "choose"}, {Key: "tab", Label: "accept"}}
	}
	return ui.FitHints(hints, width)
}

// errorText shows a failed save's error where the key hints would be.
func errorText(err string, width int) string {
	return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorHigh).Render(ansi.Truncate(err, width, "…"))
}

func fieldStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(ui.ColorField)
}

// fieldAreaStyles fills a textarea with the field colour, without the default
// cursor-line highlight, so every field reads as one block.
func fieldAreaStyles() textarea.Styles {
	styles := textarea.DefaultStyles(true)
	state := textarea.StyleState{
		Base:        fieldStyle(),
		Text:        fieldStyle().Foreground(ui.ColorStrong),
		CursorLine:  fieldStyle().Foreground(ui.ColorStrong),
		Placeholder: fieldStyle().Foreground(ui.ColorMuted),
		EndOfBuffer: fieldStyle().Foreground(ui.ColorField),
		Prompt:      fieldStyle(),
		Selection:   fieldStyle().Background(ui.ColorSelection),
	}
	styles.Focused = state
	styles.Blurred = state
	styles.Blurred.Text = fieldStyle().Foreground(ui.ColorText)
	styles.Blurred.CursorLine = styles.Blurred.Text
	styles.Cursor.Color = ui.ColorFocus
	return styles
}

func fieldInputStyles() textinput.Styles {
	styles := textinput.DefaultStyles(true)
	state := textinput.StyleState{
		Text:        fieldStyle().Foreground(ui.ColorStrong),
		Placeholder: fieldStyle().Foreground(ui.ColorMuted),
		Suggestion:  fieldStyle().Foreground(ui.ColorMuted),
		Prompt:      fieldStyle(),
	}
	styles.Focused = state
	styles.Blurred = state
	styles.Blurred.Text = fieldStyle().Foreground(ui.ColorText)
	styles.Cursor.Color = ui.ColorFocus
	return styles
}
