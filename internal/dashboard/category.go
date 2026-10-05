package dashboard

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// minCategoryInputWidth keeps room to type in a narrow terminal, where the
// keys beside the input give way first.
const minCategoryInputWidth = 24

// promptKeys are the category prompt's keys, after tab while it shows a
// suggestion.
var promptKeys = []ui.KeyHint{ui.Hint(formKeys.Enter, "save"), ui.Hint(formKeys.Cancel, "cancel")}

// categoryPrompt edits a general task's category in the footer.
type categoryPrompt struct {
	file  string
	task  store.Task
	input textinput.Model
	err   error
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
	if selected.Subtask {
		m.status = subtaskStaysPut
		return nil
	}
	if selected.Branch != "" {
		m.status = "Categories are only for general tasks"
		return nil
	}
	p := &categoryPrompt{file: m.file, task: selected, input: textinput.New()}
	p.input.Prompt = "Category: "
	p.resize(m.theme, m.width)
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
	switch {
	case key.Matches(msg, formKeys.Cancel):
		return nil, nil, nil
	case key.Matches(msg, formKeys.Tab):
		completeCategory(&p.input)
		return p, nil, nil
	case key.Matches(msg, formKeys.Enter):
		if err := store.SetCategory(p.file, p.task, p.input.Value()); err != nil {
			p.err = err
			return p, nil, nil
		}
		moved := p.task
		moved.Category = store.NormalizeCategory(p.input.Value())
		return nil, categorySetMsg{moved}, nil
	}
	p.err = nil
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, nil, cmd
}

// resize gives the input the footer's width less its keys, leaving room
// for tab, so the keys sit at the right edge.
func (p *categoryPrompt) resize(theme ui.Theme, width int) {
	hints := ansi.StringWidth(theme.RenderHints(append([]ui.KeyHint{completeHint}, promptKeys...)))
	p.input.SetWidth(max(minCategoryInputWidth, width-ansi.StringWidth(p.input.Prompt)-hints-3))
}

// footer takes the dashboard's footer while the prompt is open: the input,
// and at the right edge its keys, or a failed save's error. What's typed
// always shows, and an error may take the input's room after it.
func (p *categoryPrompt) footer(theme ui.Theme, width int) string {
	prompt := ansi.StringWidth(p.input.Prompt)
	// The input's view ends in the cursor's cell after the text.
	typed := prompt + min(p.input.Width(), ansi.StringWidth(p.input.Value())) + 1
	room := max(0, width-typed-2)
	right := theme.FitHints(p.keys(), room)
	if p.err != nil {
		right = errorText(theme, p.err, room)
	}
	// The input pads to its width without counting a suggestion's grey text,
	// so it's cut back to that width, or to what the right side leaves.
	input := ansi.Truncate(p.input.View(), min(prompt+p.input.Width()+1, width-ansi.StringWidth(right)-2), "")
	return input + strings.Repeat(" ", max(0, width-ansi.StringWidth(input)-ansi.StringWidth(right))) + right
}

// keys are the prompt's keys, led by tab while it shows a suggestion.
func (p *categoryPrompt) keys() []ui.KeyHint {
	if canCompleteCategory(p.input) {
		return append([]ui.KeyHint{completeHint}, promptKeys...)
	}
	return promptKeys
}

// completeHint offers tab while a category input shows a completion.
var completeHint = ui.Hint(formKeys.Tab, "complete")

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
	// With nothing matching, up and ctrl+p leave the input's suggestion index
	// at -1, which CurrentSuggestion doesn't guard against.
	if len(input.MatchedSuggestions()) == 0 {
		return false
	}
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
