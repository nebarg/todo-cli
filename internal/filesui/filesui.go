// Package filesui browses TODO comments in source files: a list grouped by
// todo-system category, most urgent first, and a detail page showing the
// code around one. The dashboard shows it as a tab.
package filesui

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/editor"
	"github.com/nebarg/todo-cli/internal/scan"
)

// previewRadius is how many lines either side of a TODO are read, enough
// to fill the detail page of a tall terminal.
const previewRadius = 40

// Model is the browser's state. Its zero value is an empty list; New sets
// what to scan.
type Model struct {
	dir     string
	exclude scan.Exclude
	keep    func(scan.Match) bool

	matches    []scan.Match
	loading    bool
	err        string
	cursor     int
	rootCursor int
	category   string

	details     bool
	scroll      int
	preview     []scan.ContextLine
	previewPath string
	previewLine int
	previewErr  string
}

// New browses dir, skipping exclude, and lists only the TODOs keep accepts,
// or all of them when keep is nil. It shows as loading until the first scan,
// started by Scan, comes back.
func New(dir string, exclude scan.Exclude, keep func(scan.Match) bool) Model {
	return Model{dir: dir, exclude: exclude, keep: keep, loading: true}
}

// ScannedMsg carries a scan's results.
type ScannedMsg struct {
	Matches []scan.Match
	Err     error
}

type previewMsg struct {
	path  string
	line  int
	lines []scan.ContextLine
	err   error
}

// Scan rescans in the background.
func (m *Model) Scan() tea.Cmd {
	m.loading = true
	dir, exclude, keep := m.dir, m.exclude, m.keep
	return func() tea.Msg {
		matches, err := scan.Source(dir, exclude)
		if keep != nil {
			matches = slices.DeleteFunc(matches, func(match scan.Match) bool { return !keep(match) })
		}
		return ScannedMsg{Matches: matches, Err: err}
	}
}

// PreviewCmd reads the code around the selected TODO for its detail page.
func (m *Model) PreviewCmd() tea.Cmd {
	selected, ok := m.Selected()
	if !ok {
		return nil
	}
	path := filepath.Join(m.dir, selected.Path)
	return func() tea.Msg {
		lines, err := scan.ReadContext(path, selected.Line, previewRadius)
		return previewMsg{path: selected.Path, line: selected.Line, lines: lines, err: err}
	}
}

// Update handles scan results, previews and the browser's keys.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ScannedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return nil
		}
		m.err = ""
		m.matches = msg.Matches
		if m.category != "" && len(m.rows()) == 0 {
			m.category, m.cursor = "", m.rootCursor
		}
		if m.cursor >= len(m.rows()) {
			m.cursor = 0
		}
		return m.PreviewCmd()
	case previewMsg:
		if selected, ok := m.Selected(); ok && selected.Path == msg.path && selected.Line == msg.line {
			m.previewPath, m.previewLine = msg.path, msg.line
			m.preview = msg.lines
			m.previewErr = ""
			if msg.err != nil {
				m.previewErr = msg.err.Error()
			}
		}
	case tea.KeyPressMsg:
		if m.details {
			return m.detailsKey(msg.String())
		}
		return m.listKey(msg.String())
	}
	return nil
}

func (m *Model) listKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		return m.move(-1)
	case "down", "j":
		return m.move(1)
	case "right":
		if m.openCategory() {
			return m.PreviewCmd()
		}
		if _, ok := m.Selected(); ok {
			m.details, m.scroll = true, 0
		}
	case "enter":
		if m.openCategory() {
			return m.PreviewCmd()
		}
		return m.openEditor()
	case "e":
		return m.openEditor()
	case "left", "esc":
		m.leaveCategory()
		return m.PreviewCmd()
	}
	return nil
}

func (m *Model) detailsKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		m.scroll = max(0, m.scroll-1)
	case "down", "j":
		m.scroll++
	case "enter", "e":
		return m.openEditor()
	case "left", "esc":
		m.CloseDetails()
		return m.PreviewCmd()
	}
	return nil
}

// Top goes up a level, as pressing the tab's key again does: from the
// detail page back to its list, or out of an opened category.
func (m *Model) Top() {
	if m.details {
		m.CloseDetails()
		return
	}
	m.leaveCategory()
}

// CloseDetails returns from the detail page to the list.
func (m *Model) CloseDetails() {
	m.details, m.scroll = false, 0
}

// Details is true while the detail page is showing.
func (m *Model) Details() bool { return m.details }

// Err is the last scan's error, if it failed.
func (m *Model) Err() string { return m.err }

// Loading is true while a scan is running.
func (m *Model) Loading() bool { return m.loading }

// Total is how many TODOs the last scan found.
func (m *Model) Total() int { return len(m.matches) }

func (m *Model) move(delta int) tea.Cmd {
	next := max(0, min(m.cursor+delta, len(m.rows())-1))
	if next == m.cursor {
		return nil
	}
	m.cursor = next
	m.previewPath, m.preview = "", nil
	return m.PreviewCmd()
}

func (m *Model) openCategory() bool {
	rows := m.rows()
	if m.cursor >= len(rows) || !rows[m.cursor].isGroup() {
		return false
	}
	m.rootCursor = m.cursor
	m.category = rows[m.cursor].category
	m.cursor = 0
	return true
}

func (m *Model) leaveCategory() {
	if m.category == "" {
		return
	}
	m.category = ""
	m.cursor = m.rootCursor
}

func (m *Model) openEditor() tea.Cmd {
	selected, ok := m.Selected()
	if !ok {
		return nil
	}
	return editor.Open(filepath.Join(m.dir, selected.Path), selected.Line)
}

// row is a todo@category group, or one TODO.
type row struct {
	category string
	count    int
	match    scan.Match
}

func (r row) isGroup() bool { return r.category != "" }

// rows lists the opened category's TODOs, or at the top, todo@category
// groups followed by the uncategorised TODOs, most urgent first as the scan
// returned them.
func (m *Model) rows() []row {
	var rows []row
	if m.category != "" {
		for _, match := range m.matches {
			if strings.EqualFold(match.Category, m.category) {
				rows = append(rows, row{match: match})
			}
		}
		return rows
	}
	counts := make(map[string]int)
	display := make(map[string]string)
	for _, match := range m.matches {
		if match.Category != "" {
			key := strings.ToLower(match.Category)
			counts[key]++
			if display[key] == "" {
				display[key] = match.Category
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(counts)) {
		rows = append(rows, row{category: display[key], count: counts[key]})
	}
	for _, match := range m.matches {
		if match.Category == "" {
			rows = append(rows, row{match: match})
		}
	}
	return rows
}

// Selected is the highlighted TODO, if the cursor is on one.
func (m *Model) Selected() (scan.Match, bool) {
	rows := m.rows()
	if m.cursor < 0 || m.cursor >= len(rows) || rows[m.cursor].isGroup() {
		return scan.Match{}, false
	}
	return rows[m.cursor].match, true
}
