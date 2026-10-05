package main

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/editor"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/ui"
)

// model runs the file TODO browser full screen, with a header naming the
// directory and a footer of keys.
type model struct {
	dir      string
	levelled bool
	theme    ui.Theme
	files    filesui.Model
	status   string
	width    int
	height   int
}

// newModel browses dir, listing only the TODOs keep accepts when it isn't
// nil.
func newModel(dir string, exclude scan.Exclude, keep func(scan.Match) bool) *model {
	return &model{dir: dir, levelled: keep != nil, theme: ui.NewTheme(true), files: filesui.New(dir, exclude, keep), width: 100, height: 30}
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.files.Scan(), tea.RequestBackgroundColor) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.theme = ui.NewTheme(msg.IsDark())
	case filesui.ScannedMsg:
		cmd := m.files.Update(msg)
		m.status = ""
		if err := m.files.Err(); err != nil {
			m.status = "Scan failed: " + err.Error()
		}
		return m, cmd
	case editor.ClosedMsg:
		m.status = ""
		if msg.Err != nil {
			m.status = msg.Err.Error()
		}
		return m, m.files.Scan()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.status = ""
			return m, m.files.Scan()
		}
		return m, m.files.Update(msg)
	default:
		return m, m.files.Update(msg)
	}
	return m, nil
}

var pinnedHints = []ui.KeyHint{{Key: "q", Label: "quit"}}

func (m *model) View() tea.View {
	width, height := m.width, m.height
	if width < 56 || height < 16 {
		v := tea.NewView("todo-scan · enlarge your terminal\nq quit")
		v.AltScreen = true
		return v
	}
	hints := m.files.Hints()
	if !m.files.Details() {
		hints = append(hints, ui.KeyHint{Key: "r", Label: "rescan"})
	}
	content := m.renderHeader(width) + "\n" + m.files.View(m.theme, width, height-2) + "\n" + m.theme.Footer(m.status, hints, pinnedHints, width)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// renderHeader names the app and the directory on the left, shortening
// the directory from the left, and the number of TODOs on the right.
func (m *model) renderHeader(width int) string {
	bar := lipgloss.NewStyle().Background(m.theme.ColorBar)
	title := m.theme.TitleStyle.Background(m.theme.ColorBar).Render(" todo-scan ")
	one, many := "TODO", "TODOs"
	if m.levelled {
		one, many = "levelled TODO", "levelled TODOs"
	}
	count := ui.Plural(m.files.Total(), one, many)
	if m.files.Loading() {
		count = "scanning…"
	}
	count = bar.Foreground(m.theme.ColorMuted).Render(count + " ")
	room := max(0, width-ansi.StringWidth(title)-ansi.StringWidth(count)-3)
	dir := displayDir(m.dir)
	if ansi.StringWidth(dir) > room {
		dir = "…" + ansi.TruncateLeft(dir, ansi.StringWidth(dir)-room+1, "")
	}
	left := title + bar.Foreground(m.theme.ColorText).Render(" "+dir)
	gap := bar.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(left)-ansi.StringWidth(count))))
	return left + gap + count
}

// displayDir writes dir with ~ for the home directory.
func displayDir(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	if rel, err := filepath.Rel(home, dir); err == nil && filepath.IsLocal(rel) {
		return filepath.Join("~", rel)
	}
	if dir == home {
		return "~"
	}
	return dir
}
