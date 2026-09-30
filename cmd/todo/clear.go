package main

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
)

// clearConfirmation is a planned clear waiting for the user to confirm it.
type clearConfirmation struct {
	removal     store.Removal
	targets     clearTargets
	scope       string
	wholeBranch bool
}

// clearTargets is what a clear removes: every done task, and every task of a
// branch that no longer exists in Git.
type clearTargets struct {
	tasks    []store.Task
	done     int
	branches []string
}

func pickClearTargets(tasks []store.Task, missing func(string) bool) clearTargets {
	var c clearTargets
	for _, t := range tasks {
		switch {
		case missing(t.Branch):
			c.tasks = append(c.tasks, t)
			if !slices.Contains(c.branches, t.Branch) {
				c.branches = append(c.branches, t.Branch)
			}
		case t.Done:
			c.tasks = append(c.tasks, t)
			c.done++
		}
	}
	sort.Strings(c.branches)
	return c
}

// summary reads like "3 done tasks and 1 missing branch".
func (c clearTargets) summary() string {
	var parts []string
	if c.done > 0 {
		parts = append(parts, doneTaskCount(c.done))
	}
	if len(c.branches) > 0 {
		parts = append(parts, missingBranchCount(len(c.branches)))
	}
	return strings.Join(parts, " and ")
}

// clearScope is the tasks X clears from where the user is, and a name for
// them: an opened category or branch, the current tab, or every task.
func (m *model) clearScope() ([]store.Task, string) {
	if m.indexMode {
		return m.allTasks, "all tasks"
	}
	switch m.activePane() {
	case generalPane:
		if m.generalCategory == "" {
			return m.general, "General"
		}
		var tasks []store.Task
		for _, t := range m.general {
			if taskInCategory(t, m.generalCategory) {
				tasks = append(tasks, t)
			}
		}
		return tasks, "@" + m.generalCategory
	case branchPane:
		if m.branchFilter == "" {
			return m.branches, "Branches"
		}
		var tasks []store.Task
		for _, t := range m.branches {
			if t.Branch == m.branchFilter {
				tasks = append(tasks, t)
			}
		}
		return tasks, branchIcon + " " + m.branchFilter
	}
	return nil, ""
}

// clearHint offers X only when there is something for it to clear.
func (m *model) clearHint() []keyHint {
	tasks, _ := m.clearScope()
	targets := pickClearTargets(tasks, m.branchMissing)
	var parts []string
	if targets.done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", targets.done))
	}
	switch {
	case m.viewingMissingBranch():
		return []keyHint{{"X", "delete branch"}}
	case len(targets.branches) > 0:
		parts = append(parts, fmt.Sprintf("%d missing", len(targets.branches)))
	case len(parts) == 0:
		return nil
	}
	return []keyHint{{"X", "clear " + strings.Join(parts, " + ")}}
}

func (m *model) startClearDone() {
	if m.activePane() == sourcePane && !m.indexMode {
		m.status = "File TODOs are read only"
		return
	}
	tasks, scope := m.clearScope()
	targets := pickClearTargets(tasks, m.branchMissing)
	if len(targets.tasks) == 0 {
		m.status = "No done tasks to clear"
		return
	}
	removal, err := store.PlanRemove(m.file, targets.tasks)
	if err != nil {
		m.status = errorStatus(err)
		return
	}
	m.confirmClear = &clearConfirmation{removal: removal, targets: targets, scope: scope, wholeBranch: m.viewingMissingBranch()}
	m.status = ""
}

// updateClearConfirmation removes the tasks on y; any other key cancels, so a
// stray key never deletes anything.
func (m *model) updateClearConfirmation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	confirmation := *m.confirmClear
	m.confirmClear = nil
	if msg.String() != "y" {
		return m, nil
	}
	if err := confirmation.removal.Apply(); err != nil {
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
	m.lastClear = &confirmation.removal
	if confirmation.wholeBranch {
		m.status = "Deleted branch " + confirmation.targets.branches[0]
	} else {
		m.status = "Removed " + confirmation.targets.summary()
	}
	return m, nil
}

func (m *model) undoClear() {
	if m.lastClear == nil {
		m.status = "Nothing to undo"
		return
	}
	removal := *m.lastClear
	m.lastClear = nil
	if err := removal.Undo(); err != nil {
		if errors.Is(err, store.ErrFileChanged) {
			m.status = "The file changed after clearing, so it cannot be undone"
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

func doneTaskCount(n int) string { return plural(n, "done task", "done tasks") }

func taskCount(n int) string { return plural(n, "task", "tasks") }

func missingBranchCount(n int) string { return plural(n, "missing branch", "missing branches") }

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (c clearConfirmation) render() string {
	const width = 48
	var title, reason string
	missingTasks := len(c.targets.tasks) - c.targets.done
	missing := make([]string, len(c.targets.branches))
	for i, branch := range c.targets.branches {
		missing[i] = branchIcon + " " + branch
	}
	switch {
	case c.wholeBranch:
		title = fmt.Sprintf("Delete %s and its %s?", missing[0], taskCount(missingTasks))
		reason = "The branch no longer exists in Git."
	case len(missing) == 1:
		title = fmt.Sprintf("Remove %s from %s?", c.targets.summary(), c.scope)
		reason = fmt.Sprintf("%s no longer exists in Git, so all of its %s go too.", missing[0], taskCount(missingTasks))
	case len(missing) > 1:
		title = fmt.Sprintf("Remove %s from %s?", c.targets.summary(), c.scope)
		reason = fmt.Sprintf("%s no longer exist in Git, so all of their %s go too.", joinNames(missing), taskCount(missingTasks))
	default:
		title = fmt.Sprintf("Remove %s from %s?", c.targets.summary(), c.scope)
	}
	lines := []string{titleStyle.Render(ansi.Wrap(title, width, ""))}
	if reason != "" {
		lines = append(lines, mutedStyle.Render(ansi.Wrap(reason, width, "")))
	}
	var headings []string
	for _, category := range c.removal.Categories {
		headings = append(headings, "@"+category)
	}
	for _, branch := range c.removal.Branches {
		if !slices.Contains(c.targets.branches, branch) {
			headings = append(headings, branchIcon+" "+branch)
		}
	}
	if len(headings) > 0 {
		noun := "heading"
		if len(headings) > 1 {
			noun = "headings"
		}
		lines = append(lines, mutedStyle.Render(ansi.Wrap(fmt.Sprintf("The %s %s will be empty and removed too.", joinNames(headings), noun), width, "")))
	}
	lines = append(lines, "", renderHints([]keyHint{{"y", "remove"}, {"esc", "cancel"}}))
	return lipgloss.NewStyle().Padding(0, 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorHigh).BorderBackground(colorModal).
		Background(colorModal).Render(onBackground(strings.Join(lines, "\n"), colorModal))
}

// joinNames lists names as "a", "a and b" or "a, b and c".
func joinNames(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
