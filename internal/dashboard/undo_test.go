package dashboard

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nebarg/todo-cli/internal/project"
)

var (
	ctrlZ      = tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
	ctrlShiftZ = tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl | tea.ModShift}
	cmdZ       = tea.KeyPressMsg{Code: 'z', Mod: tea.ModSuper}
	cmdShiftZ  = tea.KeyPressMsg{Code: 'z', Mod: tea.ModSuper | tea.ModShift}
	backspace  = tea.KeyPressMsg{Code: tea.KeyBackspace}
)

// editFormModel opens the edit form on a task with three lines of details.
func editFormModel(t *testing.T) *model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte("- [ ] Fix login\n\n  one\n  two\n  three\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Repo{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	return press(m, "e")
}

func TestTaskFormUndoesAndRedoesRunsOfTyping(t *testing.T) {
	m := editFormModel(t)
	f := form(t, m)
	title := func(want string, cursor int) {
		t.Helper()
		if got := textareaState(f.title); got != (textState{want, cursor}) {
			t.Fatalf("title = %+v, want %q with the cursor at %d", got, want, cursor)
		}
	}

	// A run of typing, and a run of deleting however long, are a step each.
	m = typeText(m, " now")
	for range len("Fix login now") {
		m = pressMsg(m, backspace)
	}
	title("", 0)
	m = pressMsg(m, ctrlZ)
	title("Fix login now", 13)
	m = pressMsg(m, ctrlZ)
	title("Fix login", 9)
	m = pressMsg(m, ctrlZ)
	title("Fix login", 9) // nothing left to undo

	m = pressMsg(m, ctrlShiftZ)
	title("Fix login now", 13)
	m = pressMsg(m, ctrlShiftZ)
	title("", 0)
	m = pressMsg(m, ctrlShiftZ)
	title("", 0) // nothing left to redo

	// A new change drops what was undone.
	m = typeText(pressMsg(m, ctrlZ), "!")
	m = pressMsg(m, ctrlShiftZ)
	title("Fix login now!", 14)

	// Moving the cursor ends a run, and undo puts the cursor back.
	m = pressMsg(typeText(m, "ab"), tea.KeyPressMsg{Code: tea.KeyLeft})
	m = typeText(m, "c")
	m = pressMsg(m, ctrlZ)
	title("Fix login now!ab", 15)

	// A paste is a step of its own.
	updated, _ := m.Update(tea.PasteMsg{Content: "pasted"})
	m = typeText(updated.(*model), "x")
	m = pressMsg(m, ctrlZ)
	title("Fix login now!apastedb", 21)
	m = pressMsg(m, ctrlZ)
	title("Fix login now!ab", 15)

	// esc still discards the lot.
	if m = press(m, "esc"); isOpen[*taskModal](m) {
		t.Fatal("esc did not close the form")
	}
}

func TestTaskFormUndoesEachFieldOnItsOwn(t *testing.T) {
	m := editFormModel(t)
	f := form(t, m)
	m = typeText(m, " now")

	// Deleting a line in the middle of Details puts the cursor back there.
	m = pressTab(pressTab(m))
	if f.field != detailsField {
		t.Fatalf("tab twice focused field %d, not Details", f.field)
	}
	m = pressMsg(m, tea.KeyPressMsg{Code: tea.KeyUp})
	for range len("two") {
		m = pressMsg(m, backspace)
	}
	if got := f.details.Value(); got != "one\n\nthree" {
		t.Fatalf("details after deleting = %q", got)
	}
	m = pressMsg(m, cmdZ)
	if got := f.details.Value(); got != "one\ntwo\nthree" || f.details.Line() != 1 || f.details.Column() != 3 {
		t.Fatalf("details after undo = %q, cursor at line %d column %d", got, f.details.Line(), f.details.Column())
	}

	// Details has nothing more to undo; the title's change stays.
	m = pressMsg(m, cmdZ)
	if f.details.Value() != "one\ntwo\nthree" || f.title.Value() != "Fix login now" {
		t.Fatalf("undo reached past Details: details %q, title %q", f.details.Value(), f.title.Value())
	}
	m = pressMsg(m, cmdShiftZ)
	if got := f.details.Value(); got != "one\n\nthree" {
		t.Fatalf("details after redo = %q", got)
	}

	// Coming back to a field starts a new step.
	m = pressTab(m)
	pressMsg(typeText(m, "!"), ctrlZ)
	if got := f.title.Value(); got != "Fix login now" {
		t.Fatalf("title after undo = %q, want the typing before leaving it kept", got)
	}
}

func TestCategoryPromptUndoesAndRedoes(t *testing.T) {
	m, _ := categoriesModel(t)
	m.general.cursor = 3
	m = pressTab(typeText(press(m, "c"), "My"))
	p := prompt(t, m)
	for _, step := range []struct {
		key  tea.KeyPressMsg
		want string
	}{
		{cmdZ, "My"}, // the completion was a step of its own
		{ctrlZ, ""},
		{ctrlShiftZ, "My"},
		{cmdShiftZ, "my category"},
	} {
		if m = pressMsg(m, step.key); p.input.Value() != step.want {
			t.Fatalf("after %s the prompt holds %q, want %q", step.key, p.input.Value(), step.want)
		}
	}
}

func TestEditKindTellsInsertingFromDeleting(t *testing.T) {
	for _, c := range []struct {
		before, after string
		want          editKind
	}{
		{"caf", "café", editInsert},
		{"cafe", "café", editReplace},
		{"aa", "aaa", editInsert},
		{"naïve", "nave", editDelete},
		{"one\ntwo", "one\n", editDelete},
		{"abcdef", "axef", editReplace},
		{"main", "f", editReplace},
	} {
		if got := editKindOf(c.before, c.after); got != c.want {
			t.Errorf("editKindOf(%q, %q) = %d, want %d", c.before, c.after, got, c.want)
		}
	}
}
