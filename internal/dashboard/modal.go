package dashboard

import (
	"cmp"
	"errors"
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
	modalAddSubtask
	modalEdit
)

type taskModal struct {
	mode         modalMode
	selected     store.Task
	parent       store.Task // the task a subtask's form adds under, or edits under
	file         string
	target       store.Section
	onCurrent    bool // target is the current Git branch, which Git's answer may update
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

// startTaskModal opens the task form. A branch form starts with the branches
// Git last listed, and takes a fresh list when Git answers in the background.
func (m *model) startTaskModal(mode modalMode) tea.Cmd {
	modal := &taskModal{mode: mode, file: m.file}
	var check tea.Cmd
	if mode == modalAddGeneral && m.all == nil && m.activePane() == generalPane && m.general.open.kind == rowCategory {
		modal.target.Category = m.general.open.name
	}
	if mode == modalAddBranch {
		modal.branches, modal.target.Branch, check = m.localBranches, m.project.Branch, m.checkBranches()
		modal.onCurrent = true
		if m.all == nil && m.activePane() == branchPane && m.branch.open != (group{}) {
			modal.target.Branch, modal.onCurrent = m.branch.open.name, false
		}
	}
	if mode == modalEdit {
		selected, ok := m.selectedTask()
		if !ok {
			m.status = "Select a Markdown task to edit"
			return nil
		}
		if selected.Branch != "" && !selected.Subtask {
			modal.branches, check = m.localBranches, m.checkBranches()
		}
		modal.selected = selected
		modal.target = selected.Section
	}
	if mode == modalAddSubtask || modal.selected.Subtask {
		parent, ok := m.selectedParent()
		if !ok {
			m.status = "Open a Markdown task's details to add a subtask"
			return nil
		}
		modal.parent, modal.target = parent, parent.Section
	}
	modal.title = textarea.New()
	modal.scope = textinput.New()
	modal.details = textarea.New()
	modal.setTheme(m.theme)
	modal.title.Prompt = ""
	modal.title.ShowLineNumbers = false
	modal.title.Placeholder = "What needs doing?"
	modal.title.SetHeight(2)
	modal.scope.Prompt = ""
	if modal.branchScope() {
		modal.scope.Placeholder = "Search local branches, or name a new one"
		modal.scope.SetValue(modal.target.Branch)
		modal.branchCursor = slices.Index(modal.branchChoices(), modal.target.Branch)
	} else {
		modal.scope.Placeholder = "Optional category"
		modal.scope.SetValue(modal.target.Category)
		suggestCategories(&modal.scope, m.categoryNames())
	}
	modal.details.Prompt = ""
	modal.details.ShowLineNumbers = false
	modal.details.Placeholder = "Add context, steps, or links…"
	if mode == modalEdit {
		modal.title.SetValue(modal.selected.Text)
		modal.details.SetValue(modal.selected.Details)
	}
	modal.resize(m.theme, m.width, m.height)
	m.overlay = modal
	m.status = ""
	return tea.Batch(modal.title.Focus(), check)
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
	var changed []store.Task
	if !msg.added {
		changed = append(changed, msg.task)
	}
	if err := m.refresh(changed...); err != nil {
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
	case tea.PasteMsg:
		return f, nil, f.paste(msg)
	case tea.KeyPressMsg:
		return f.key(msg)
	}
	return f, nil, nil
}

func (f *taskModal) view(theme ui.Theme, width, height int) string {
	width, height = f.dimensions(width, height)
	return f.render(theme, width, height)
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
		switch {
		case f.field != scopeField:
		case f.branchScope():
			f.acceptBranch()
		case completeCategory(&f.scope):
			return f, nil, nil
		}
		return f, nil, f.focusField((f.field + 1) % (f.lastField() + 1))
	case "shift+tab":
		return f, nil, f.focusField((f.field + f.lastField()) % (f.lastField() + 1))
	case "down":
		if f.branchScope() && f.field == scopeField {
			if count := len(f.branchChoices()); count > 0 {
				f.branchCursor = (f.branchCursor + 1) % count
			}
			return f, nil, nil
		}
		if f.field == titleField && f.title.Line() < strings.Count(f.title.Value(), "\n") {
			break
		}
		if f.field < f.lastField() {
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
				if count := len(f.branchChoices()); count > 0 {
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
		if f.branchScope() && f.branchFresh && msg.Text != "" {
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
		if f.branchScope() && f.branchFresh {
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
	return !f.subtaskForm() && (f.mode == modalAddBranch || f.mode == modalEdit && f.selected.Branch != "")
}

// subtaskForm is true for a subtask's form, which has only the Task field: a
// subtask keeps its task's category or branch, and has no details to edit.
func (f *taskModal) subtaskForm() bool {
	return f.mode == modalAddSubtask || f.mode == modalEdit && f.selected.Subtask
}

// lastField is the form's last field: Details, or Task in a subtask's form.
func (f *taskModal) lastField() int {
	if f.subtaskForm() {
		return titleField
	}
	return detailsField
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
	switch {
	case f.mode == modalAddSubtask:
		return store.AddSubtask(f.file, f.parent, f.taskTitle())
	case f.subtaskForm():
		return store.Edit(f.file, f.selected, f.taskTitle(), f.selected.Details, f.selected.Section)
	}
	if f.branchScope() {
		f.target.Branch = f.chosenBranch()
		if f.target.Branch == "" {
			return errors.New("choose or type a branch")
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
	text, p := store.SplitPriority(f.taskTitle())
	if p == store.PriorityNone {
		p = f.selected.Priority
	}
	task := store.Task{Text: text, Section: f.target, Line: f.selected.Line, Done: f.selected.Done, Priority: p, Subtask: f.subtaskForm()}
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

// branchChoices are the local branches matching what's typed, then the typed
// name itself when Git has no branch of that name, as the task may be for a
// branch not created yet.
func (f *taskModal) branchChoices() []string {
	choices := f.matchingBranches()
	if name := strings.TrimSpace(f.scope.Value()); name != "" && !slices.Contains(f.branches, name) {
		choices = append(choices, name)
	}
	return choices
}

// setBranches takes a fresh list of local branches and the current branch,
// keeping the highlighted suggestion while Git still has it. A form still
// showing the current branch it opened with moves to the new current branch.
func (f *taskModal) setBranches(branches []string, current string) {
	highlighted := ""
	if choices := f.branchChoices(); f.branchCursor >= 0 && f.branchCursor < len(choices) {
		highlighted = choices[f.branchCursor]
	}
	if f.onCurrent && f.scope.Value() == f.target.Branch {
		f.target.Branch = current
		f.scope.SetValue(current)
		highlighted = current
	}
	f.branches = branches
	f.branchCursor = slices.Index(f.branchChoices(), highlighted)
}

func (f *taskModal) resetBranchCursor() {
	if strings.TrimSpace(f.scope.Value()) == "" {
		f.branchCursor = -1
	} else {
		f.branchCursor = 0
	}
}

// chosenBranch is the highlighted choice, or "" when there's none, such as
// when Git no longer lists the one that was highlighted.
func (f *taskModal) chosenBranch() string {
	if choices := f.branchChoices(); f.branchCursor >= 0 && f.branchCursor < len(choices) {
		return choices[f.branchCursor]
	}
	return ""
}

// acceptBranch fills the field with the chosen branch, keeping it
// highlighted even where a local branch's name contains it.
func (f *taskModal) acceptBranch() {
	if branch := f.chosenBranch(); branch != "" {
		f.scope.SetValue(branch)
		f.branchCursor = slices.Index(f.branchChoices(), branch)
		f.branchFresh = true
	}
}

func (f *taskModal) branchSuggestions(theme ui.Theme, width int) []string {
	rows := 2
	choices := f.branchChoices()
	lines := make([]string, 0, rows)
	if len(choices) == 0 {
		lines = append(lines, theme.MutedStyle.Render(ansi.Truncate("No local Git branches", width, "…")))
	} else {
		start := max(0, f.branchCursor-rows+1)
		for i := start; i < len(choices) && len(lines) < rows; i++ {
			known := slices.Contains(f.branches, choices[i])
			mark, label := "  ", branchIcon+" "+choices[i]
			if !known {
				label = "⚠ " + choices[i] + " not in Git"
			}
			style := theme.MutedStyle
			if i == f.branchCursor {
				mark = "› "
				style = lipgloss.NewStyle().Foreground(theme.ColorGreen)
				if !known {
					style = style.Foreground(theme.ColorHigh)
				}
				if f.field == scopeField {
					style = style.Background(theme.ColorSelection)
				}
			}
			lineWidth := width
			if len(choices) > rows {
				lineWidth = max(1, width-2)
			}
			line := ansi.Truncate(mark+label, lineWidth, "…")
			if len(choices) > rows {
				line += strings.Repeat(" ", max(0, lineWidth-ansi.StringWidth(line)))
				thumb := min(rows-1, max(0, f.branchCursor)*rows/len(choices))
				bar := "│"
				if i-start == thumb {
					bar = "┃"
				}
				lines = append(lines, style.Render(line)+theme.MutedStyle.Render(" "+bar))
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
	switch {
	case f.subtaskForm():
		modalHeight = min(height, subtaskFormHeight)
	case f.branchScope():
		modalHeight = min(height, max(modalHeight, 13))
	}
	return min(width-2, 76), modalHeight
}

// subtaskFormHeight fits a subtask's form, with its spacer lines, in its
// border.
const subtaskFormHeight = 8

// isCompact drops spacer lines so add forms still fit short terminals.
func (f *taskModal) isCompact(height int) bool {
	if f.subtaskForm() {
		return height < subtaskFormHeight
	}
	return height < 15 || f.branchScope() && height < 18
}

func (f *taskModal) resize(theme ui.Theme, width, height int) {
	modalWidth, modalHeight := f.dimensions(width, height)
	innerWidth := max(1, modalWidth-6)
	f.title.SetWidth(innerWidth)
	f.title.SetHeight(2)
	f.scope.SetWidth(innerWidth)
	f.details.SetWidth(innerWidth)
	// Details takes whatever height the rest of the layout leaves, measured
	// from the real content so the two can never drift apart.
	f.details.SetHeight(1)
	otherLines := lipgloss.Height(strings.Join(f.contentLines(theme, modalWidth, modalHeight), "\n")) - 1
	f.details.SetHeight(max(2, modalHeight-2*modalBorder-otherLines))
}

const modalBorder = 1

func (f *taskModal) render(theme ui.Theme, width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Padding(0, 2, 0, 1).
		Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorFocus).BorderBackground(theme.ColorModal).
		Background(theme.ColorModal).Render(strings.Join(f.contentLines(theme, width, height), "\n"))
}

// contentLines lays out the form. Every line starts with a one-cell gutter
// that holds the focus bar beside the active field.
func (f *taskModal) contentLines(theme ui.Theme, width, height int) []string {
	innerWidth := max(1, width-6)
	compact := f.isCompact(height)
	var lines []string
	add := func(focused bool, blocks ...string) {
		gutter := " "
		if focused {
			gutter = lipgloss.NewStyle().Foreground(theme.ColorFocus).Render("┃")
		}
		for _, block := range blocks {
			for line := range strings.SplitSeq(block, "\n") {
				lines = append(lines, ui.OnBackground(gutter+line, theme.ColorModal))
			}
		}
	}
	gap := func() { lines = append(lines, "") }

	add(false, theme.Breadcrumb(f.breadcrumb(), "", innerWidth))
	if !compact {
		gap()
	}
	add(f.field == titleField, ui.OnBackground(f.title.View(), theme.ColorField))
	if f.subtaskForm() {
		if !compact {
			gap()
		}
		add(false, f.footer(theme, innerWidth))
		return lines
	}
	if !f.branchScope() || !compact {
		gap()
	}
	scopeName := "Category"
	if f.branchScope() {
		scopeName = "Branch"
	}
	add(f.field == scopeField, f.label(theme, scopeName, scopeField), ui.OnBackground(fieldStyle(theme).Width(innerWidth).Render(f.scope.View()), theme.ColorField))
	if f.branchScope() {
		add(f.field == scopeField, f.branchSuggestions(theme, innerWidth)...)
	}
	if !compact {
		gap()
	}
	add(f.field == detailsField, f.label(theme, "Details", detailsField), ui.OnBackground(f.details.View(), theme.ColorField))
	if !compact {
		gap()
	}
	add(false, f.footer(theme, innerWidth))
	return lines
}

// breadcrumb names what the form adds or edits, and where: a subtask's form
// names its task too.
func (f *taskModal) breadcrumb() []string {
	switch f.mode {
	case modalAddGeneral:
		return []string{"General", "New task"}
	case modalAddBranch:
		return []string{"Branches", "New task"}
	case modalAddSubtask:
		return append(sectionCrumbs(f.parent.Section), ui.CleanDisplay(f.parent.Text), "New subtask")
	}
	if f.subtaskForm() {
		return append(sectionCrumbs(f.selected.Section), ui.CleanDisplay(f.parent.Text), "Edit subtask")
	}
	return append(sectionCrumbs(f.selected.Section), "Edit task")
}

// sectionCrumbs names a section as the dashboard's breadcrumbs do.
func sectionCrumbs(s store.Section) []string {
	switch {
	case s.Branch != "":
		return []string{"Branches", branchIcon + " " + s.Branch}
	case s.Category != "":
		return []string{"General", "@" + s.Category}
	}
	return []string{"General"}
}

func (f *taskModal) label(theme ui.Theme, name string, field int) string {
	if f.field == field {
		return theme.TitleStyle.Render(name)
	}
	return theme.MutedStyle.Render(name)
}

func (f *taskModal) footer(theme ui.Theme, width int) string {
	if f.err != "" {
		return errorText(theme, f.err, width)
	}
	hints := []ui.KeyHint{{Key: "ctrl+enter", Label: "save"}, {Key: "esc", Label: "cancel"}, {Key: "tab", Label: "next field"}}
	if f.subtaskForm() {
		hints = hints[:2]
	}
	if f.field == scopeField && canCompleteCategory(f.scope) {
		hints = []ui.KeyHint{completeHint, {Key: "ctrl+enter", Label: "save"}, {Key: "esc", Label: "cancel"}}
	}
	if f.branchScope() && f.field == scopeField {
		hints = []ui.KeyHint{{Key: "ctrl+enter", Label: "save"}, {Key: "esc", Label: "cancel"}, {Key: "↑↓", Label: "choose"}, {Key: "tab", Label: "accept"}}
	}
	return theme.FitHints(hints, width)
}

// errorText shows a failed save's error where the key hints would be.
func errorText(theme ui.Theme, err string, width int) string {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.ColorHigh).Render(ansi.Truncate(err, width, "…"))
}

// setTheme restyles the form's fields for theme.
func (f *taskModal) setTheme(theme ui.Theme) {
	f.title.SetStyles(fieldAreaStyles(theme))
	f.scope.SetStyles(fieldInputStyles(theme))
	f.details.SetStyles(fieldAreaStyles(theme))
}

func fieldStyle(theme ui.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Background(theme.ColorField)
}

// fieldAreaStyles fills a textarea with the field colour, without the default
// cursor-line highlight, so every field reads as one block.
func fieldAreaStyles(theme ui.Theme) textarea.Styles {
	styles := textarea.DefaultStyles(true)
	state := textarea.StyleState{
		Base:        fieldStyle(theme),
		Text:        fieldStyle(theme).Foreground(theme.ColorStrong),
		CursorLine:  fieldStyle(theme).Foreground(theme.ColorStrong),
		Placeholder: fieldStyle(theme).Foreground(theme.ColorMuted),
		EndOfBuffer: fieldStyle(theme).Foreground(theme.ColorField),
		Prompt:      fieldStyle(theme),
		Selection:   fieldStyle(theme).Background(theme.ColorSelection),
	}
	styles.Focused = state
	styles.Blurred = state
	styles.Blurred.Text = fieldStyle(theme).Foreground(theme.ColorText)
	styles.Blurred.CursorLine = styles.Blurred.Text
	styles.Cursor.Color = theme.ColorFocus
	return styles
}

func fieldInputStyles(theme ui.Theme) textinput.Styles {
	styles := textinput.DefaultStyles(true)
	state := textinput.StyleState{
		Text:        fieldStyle(theme).Foreground(theme.ColorStrong),
		Placeholder: fieldStyle(theme).Foreground(theme.ColorMuted),
		Suggestion:  fieldStyle(theme).Foreground(theme.ColorMuted),
		Prompt:      fieldStyle(theme),
	}
	styles.Focused = state
	styles.Blurred = state
	styles.Blurred.Text = fieldStyle(theme).Foreground(theme.ColorText)
	styles.Cursor.Color = theme.ColorFocus
	return styles
}
