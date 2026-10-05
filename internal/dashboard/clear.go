package dashboard

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// clearConfirmation is a planned clear waiting for the user to confirm it.
type clearConfirmation struct {
	removal store.Removal
	targets store.ClearTargets
	scope   string
}

// clearScope is the tasks X clears from where the user is, and a name for
// them: an opened category or branch, the current tab, or every task.
func (m *model) clearScope() ([]store.Task, string) {
	if m.all != nil {
		return m.tasks.all, "all tasks"
	}
	switch m.activePane() {
	case generalPane:
		switch {
		case m.general.open.inReadme():
			return nil, readmeGroup
		case m.general.open.kind == rowCategory:
			return rowTasks(m.rows(generalPane)), "@" + m.general.open.name
		}
		return m.tasks.general, "General"
	case branchPane:
		if m.branch.open == (group{}) {
			return m.tasks.branches, "Branches"
		}
		return rowTasks(m.rows(branchPane)), branchIcon + " " + m.branch.open.name
	}
	return nil, ""
}

// clearHint offers X only when there is something for it to clear, counting
// the done tasks and subtasks it would remove on their own.
func (m *model) clearHint() []ui.KeyHint {
	tasks, _ := m.clearScope()
	if done := len(doneTargets(tasks).Tasks); done > 0 {
		return []ui.KeyHint{{Key: "X", Label: fmt.Sprintf("clear %d done", done)}}
	}
	return nil
}

// doneTargets are the done tasks of tasks. Tasks of a branch Git doesn't
// have stay, as the branch may not be created yet.
func doneTargets(tasks []store.Task) store.ClearTargets {
	return store.PickClearTargets(tasks, func(string) bool { return false })
}

func (m *model) startClearDone() {
	if m.activePane() == sourcePane && m.all == nil {
		m.status = "File TODOs are read only"
		return
	}
	if m.readmeSelected() {
		m.status = readmeReadOnly
		return
	}
	tasks, scope := m.clearScope()
	targets := doneTargets(tasks)
	if len(targets.Tasks) == 0 {
		m.status = "No done tasks to clear"
		return
	}
	removal, err := store.PlanRemove(m.file, targets.Tasks)
	if err != nil {
		m.status = errorStatus(err)
		return
	}
	m.overlay = &clearConfirmation{removal: removal, targets: targets, scope: scope}
	m.status = ""
}

// removalConfirmedMsg is a clear or delete the user confirmed, with the
// status that reports it.
type removalConfirmedMsg struct {
	removal store.Removal
	status  string
}

func (c *clearConfirmation) update(msg tea.Msg) (overlay, tea.Msg, tea.Cmd) {
	return confirmOnY(c, msg, removalConfirmedMsg{c.removal, c.doneStatus()})
}

func (c *clearConfirmation) view(theme ui.Theme, _, _ int) string { return c.render(theme) }

// doneStatus reports the clear once it is applied.
func (c *clearConfirmation) doneStatus() string {
	return "Removed " + ui.ClearSummary(c.targets)
}

// confirmOnY closes o with confirmed as its outcome on y; any other key
// cancels, so a stray key never deletes anything.
func confirmOnY(o overlay, msg tea.Msg, confirmed removalConfirmedMsg) (overlay, tea.Msg, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	switch {
	case !ok:
		return o, nil, nil
	case key.String() == "y":
		return nil, confirmed, nil
	}
	return nil, nil, nil
}

// removalUndo is a clear or delete u can undo, with the order the tasks
// were shown in before it, which the undo puts back.
type removalUndo struct {
	removal store.Removal
	order   []store.Task
}

// applyRemoval removes a confirmed clear's or delete's tasks, keeping the
// removal so u can undo it.
func (m *model) applyRemoval(msg removalConfirmedMsg) (tea.Model, tea.Cmd) {
	if err := msg.removal.Apply(); err != nil {
		m.status = errorStatus(err)
		return m, nil
	}
	if m.focus == detailPane {
		m.focus = m.detailFrom
	}
	order := m.tasks.all
	// Rows keep their place by matching each task to one with the same text
	// nearest its old line. Without the removed tasks, an identical task left
	// can't take one's place.
	if err := m.refreshFrom(withoutTasks(order, msg.removal.Tasks)); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.lastRemoval = &removalUndo{removal: msg.removal, order: order}
	m.status = msg.status
	return m, nil
}

// withoutTasks is tasks less those in removed, both from the same read of
// the file, where a line holds one task.
func withoutTasks(tasks, removed []store.Task) []store.Task {
	return slices.DeleteFunc(slices.Clone(tasks), func(t store.Task) bool {
		return slices.ContainsFunc(removed, func(r store.Task) bool { return r.Line == t.Line })
	})
}

func (m *model) undoRemoval() {
	if m.lastRemoval == nil {
		m.status = "Nothing to undo"
		return
	}
	undo := *m.lastRemoval
	m.lastRemoval = nil
	if err := undo.removal.Undo(); err != nil {
		if errors.Is(err, store.ErrFileChanged) {
			m.status = "The file has changed since, so it cannot be undone"
		} else {
			m.status = err.Error()
		}
		return
	}
	// The file is as it was before the removal, so its order fits exactly.
	if err := m.refreshFrom(undo.order); err != nil {
		m.status = err.Error()
		return
	}
	m.status = "Restored " + taskCount(len(undo.removal.Tasks))
}

func taskCount(n int) string { return ui.Plural(n, "task", "tasks") }

func (c clearConfirmation) render(theme ui.Theme) string {
	title := fmt.Sprintf("Remove %s from %s?", ui.ClearSummary(c.targets), c.scope)
	return renderConfirmation(theme, title, []string{ui.SubtaskNote(c.targets), emptiedGroups(c.removal)}, "remove")
}

// renderConfirmation draws a dialog asking to go ahead with action, with
// any non-empty notes under the title.
func renderConfirmation(theme ui.Theme, title string, notes []string, action string) string {
	const width = 48
	lines := []string{theme.TitleStyle.Render(ansi.Wrap(title, width, ""))}
	for _, note := range notes {
		if note != "" {
			lines = append(lines, theme.MutedStyle.Render(ansi.Wrap(note, width, "")))
		}
	}
	lines = append(lines, "", theme.RenderHints([]ui.KeyHint{{Key: "y", Label: action}, {Key: "esc", Label: "cancel"}}))
	return lipgloss.NewStyle().Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorHigh).BorderBackground(theme.ColorModal).
		Background(theme.ColorModal).Render(ui.OnBackground(strings.Join(lines, "\n"), theme.ColorModal))
}

// emptiedGroups says which categories and branches removal leaves with
// nothing in them, which go with it, or is "" when there are none.
func emptiedGroups(removal store.Removal) string {
	var groups []string
	if n := len(removal.Categories); n > 0 {
		groups = append(groups, "the empty "+quotedNames(removal.Categories)+" "+ui.PluralWord(n, "category", "categories"))
	}
	if n := len(removal.Branches); n > 0 {
		groups = append(groups, "the empty "+quotedNames(removal.Branches)+" "+ui.PluralWord(n, "branch", "branches"))
	}
	if len(groups) == 0 {
		return ""
	}
	return "T" + ui.JoinNames(groups)[1:] + " will be removed too."
}

// quotedNames lists names in quotes, as "a", "a" and "b" or "a", "b" and "c",
// since a category's own name may have "and" in it.
func quotedNames(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = `"` + ui.CleanDisplay(name) + `"`
	}
	return ui.JoinNames(quoted)
}
