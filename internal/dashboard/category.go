package dashboard

import (
	"strings"
	"unicode"

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
	p.input.SetValue(selected.Category)
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
	if err := m.refresh(); err != nil {
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
		msg.Content = stripCategorySpaces(msg.Content)
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
	case "enter":
		if err := store.SetCategory(p.file, p.task, p.input.Value()); err != nil {
			p.err = errorStatus(err)
			return p, nil, nil
		}
		moved := p.task
		moved.Category = store.NormalizeCategory(p.input.Value())
		return nil, categorySetMsg{moved}, nil
	}
	if msg.Code == tea.KeySpace {
		return p, nil, nil
	}
	p.err = ""
	msg.Text = stripCategorySpaces(msg.Text)
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, nil, cmd
}

// footer takes the dashboard's footer while the prompt is open, with a
// failed save's error in place of the key hints.
func (p *categoryPrompt) footer(width int) string {
	input := p.input.View()
	rest := width - ansi.StringWidth(input) - 2
	if p.err != "" {
		return input + "  " + errorText(p.err, rest)
	}
	return input + "  " + ui.FitHints([]ui.KeyHint{{Key: "enter", Label: "save"}, {Key: "esc", Label: "cancel"}}, rest)
}

func stripCategorySpaces(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
}
