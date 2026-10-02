package dashboard

import (
	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/ui"
)

// overlay takes the keyboard while it is open over the dashboard: the help,
// a clear or delete confirmation, the task form or the category prompt.
type overlay interface {
	// update handles a key or a paste. It returns the overlay to keep
	// open, or nil once it has closed, and anything the model should act
	// on, such as a saved task.
	update(msg tea.Msg) (next overlay, outcome tea.Msg, cmd tea.Cmd)
}

// dialog is an overlay drawn in a box over the middle of the dashboard.
type dialog interface {
	overlay
	view(theme ui.Theme, width, height int) string // in a terminal of width by height cells
}

// updateOverlay passes msg to the open overlay, then acts on its outcome
// straight away, so the dashboard is up to date as the overlay closes.
func (m *model) updateOverlay(msg tea.Msg) tea.Cmd {
	next, outcome, cmd := m.overlay.update(msg)
	m.overlay = next
	if outcome == nil {
		return cmd
	}
	_, outcomeCmd := m.Update(outcome)
	return tea.Batch(cmd, outcomeCmd)
}
