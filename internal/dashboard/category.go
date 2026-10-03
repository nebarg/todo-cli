package dashboard

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// categoryInputWidth fits a category name and leaves the rest of the
// footer for the key hints or an error.
const categoryInputWidth = 32

// categoryPrompt edits a general task's category in the footer.
type categoryPrompt struct {
	file  string
	task  store.Task
	input textinput.Model
	err   string
}

func (m *model) startCategoryInput() tea.Cmd {
	if m.readmeSelected() {
		m.status = readmeReadOnly
		return nil
	}
	selected, ok := m.selectedTask()
	if !ok {
		m.status = "Select a Markdown task to edit its category"
		return nil
	}
	if selected.Branch != "" {
		m.status = "Categories are only for general tasks"
		return nil
	}
	p := &categoryPrompt{file: m.file, task: selected, input: textinput.New()}
	p.input.Prompt = "Category: "
	p.input.SetWidth(categoryInputWidth)
	styles := p.input.Styles()
	styles.Focused.Suggestion = m.theme.MutedStyle
	p.input.SetStyles(styles)
	p.input.SetValue(selected.Category)
	suggestCategories(&p.input, m.categoryNames())
	m.overlay = p
	m.status = ""
	return p.input.Focus()
}

// categorySetMsg is a task the prompt filed under a new category, as it now
// is, with its line before the move.
type categorySetMsg struct{ task store.Task }

// categorySet follows a recategorised task: to its new category in General,
// or to its new place in the All tasks view.
func (m *model) categorySet(msg categorySetMsg) (tea.Model, tea.Cmd) {
	if err := m.refresh(msg.task); err != nil {
		m.status = err.Error()
		return m, nil
	}
	switch {
	case m.all != nil:
		m.selectInAllTasks(msg.task)
	case m.activePane() == generalPane:
		m.reveal(msg.task)
	}
	m.status = ""
	return m, nil
}

func (p *categoryPrompt) update(msg tea.Msg) (overlay, tea.Msg, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.PasteMsg:
		p.input, cmd = p.input.Update(msg)
	case tea.KeyPressMsg:
		return p.key(msg)
	}
	return p, nil, cmd
}

func (p *categoryPrompt) key(msg tea.KeyPressMsg) (overlay, tea.Msg, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return nil, nil, nil
	case "tab":
		completeCategory(&p.input)
		return p, nil, nil
	case "enter":
		if err := store.SetCategory(p.file, p.task, p.input.Value()); err != nil {
			p.err = errorStatus(err)
			return p, nil, nil
		}
		moved := p.task
		moved.Category = store.NormalizeCategory(p.input.Value())
		return nil, categorySetMsg{moved}, nil
	}
	p.err = ""
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, nil, cmd
}

// footer takes the dashboard's footer while the prompt is open, with a
// failed save's error in place of the key hints.
func (p *categoryPrompt) footer(theme ui.Theme, width int) string {
	input := p.input.View()
	rest := width - ansi.StringWidth(input) - 2
	if p.err != "" {
		return input + "  " + errorText(theme, p.err, rest)
	}
	hints := []ui.KeyHint{{Key: "enter", Label: "save"}, {Key: "esc", Label: "cancel"}}
	if canCompleteCategory(p.input) {
		hints = append([]ui.KeyHint{completeHint}, hints...)
	}
	return input + "  " + theme.FitHints(hints, rest)
}

// completeHint offers tab while a category input shows a completion.
var completeHint = ui.KeyHint{Key: "tab", Label: "complete"}

// categoryNames lists General's categories, in the order its rows show them.
func (m *model) categoryNames() []string {
	var names []string
	for _, row := range generalRows(m.tasks.general, nil, group{}) {
		if row.kind == rowCategory {
			names = append(names, row.name)
		}
	}
	return names
}

// suggestCategories has input show the rest of the first category that
// starts with what's typed, in grey after the cursor. ctrl+n and ctrl+p
// switch between the categories that match.
func suggestCategories(input *textinput.Model, categories []string) {
	input.ShowSuggestions = true
	input.SetSuggestions(categories)
}

// canCompleteCategory is true while input shows more of a category than is
// typed.
func canCompleteCategory(input textinput.Model) bool {
	suggestion := input.CurrentSuggestion()
	return suggestion != "" && !strings.EqualFold(suggestion, input.Value())
}

// completeCategory fills input with the category it suggests, written as the
// category is rather than in the case typed, and reports whether there was
// one to complete.
func completeCategory(input *textinput.Model) bool {
	if !canCompleteCategory(*input) {
		return false
	}
	input.SetValue(input.CurrentSuggestion())
	input.CursorEnd()
	return true
}
