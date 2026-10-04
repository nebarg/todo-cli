package dashboard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// deleteConfirmation is a planned delete of one task with its details, or of
// every task in a category or branch, waiting for the user to confirm it.
type deleteConfirmation struct {
	removal  store.Removal
	group    string // the category or branch that goes, such as the "docs" category, or "" for one task
	subtasks int    // how many subtasks go with one task
}

// startDelete asks to delete the selected task of todo.md, or every task of
// the selected category or branch, done or not.
func (m *model) startDelete() {
	if m.readmeSelected() {
		m.status = readmeReadOnly
		return
	}
	// The All tasks view lists only tasks, over a General or Branches row
	// that may be a group.
	if row, ok := m.selectedNavigationRow(); ok && !row.isTask() && m.all == nil {
		m.startGroupDelete(row)
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
	_, subtasks := m.selectedSubtasks()
	m.overlay = &deleteConfirmation{removal: removal, subtasks: subtasks}
	m.status = ""
}

// startGroupDelete asks to delete every task of a category or branch row,
// which takes its heading too once nothing else is under it.
func (m *model) startGroupDelete(row navigationRow) {
	if row.kind == rowReadme {
		m.status = readmeReadOnly
		return
	}
	tasks := rowTasks(m.rowsIn(m.activePane(), group{kind: row.kind, name: row.name}))
	removal, err := store.PlanRemove(m.file, tasks)
	if err != nil {
		m.status = errorStatus(err)
		return
	}
	name := `the "` + ui.CleanDisplay(row.name) + `" category`
	if row.kind == rowBranch {
		name = `the "` + ui.CleanDisplay(row.name) + `" branch`
	}
	m.overlay = &deleteConfirmation{removal: removal, group: name}
	m.status = ""
}

func (d *deleteConfirmation) update(msg tea.Msg) (overlay, tea.Msg, tea.Cmd) {
	return confirmOnY(d, msg, removalConfirmedMsg{d.removal, d.doneStatus()})
}

// doneStatus reports the delete once it is applied.
func (d *deleteConfirmation) doneStatus() string {
	if d.group != "" {
		return fmt.Sprintf("Deleted %s and its %s", d.group, taskCount(len(d.removal.Tasks)))
	}
	return fmt.Sprintf("Deleted \"%s\"", d.text())
}

func (d *deleteConfirmation) view(theme ui.Theme, _, _ int) string { return d.render(theme) }

func (d deleteConfirmation) render(theme ui.Theme) string {
	if d.group != "" {
		title := fmt.Sprintf("Delete %s and its %s?", d.group, taskCount(len(d.removal.Tasks)))
		return renderConfirmation(theme, title, []string{openAndDone(d.removal.Tasks)}, "delete")
	}
	title := fmt.Sprintf("Delete \"%s\"?", d.text())
	return renderConfirmation(theme, title, []string{d.goesWithIt(), emptiedGroups(d.removal)}, "delete")
}

// goesWithIt names what a task takes with it, as in "Its details and 2
// subtasks go too.", or is "" for a task with neither.
func (d deleteConfirmation) goesWithIt() string {
	var parts []string
	if strings.TrimSpace(d.removal.Tasks[0].Details) != "" {
		parts = append(parts, "details")
	}
	if d.subtasks > 0 {
		parts = append(parts, ui.Plural(d.subtasks, "subtask", "subtasks"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Its " + strings.Join(parts, " and ") + " go too."
}

// openAndDone counts tasks as "2 open, 1 done", leaving out a count of none.
func openAndDone(tasks []store.Task) string {
	done := completedCount(tasks)
	var counts []string
	if open := len(tasks) - done; open > 0 {
		counts = append(counts, fmt.Sprintf("%d open", open))
	}
	if done > 0 {
		counts = append(counts, fmt.Sprintf("%d done", done))
	}
	return strings.Join(counts, ", ")
}

func (d deleteConfirmation) text() string { return ui.CleanDisplay(d.removal.Tasks[0].Text) }
