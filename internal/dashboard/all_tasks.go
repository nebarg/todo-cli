package dashboard

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// allTasksView is the All tasks view, opened with i: every Markdown task in
// one list, sorted by priority, branch or category.
type allTasksView struct {
	sort   sortOrder
	cursor int
}

type sortOrder string

const (
	sortPriority sortOrder = "priority"
	sortBranch   sortOrder = "branch"
	sortCategory sortOrder = "category"
)

func (s sortOrder) next() sortOrder {
	switch s {
	case sortPriority:
		return sortBranch
	case sortBranch:
		return sortCategory
	default:
		return sortPriority
	}
}

func (m *model) openAllTasks() {
	m.all = &allTasksView{sort: sortPriority}
	m.status = ""
}

// allTasksKey handles the All tasks view's own keys, reporting false for
// any other.
func (m *model) allTasksKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keys.AllTasks, keys.Back):
		m.all = nil
		m.status = ""
	case key.Matches(msg, keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, keys.Sort):
		selected, ok := m.selectedTask()
		m.all.sort = m.all.sort.next()
		if ok {
			m.selectInAllTasks(selected)
		}
	case key.Matches(msg, keys.Enter):
		return m.startTaskModal(modalEdit), true
	default:
		return nil, false
	}
	return nil, true
}

// sorted is tasks in the view's order: grouped by branch or category when
// sorted by one, otherwise in the dashboard's order, with each task's
// subtasks after it.
func (v *allTasksView) sorted(tasks []store.Task) []store.Task {
	tasks = slices.Clone(tasks)
	slices.SortStableFunc(tasks, func(a, b store.Task) int {
		switch v.sort {
		case sortBranch:
			if c := compareGroup(a.Branch, b.Branch); c != 0 {
				return c
			}
			if a.Branch == "" {
				if c := compareGroup(a.Category, b.Category); c != 0 {
					return c
				}
			}
		case sortCategory:
			if c := compareGroup(a.Category, b.Category); c != 0 {
				return c
			}
			if a.Category == "" {
				if c := compareGroup(a.Branch, b.Branch); c != 0 {
					return c
				}
			}
		}
		return 0
	})
	return nestSubtasks(tasks, func(t store.Task) int { return t.Line }, func(t store.Task) bool { return t.Subtask })
}

// compareGroup orders category or branch names, with tasks outside any last.
func compareGroup(a, b string) int {
	if a == "" && b != "" {
		return 1
	}
	if b == "" && a != "" {
		return -1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func (m *model) selectInAllTasks(selected store.Task) {
	if i := nearestTask(m.all.sorted(m.tasks.all), selected); i >= 0 {
		m.all.cursor = i
	}
}

func priorityStyle(theme ui.Theme, p store.Priority) lipgloss.Style {
	color := theme.ColorMuted
	switch p {
	case store.PriorityHigh:
		color = theme.ColorHigh
	case store.PriorityMedium:
		color = theme.ColorMedium
	case store.PriorityLow:
		color = theme.ColorLow
	}
	return lipgloss.NewStyle().Bold(p != store.PriorityNone).Foreground(color)
}

// priorityMark fills an open task's circle when it has a priority, whose
// colour then tells the levels apart; an empty circle means no priority.
func priorityMark(p store.Priority) string {
	if p == store.PriorityNone {
		return "○"
	}
	return "●"
}

// view draws tasks in the view's order, marking those of a branch missing
// reports as gone.
func (v *allTasksView) view(theme ui.Theme, tasks []store.Task, missing func(branch string) bool, width, height int) string {
	innerWidth := max(1, width-4)
	tasks = v.sorted(tasks)
	scopeWidth := min(24, max(17, innerWidth/3))
	taskWidth := max(1, innerWidth-2-scopeWidth)
	heading := theme.TitleStyle.Render("All tasks") + theme.MutedStyle.Render("  "+countText(countTasks(tasks)))
	sorted := theme.MutedStyle.Render("sorted by ") + lipgloss.NewStyle().Foreground(theme.ColorText).Render(string(v.sort))
	if gap := innerWidth - ansi.StringWidth(heading) - ansi.StringWidth(sorted); gap >= 2 {
		heading += strings.Repeat(" ", gap) + sorted
	}
	lines := []string{ansi.Truncate(heading, innerWidth, "…"), ""}
	columns := ui.Column("TASK: DETAILS", taskWidth) + "  " + ui.Column("CATEGORY / BRANCH", scopeWidth)
	lines = append(lines, theme.MutedStyle.Render(columns))
	visible := max(1, height-5)
	start, end := ui.VisibleRange(v.cursor, len(tasks), visible)
	if len(tasks) == 0 {
		lines = append(lines, theme.MutedStyle.Render("No tasks in the Markdown file"))
	}
	parentDone := make([]bool, len(tasks)) // each subtask's task is done
	for i, done := range tasks {
		if done.Subtask && i > 0 {
			parentDone[i] = parentDone[i-1] || !tasks[i-1].Subtask && tasks[i-1].Done
		}
	}
	for i := start; i < end; i++ {
		t := tasks[i]
		scope := "-"
		if t.Category != "" {
			scope = "@" + t.Category
		}
		gone := missing(t.Branch)
		if t.Branch != "" {
			scope = branchIcon + " " + t.Branch
		}
		if gone {
			scope = "⚠ " + t.Branch
		}
		mark, markStyle := priorityMark(t.Priority)+" ", theme.MutedStyle
		if t.Priority != store.PriorityNone {
			markStyle = priorityStyle(theme, t.Priority)
		}
		taskStyle := lipgloss.NewStyle().Foreground(theme.ColorStrong)
		switch {
		case t.Done:
			mark, markStyle, taskStyle = "✓ ", theme.MutedStyle, theme.MutedStyle
		case parentDone[i]:
			markStyle, taskStyle = theme.MutedStyle, theme.MutedStyle
		}
		mark = strings.Repeat(" ", rowIndent(t.Subtask)) + mark
		selected := i == v.cursor
		scopeStyle := theme.MutedStyle
		switch {
		case gone:
			scopeStyle = lipgloss.NewStyle().Foreground(theme.ColorHigh)
		case t.Branch != "":
			scopeStyle = lipgloss.NewStyle().Foreground(theme.ColorGreen)
		case t.Category != "":
			scopeStyle = lipgloss.NewStyle().Foreground(theme.ColorPurple)
		}
		if selected {
			markStyle = markStyle.Background(theme.ColorSelection)
			taskStyle = taskStyle.Background(theme.ColorSelection)
			scopeStyle = scopeStyle.Background(theme.ColorSelection)
		}
		gap := "  "
		if selected {
			gap = lipgloss.NewStyle().Background(theme.ColorSelection).Render(gap)
		}
		title := theme.Inline(t.Text, taskStyle)
		if details := strings.Join(strings.Fields(ui.CleanDisplay(t.Details)), " "); details != "" {
			title += taskStyle.Render(": ") + theme.Inline(details, taskStyle)
		}
		row := markStyle.Render(mark) + ui.StyledColumn(title, taskWidth-ansi.StringWidth(mark), taskStyle) + gap +
			scopeStyle.Render(ui.Column(scope, scopeWidth))
		lines = append(lines, row)
	}
	return theme.Panel(width, height, lines, "")
}
