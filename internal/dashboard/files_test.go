package dashboard

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/ui"
)

// filesModel is a dashboard showing the Files tab, with a TODO that has note.
func filesModel(t *testing.T, note string) *model {
	t.Helper()
	m, _ := modelWithTasks(t, "- [ ] Task\n")
	m.files.Update(filesui.ScannedMsg{Matches: []scan.Match{{Path: "a.go", Line: 1, Note: note}}})
	return press(m, "3")
}

func TestFilesDetailsStopScrollingAtTheBottomOfTheTerminal(t *testing.T) {
	for _, c := range []struct {
		name   string
		note   string
		resize tea.Msg // nil leaves the terminal at the dashboard's default size
	}{
		{"the default size", strings.Repeat("word ", 600), nil},
		// The default size has room for this note, so only the smaller terminal needs scrolling.
		{"a resized terminal", strings.Repeat("word ", 150), tea.WindowSizeMsg{Width: 56, Height: 16}},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := filesModel(t, c.note)
			if c.resize != nil {
				m.Update(c.resize)
			}
			m = press(m, "right")
			for range 100 {
				m = press(m, "j")
			}
			atBottom := ansi.Strip(m.View().Content)
			m = press(m, "k")
			if !strings.Contains(atBottom, "Loading preview…") || ansi.Strip(m.View().Content) == atBottom {
				t.Fatalf("one up from the bottom of the details did not scroll them:\n%s", atBottom)
			}
		})
	}
}

func TestFilesTabFollowsTheTerminalsBackground(t *testing.T) {
	m := filesModel(t, "urgent")
	// The Files tab's panel is the only thing drawn in the border colour.
	border := func(dark bool) string {
		return ansi.NewStyle().ForegroundColor(ui.NewTheme(dark).ColorBorder).String() + "╭"
	}
	if view := m.View().Content; !strings.Contains(view, border(true)) {
		t.Fatalf("the Files tab is not drawn in the dark theme at first: %q", view)
	}
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if view := m.View().Content; !strings.Contains(view, border(false)) || strings.Contains(view, border(true)) {
		t.Fatalf("the Files tab did not take the light theme: %q", view)
	}
}
