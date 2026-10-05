package dashboard

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
	"github.com/nebarg/todo-cli/internal/store"
	"github.com/nebarg/todo-cli/internal/ui"
)

// selectTask puts the cursor on the task reading text in the focused list.
func selectTask(t *testing.T, m *model, text string) {
	t.Helper()
	for i, row := range m.rows(m.focus) {
		if row.kind == rowTask && row.todo.Text == text {
			m.list(m.focus).cursor = i
			return
		}
	}
	t.Fatalf("no task %q in the list", text)
}

// deleteDialog is the open delete confirmation as plain text on one line.
func deleteDialog(t *testing.T, m *model) string {
	t.Helper()
	dialog := ansi.Strip(opened[*deleteConfirmation](t, m).render(m.theme))
	return strings.Join(strings.Fields(strings.ReplaceAll(dialog, "│", " ")), " ")
}

func TestDeleteRemovesTheSelectedTaskUntilUndone(t *testing.T) {
	m, path := clearModel(t)
	selectTask(t, m, "Loose open")
	if footer := ansi.Strip(m.renderFooter(200)); !strings.Contains(footer, "⌫ delete") {
		t.Fatalf("footer lacks the delete hint: %q", footer)
	}
	if help := strings.Join(strings.Fields(ansi.Strip(renderHelp(m.theme))), " "); !strings.Contains(help, "⌫ delete") {
		t.Fatalf("help does not list delete: %s", help)
	}
	m = press(m, "backspace")
	if dialog := deleteDialog(t, m); !strings.Contains(dialog, `Delete "Loose open"?`) || !strings.Contains(dialog, "y delete") || strings.Contains(dialog, "heading") {
		t.Fatalf("dialog = %s", dialog)
	}
	if fileContent(t, path) != clearContent {
		t.Fatal("asking for confirmation changed the file")
	}
	m = press(m, "y")
	want := strings.Replace(clearContent, "- [ ] Loose open\n\n", "", 1)
	if got := fileContent(t, path); got != want || m.status != `Deleted "Loose open"` {
		t.Fatalf("file = %q, status %q", got, m.status)
	}
	if hints := m.footerHints(); len(hints) == 0 || hints[0] != (ui.KeyHint{Key: "u", Label: "undo"}) {
		t.Fatalf("footer does not offer undo first: %v", hints)
	}
	m = press(m, "u")
	if got := fileContent(t, path); got != clearContent || m.status != "Restored 1 task" {
		t.Fatalf("undo left %q with status %q", got, m.status)
	}
}

func TestDeleteTakesDetailsAndEmptiedHeadings(t *testing.T) {
	const content = "- [ ] Keep\n\n# docs\n\n- [ ] Write guide\n\n  Cover install and usage.\n\n# auth\n\n- [ ] Auth open\n\n# Branches\n\n## feature/x\n\n- [ ] Branch open\n"
	for _, item := range []struct {
		name  string
		open  func(m *model)
		task  string
		notes []string
		want  string
	}{
		{"category", func(m *model) { m.general.open = categoryGroup("docs") }, "Write guide",
			[]string{"Its details go too.", `The empty "docs" category will be removed too.`},
			"- [ ] Keep\n\n# auth\n\n- [ ] Auth open\n\n# Branches\n\n## feature/x\n\n- [ ] Branch open\n"},
		{"branch", func(m *model) { m.focus, m.branch.open = branchPane, branchGroup("feature/x") }, "Branch open",
			[]string{`The empty "feature/x" branch will be removed too.`},
			"- [ ] Keep\n\n# docs\n\n- [ ] Write guide\n\n  Cover install and usage.\n\n# auth\n\n- [ ] Auth open\n"},
	} {
		t.Run(item.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			m, err := newModel(path, project.Repo{}, testFiles())
			if err != nil {
				t.Fatal(err)
			}
			item.open(m)
			selectTask(t, m, item.task)
			m = press(m, "backspace")
			dialog := deleteDialog(t, m)
			for _, note := range item.notes {
				if !strings.Contains(dialog, note) {
					t.Errorf("dialog lacks %q: %s", note, dialog)
				}
			}
			m = press(m, "y")
			if got := fileContent(t, path); got != item.want {
				t.Fatalf("file = %q, want %q", got, item.want)
			}
			if m.general.open != (group{}) || m.branch.open != (group{}) {
				t.Fatal("the emptied group stayed open")
			}
		})
	}
}

func TestDeleteOnlyOnY(t *testing.T) {
	for _, key := range []string{"esc", "n", "enter", "backspace"} {
		t.Run(key, func(t *testing.T) {
			m, path := clearModel(t)
			selectTask(t, m, "Loose open")
			m = press(press(m, "backspace"), key)
			if isOpen[*deleteConfirmation](m) || fileContent(t, path) != clearContent || m.lastRemoval != nil {
				t.Fatalf("%s did not cancel cleanly", key)
			}
		})
	}
}

func TestDeleteFromAllTasksAndDetails(t *testing.T) {
	for _, item := range []struct {
		name string
		pick func(t *testing.T, m *model)
	}{
		{"all tasks", func(t *testing.T, m *model) {
			m.openAllTasks()
			m.all.cursor = slices.IndexFunc(m.all.sorted(m.tasks.all), func(t store.Task) bool { return t.Text == "Auth open" })
		}},
		{"details", func(t *testing.T, m *model) {
			m.general.open = categoryGroup("auth")
			selectTask(t, m, "Auth open")
			m.detailFrom, m.focus = generalPane, detailPane
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			m, path := clearModel(t)
			item.pick(t, m)
			m = press(press(m, "backspace"), "y")
			if strings.Contains(fileContent(t, path), "Auth open") || m.status != `Deleted "Auth open"` {
				t.Fatalf("file = %q, status %q", fileContent(t, path), m.status)
			}
			if m.focus == detailPane {
				t.Fatal("the details of the deleted task stayed open")
			}
		})
	}
}

func TestDeleteRefusesWhatItCannotDelete(t *testing.T) {
	for _, item := range []struct {
		name   string
		setup  func(t *testing.T) *model
		status string
	}{
		{"file TODO", func(t *testing.T) *model {
			m, _ := clearModel(t)
			m.focus = sourcePane
			return m
		}, "File TODOs are read only"},
		{"README.md row", func(t *testing.T) *model {
			m, path := clearModel(t)
			if err := os.WriteFile(filepath.Join(filepath.Dir(path), "README.md"), []byte("## TODOs\n\n- Write docs\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := m.refresh(); err != nil {
				t.Fatal(err)
			}
			selectGroup(t, m, rowReadme, readmeGroup)
			if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); strings.Contains(hints, "delete") {
				t.Fatalf("README.md row offers delete: %q", hints)
			}
			return m
		}, readmeReadOnly},
		{"README task", func(t *testing.T) *model {
			m, path := clearModel(t)
			if err := os.WriteFile(filepath.Join(filepath.Dir(path), "README.md"), []byte("## TODOs\n\n- Write docs\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := m.refresh(); err != nil {
				t.Fatal(err)
			}
			m.general.open = group{kind: rowReadme, name: readmeGroup}
			return m
		}, readmeReadOnly},
	} {
		t.Run(item.name, func(t *testing.T) {
			m := press(item.setup(t), "backspace")
			if isOpen[*deleteConfirmation](m) || m.status != item.status {
				t.Fatalf("status = %q, want %q", m.status, item.status)
			}
		})
	}
}

func TestDeleteWorksInABranchNotInGit(t *testing.T) {
	m, path := missingBranchModel(t)
	m.branch.open = branchGroup("feature/gone")
	selectTask(t, m, "Gone open")
	m = press(press(m, "backspace"), "y")
	want := "- [x] Loose done\n\n# Branches\n\n## feature/gone\n\n- [x] Gone done\n\n## feature/live\n\n- [ ] Live open\n\n- [x] Live done\n"
	if got := fileContent(t, path); got != want || m.status != `Deleted "Gone open"` {
		t.Fatalf("file = %q, status %q", got, m.status)
	}
}

// selectGroup puts the cursor on the category, branch or README.md row
// called name in the focused list.
func selectGroup(t *testing.T, m *model, kind navigationKind, name string) {
	t.Helper()
	for i, row := range m.rows(m.focus) {
		if row.kind == kind && row.name == name {
			m.list(m.focus).cursor = i
			return
		}
	}
	t.Fatalf("no group %q in the list", name)
}

func TestDeleteRemovesACategoryAndAllItsTasks(t *testing.T) {
	m, path := clearModel(t)
	selectGroup(t, m, rowCategory, "auth")
	if hints := ansi.Strip(m.theme.RenderHints(m.footerHints())); !strings.Contains(hints, "→ open  ⌫ delete  a add") {
		t.Fatalf("category row footer = %q", hints)
	}
	m = press(m, "backspace")
	dialog := strings.Join(strings.Fields(strings.ReplaceAll(ansi.Strip(opened[*deleteConfirmation](t, m).render(m.theme)), "│", " ")), " ")
	for _, want := range []string{`Delete the "auth" category and its 2 tasks?`, "1 open, 1 done"} {
		if !strings.Contains(dialog, want) {
			t.Errorf("dialog lacks %q: %s", want, dialog)
		}
	}
	m = press(m, "y")
	want := "- [x] Loose done\n\n- [ ] Loose open\n\n# docs\n\n- [x] Doc done\n\n# Branches\n\n## feature/x\n\n- [x] Branch done\n"
	if got := fileContent(t, path); got != want || m.status != `Deleted the "auth" category and its 2 tasks` {
		t.Fatalf("file = %q, status %q", got, m.status)
	}
	if rows := m.rows(generalPane); slices.ContainsFunc(rows, func(r navigationRow) bool { return r.name == "auth" }) {
		t.Fatalf("the deleted category is still listed: %+v", rows)
	}
	if press(m, "u"); fileContent(t, path) != clearContent {
		t.Fatal("undo did not bring the category back")
	}
}

func TestDeleteRemovesABranchAndAllItsTasks(t *testing.T) {
	m, path := missingBranchModel(t)
	selectGroup(t, m, rowBranch, "feature/live")
	m = press(m, "backspace")
	dialog := strings.Join(strings.Fields(strings.ReplaceAll(ansi.Strip(opened[*deleteConfirmation](t, m).render(m.theme)), "│", " ")), " ")
	for _, want := range []string{`Delete the "feature/live" branch and its 2 tasks?`, "1 open, 1 done"} {
		if !strings.Contains(dialog, want) {
			t.Errorf("dialog lacks %q: %s", want, dialog)
		}
	}
	m = press(m, "y")
	want := "- [x] Loose done\n\n# Branches\n\n## feature/gone\n\n- [ ] Gone open\n\n- [x] Gone done\n"
	if got := fileContent(t, path); got != want || m.status != `Deleted the "feature/live" branch and its 2 tasks` {
		t.Fatalf("file = %q, status %q", got, m.status)
	}
	// A branch Git doesn't have goes the same way, and the last one takes
	// the Branches heading with it.
	selectGroup(t, m, rowBranch, "feature/gone")
	m = press(press(m, "backspace"), "y")
	if got := fileContent(t, path); got != "- [x] Loose done\n" || len(m.rows(branchPane)) != 0 {
		t.Fatalf("file = %q, rows %+v", got, m.rows(branchPane))
	}
}

func TestDeleteLeavesAFileChangedSinceAlone(t *testing.T) {
	m, path := clearModel(t)
	selectTask(t, m, "Loose open")
	m = press(m, "backspace")
	const elsewhere = "- [ ] Loose open\n\n- [ ] Written elsewhere\n"
	if err := os.WriteFile(path, []byte(elsewhere), 0644); err != nil {
		t.Fatal(err)
	}
	m = press(m, "y")
	if got := fileContent(t, path); got != elsewhere || !strings.Contains(m.status, "changed") || m.lastRemoval != nil {
		t.Fatalf("file = %q, status %q", got, m.status)
	}
}

func TestDeletingOneOfIdenticalTasksLeavesTheOtherInPlace(t *testing.T) {
	const content = "- [ ] scan\n\n- [ ] list\n\n- [ ] scan\n\n- [ ] need\n"
	for _, item := range []struct {
		name string
		row  int
		file string
		rows []string
	}{
		{"upper", 0, "- [ ] list\n\n- [ ] scan\n\n- [ ] need\n", []string{"list:0", "scan:2", "need:4"}},
		{"lower", 2, "- [ ] scan\n\n- [ ] list\n\n- [ ] need\n", []string{"scan:0", "list:2", "need:4"}},
	} {
		t.Run(item.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			m, err := newModel(path, project.Repo{}, testFiles())
			if err != nil {
				t.Fatal(err)
			}
			before := taskOrder(m)
			m.general.cursor = item.row
			m = press(press(m, "backspace"), "y")
			if got := fileContent(t, path); got != item.file {
				t.Fatalf("file = %q, want %q", got, item.file)
			}
			if got := taskOrder(m); !slices.Equal(got, item.rows) {
				t.Fatalf("rows after the delete = %v, want %v", got, item.rows)
			}
			m = press(m, "u")
			if got := taskOrder(m); !slices.Equal(got, before) {
				t.Fatalf("rows after the undo = %v, want %v as before", got, before)
			}
		})
	}
}
