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
	removal     store.Removal
	targets     store.ClearTargets
	scope       string
	wholeBranch bool
}

// clearScope is the tasks X clears from where the user is, and a name for
// them: an opened category or branch, the current tab, or every task.
func (m *model) clearScope() ([]store.Task, string) {
	if m.all != nil {
		return m.tasks.all, "all tasks"
	}
	switch m.activePane() {
	case generalPane:
		switch m.general.open.kind {
		case rowReadme:
			return nil, readmeGroup
		case rowCategory:
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

// clearHint offers X only when there is something for it to clear.
func (m *model) clearHint() []ui.KeyHint {
	tasks, _ := m.clearScope()
	targets := store.PickClearTargets(tasks, m.branchMissing)
	var parts []string
	if targets.Done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", targets.Done))
	}
	switch {
	case m.viewingMissingBranch():
		return []ui.KeyHint{{Key: "X", Label: "remove its tasks"}}
	case len(targets.Branches) > 0:
		parts = append(parts, fmt.Sprintf("%d missing", len(targets.Branches)))
	case len(parts) == 0:
		return nil
	}
	return []ui.KeyHint{{Key: "X", Label: "clear " + strings.Join(parts, " + ")}}
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
	targets := store.PickClearTargets(tasks, m.branchMissing)
	if len(targets.Tasks) == 0 {
		m.status = "No done tasks to clear"
		return
	}
	removal, err := store.PlanRemove(m.file, targets.Tasks)
	if err != nil {
		m.status = errorStatus(err)
		return
	}
	m.overlay = &clearConfirmation{removal: removal, targets: targets, scope: scope, wholeBranch: m.viewingMissingBranch()}
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
	if c.wholeBranch {
		return "Removed the tasks of " + c.targets.Branches[0]
	}
	return "Removed " + c.targets.Summary()
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
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.lastRemoval = &msg.removal
	m.status = msg.status
	return m, nil
}

func (m *model) undoRemoval() {
	if m.lastRemoval == nil {
		m.status = "Nothing to undo"
		return
	}
	removal := *m.lastRemoval
	m.lastRemoval = nil
	if err := removal.Undo(); err != nil {
		if errors.Is(err, store.ErrFileChanged) {
			m.status = "The file has changed since, so it cannot be undone"
		} else {
			m.status = err.Error()
		}
		return
	}
	if err := m.refresh(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = "Restored " + taskCount(len(removal.Tasks))
}

func taskCount(n int) string { return ui.Plural(n, "task", "tasks") }

func (c clearConfirmation) render(theme ui.Theme) string {
	var title, reason string
	missingTasks := len(c.targets.Tasks) - c.targets.Done
	missing := make([]string, len(c.targets.Branches))
	for i, branch := range c.targets.Branches {
		missing[i] = branchIcon + " " + branch
	}
	switch {
	case c.wholeBranch:
		title = fmt.Sprintf("Remove the %s of %s?", taskCount(missingTasks), missing[0])
		reason = "The branch no longer exists in Git."
	case len(missing) == 1:
		title = fmt.Sprintf("Remove %s from %s?", c.targets.Summary(), c.scope)
		reason = fmt.Sprintf("%s no longer exists in Git, so all of its %s go too.", missing[0], taskCount(missingTasks))
	case len(missing) > 1:
		title = fmt.Sprintf("Remove %s from %s?", c.targets.Summary(), c.scope)
		reason = fmt.Sprintf("%s no longer exist in Git, so all of their %s go too.", joinNames(missing), taskCount(missingTasks))
	default:
		title = fmt.Sprintf("Remove %s from %s?", c.targets.Summary(), c.scope)
	}
	return renderConfirmation(theme, title, []string{reason, emptiedHeadings(c.removal, c.targets.Branches)}, "remove")
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

// emptiedHeadings names the headings removal leaves empty, other than those
// of the branches in named, or is "" when there are none.
func emptiedHeadings(removal store.Removal, named []string) string {
	var headings []string
	for _, category := range removal.Categories {
		headings = append(headings, "@"+category)
	}
	for _, branch := range removal.Branches {
		if !slices.Contains(named, branch) {
			headings = append(headings, branchIcon+" "+branch)
		}
	}
	switch len(headings) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("The %s heading will be empty and removed too.", headings[0])
	}
	return fmt.Sprintf("The %s headings will be empty and removed too.", joinNames(headings))
}

// joinNames lists names as "a", "a and b" or "a, b and c".
func joinNames(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
