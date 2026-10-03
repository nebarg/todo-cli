package dashboard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// deleteConfirmation is a planned delete of one task, with its details,
// waiting for the user to confirm it.
type deleteConfirmation struct {
	removal store.Removal
}

// startDelete asks to delete the selected task of todo.md.
func (m *model) startDelete() {
	if m.readmeSelected() {
		m.status = readmeReadOnly
		return
	}
	selected, ok := m.selectedTask()
	if !ok {
		if m.activePane() == sourcePane {
			m.status = "File TODOs are read only"
		} else {
			m.status = "Select a task to delete"
		}
		return
	}
	removal, err := store.PlanRemove(m.file, []store.Task{selected})
	if err != nil {
		m.status = errorStatus(err)
		return
	}
	m.overlay = &deleteConfirmation{removal: removal}
	m.status = ""
}

func (d *deleteConfirmation) update(msg tea.Msg) (overlay, tea.Msg, tea.Cmd) {
	return confirmOnY(d, msg, removalConfirmedMsg{d.removal, fmt.Sprintf("Deleted \"%s\"", d.text())})
}

func (d *deleteConfirmation) view(theme ui.Theme, _, _ int) string { return d.render(theme) }

func (d deleteConfirmation) render(theme ui.Theme) string {
	var details string
	if strings.TrimSpace(d.removal.Tasks[0].Details) != "" {
		details = "Its details go too."
	}
	title := fmt.Sprintf("Delete \"%s\"?", d.text())
	return renderConfirmation(theme, title, []string{details, emptiedHeadings(d.removal)}, "delete")
}

func (d deleteConfirmation) text() string { return ui.CleanDisplay(d.removal.Tasks[0].Text) }
