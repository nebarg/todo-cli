package dashboard

import (
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

func TestThemeFollowsTerminalBackground(t *testing.T) {
	m := &model{theme: ui.NewTheme(true)}
	dark := m.theme.ColorText
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if m.theme.ColorText == dark || m.theme.ColorStrong == color.Color(lipgloss.Color("#FFFFFF")) {
		t.Fatal("light background kept the dark palette")
	}
	if got := priorityStyle(m.theme, store.PriorityHigh).GetForeground(); got != m.theme.ColorHigh {
		t.Fatalf("priority style did not use the light palette: %v", got)
	}
	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if m.theme.ColorText != dark {
		t.Fatal("dark background did not restore the dark palette")
	}
}

func TestTaskFormRestylesWhenTheBackgroundChanges(t *testing.T) {
	m, err := newModel(filepath.Join(t.TempDir(), "todo.md"), project.Repo{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 24
	pressKey(t, m, "a")
	f := form(t, m)
	dark, light := ui.NewTheme(true), ui.NewTheme(false)
	checkStyles := func(theme ui.Theme) {
		t.Helper()
		for _, c := range []struct {
			name      string
			got, want color.Color
		}{
			{"title text", f.title.Styles().Focused.Text.GetForeground(), theme.ColorStrong},
			{"title field", f.title.Styles().Focused.Base.GetBackground(), theme.ColorField},
			{"title cursor", f.title.Styles().Cursor.Color, theme.ColorFocus},
			{"details blurred text", f.details.Styles().Blurred.Text.GetForeground(), theme.ColorText},
			{"scope text", f.scope.Styles().Focused.Text.GetForeground(), theme.ColorStrong},
			{"scope placeholder", f.scope.Styles().Focused.Placeholder.GetForeground(), theme.ColorMuted},
			{"scope cursor", f.scope.Styles().Cursor.Color, theme.ColorFocus},
		} {
			if c.got != c.want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		}
	}
	checkStyles(dark)

	m.Update(tea.BackgroundColorMsg{Color: color.White})
	checkStyles(light)
	// The field colour is one parameter of the escape sequences the form draws with.
	field := func(theme ui.Theme) string {
		return strings.TrimSuffix(strings.TrimPrefix(ansi.NewStyle().BackgroundColor(theme.ColorField).String(), "\x1b["), "m")
	}
	if view := m.View().Content; !strings.Contains(view, field(light)) || strings.Contains(view, field(dark)) {
		t.Fatalf("the open form is not drawn on the light field colour: %q", view)
	}

	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	checkStyles(dark)
}
