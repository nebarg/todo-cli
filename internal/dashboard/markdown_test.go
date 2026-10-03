package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/filesui"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

const markdownTask = "- [ ] Fix **login** in `auth.go` !high\n\n  Check the `redirect` *carefully*:\n\n  ```sh\n  go test -run **Login**\n  ```\n"

func markdownModel(t *testing.T) *model {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	if err := os.WriteFile(path, []byte(markdownTask), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("## TODOs\n\n- todo0 Document `--force`\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 120, 30
	return m
}

func TestTaskListShowsMarkdownFormatting(t *testing.T) {
	m := markdownModel(t)
	m.general.cursor = 1
	if row, ok := m.selectedNavigationRow(); !ok || row.kind != rowTask {
		t.Fatalf("the task is not the second row: %+v", m.rows(generalPane))
	}
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "● Fix login in  auth.go ") || strings.Contains(plain, "**") || strings.Contains(plain, "`") {
		t.Errorf("task row shows its Markdown markers:\n%s", plain)
	}

	task := m.tasks.general[0]
	selected := renderTaskRow(m.theme, task, 60, true)
	text := lipgloss.NewStyle().Foreground(m.theme.ColorStrong).Background(m.theme.ColorSelection)
	if !strings.Contains(selected, text.Bold(true).Render("login")) || !strings.Contains(selected, text.Background(m.theme.ColorField).Foreground(m.theme.ColorCode).Render(" auth.go ")) {
		t.Errorf("selected row lost its formatting or its background: %q", selected)
	}
	if got := ansi.StringWidth(selected); got != 60 {
		t.Errorf("selected row width = %d, want 60", got)
	}
	if cut := ansi.Strip(renderTaskRow(m.theme, task, 15, false)); cut != "● Fix login…  ⋯" {
		t.Errorf("narrow row = %q", cut)
	}
	done := task
	done.Done = true
	if row := renderTaskRow(m.theme, done, 60, false); !strings.Contains(row, m.theme.MutedStyle.Bold(true).Render("login")) {
		t.Errorf("done row is not muted with its formatting: %q", row)
	}
}

func TestDetailPageShowsMarkdownFormattingOutsideCodeBlocks(t *testing.T) {
	m := markdownModel(t)
	m.general.cursor = 1
	pressKey(t, m, "right")
	if m.focus != detailPane {
		t.Fatalf("right did not open the task's details")
	}
	plain := ansi.Strip(m.renderDetailPane(80, 20))
	for _, want := range []string{"Fix login in  auth.go ", "Check the  redirect  carefully:", "```sh", "go test -run **Login**"} {
		if !strings.Contains(plain, want) {
			t.Errorf("details are missing %q:\n%s", want, plain)
		}
	}
	if got := m.taskDetails(80)[0]; got != m.theme.Inline("Fix **login** in `auth.go`", m.theme.TaskTitleStyle) {
		t.Errorf("title is not drawn in the title style: %q", got)
	}
}

func TestReadmeTasksShowMarkdownFormatting(t *testing.T) {
	m := markdownModel(t)
	pressKey(t, m, "right")
	if m.general.open.kind != rowReadme {
		t.Fatalf("right did not open the README group")
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "0 Document  --force ") {
		t.Errorf("README row shows its Markdown markers:\n%s", plain)
	}
	pressKey(t, m, "right")
	if title := ansi.Strip(m.taskDetails(60)[0]); title != "0 Document  --force " {
		t.Errorf("README detail title = %q", title)
	}
}

func TestAllTasksShowMarkdownFormatting(t *testing.T) {
	m := markdownModel(t)
	pressKey(t, m, "i")
	view := m.View().Content
	if plain := ansi.Strip(view); !strings.Contains(plain, "● Fix login in  auth.go : Check the  redirect  carefully:") {
		t.Errorf("All tasks shows the Markdown markers:\n%s", plain)
	}
	if lipgloss.Width(view) != m.width {
		t.Errorf("All tasks width = %d, want %d", lipgloss.Width(view), m.width)
	}
	selected := lipgloss.NewStyle().Foreground(m.theme.ColorStrong).Background(m.theme.ColorSelection)
	gap := lipgloss.NewStyle().Background(m.theme.ColorSelection).Render("  ")
	if !strings.Contains(view, selected.Bold(true).Render("login")) || !strings.Contains(view, selected.Render("    ")+gap) {
		t.Errorf("selected task lost its formatting or its padded background: %q", view)
	}
}

func TestEditFormShowsTheMarkdownAsWritten(t *testing.T) {
	m := markdownModel(t)
	m.general.cursor = 1
	pressKey(t, m, "e")
	f := form(t, m)
	if got := f.title.Value(); got != "Fix **login** in `auth.go`" {
		t.Errorf("edit form title = %q", got)
	}
	if got := f.details.Value(); !strings.Contains(got, "Check the `redirect` *carefully*:") || !strings.Contains(got, "go test -run **Login**") {
		t.Errorf("edit form details = %q", got)
	}
}

func TestFileTodosShowTheirTextAsWritten(t *testing.T) {
	m := &model{theme: ui.NewTheme(true), width: 100, height: 20, tasks: taskSet{general: []store.Task{}}}
	m.files.Update(filesui.ScannedMsg{Matches: []scan.Match{{Path: "a.c", Line: 1, Note: "free `buf` and *ptr*"}}})
	m = press(m, "3")
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "free `buf` and *ptr*") {
		t.Errorf("Files list formatted a code comment:\n%s", plain)
	}
	pressKey(t, m, "right")
	if plain := ansi.Strip(m.View().Content); !m.files.Details() || !strings.Contains(plain, "free `buf` and *ptr*") {
		t.Errorf("Files details formatted a code comment:\n%s", plain)
	}
}
