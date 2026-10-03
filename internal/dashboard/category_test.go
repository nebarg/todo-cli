package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
)

const categoriesFile = "- [ ] Loose task\n\n# my category\n\n- [ ] A\n\n# My Category Two\n\n- [ ] B\n\n# auth\n\n- [ ] C\n"

func categoriesModel(t *testing.T) (*model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte(categoriesFile), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 100, 24
	return m, path
}

func typeText(m *model, text string) *model {
	for _, r := range text {
		updated, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(*model)
	}
	return m
}

func pressTab(m *model) *model {
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	return updated.(*model)
}

func pressCtrl(m *model, r rune) *model {
	updated, _ := m.Update(tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl})
	return updated.(*model)
}

func TestCategoryNamesAreGeneralsCategories(t *testing.T) {
	m, _ := categoriesModel(t)
	if got := strings.Join(m.categoryNames(), ","); got != "auth,my category,My Category Two" {
		t.Fatalf("category names = %q", got)
	}
}

func TestCategoryPromptCompletesExistingCategories(t *testing.T) {
	m, path := categoriesModel(t)
	m.general.cursor = 3
	if row, ok := m.selectedNavigationRow(); !ok || row.todo.Text != "Loose task" {
		t.Fatalf("the loose task is not the fourth row: %+v", m.rows(generalPane))
	}
	m = typeText(press(m, "c"), "My")
	footer := ansi.Strip(m.renderFooter(100))
	if !strings.HasPrefix(footer, "Category: My category ") || !strings.Contains(footer, "tab complete") {
		t.Fatalf("prompt does not suggest the category: %q", footer)
	}

	m = pressTab(m)
	if got := prompt(t, m).input.Value(); got != "my category" {
		t.Fatalf("tab completed to %q, want the category as written", got)
	}
	if footer := ansi.Strip(m.renderFooter(100)); strings.Contains(footer, "tab complete") {
		t.Fatalf("a completed category still offers completion: %q", footer)
	}

	m = pressCtrl(m, 'n')
	if footer := ansi.Strip(m.renderFooter(100)); !strings.HasPrefix(footer, "Category: my category Two ") {
		t.Fatalf("ctrl+n did not suggest the next category: %q", footer)
	}
	m = press(pressTab(m), "enter")
	if isOpen[*categoryPrompt](m) {
		t.Fatalf("completed category was not saved: %q", prompt(t, m).err)
	}
	if got := readFile(t, path); !strings.Contains(got, "# My Category Two\n\n- [ ] B\n\n- [ ] Loose task\n") {
		t.Fatalf("task was not filed under the completed category:\n%s", got)
	}
}

func TestPromptTabWithoutACompletionDoesNothing(t *testing.T) {
	m, _ := categoriesModel(t)
	m.general.cursor = 3
	m = pressTab(typeText(press(m, "c"), "x"))
	if got := prompt(t, m).input.Value(); got != "x" {
		t.Fatalf("tab changed an unmatched category to %q", got)
	}
}

func TestTaskFormTabCompletesACategoryThenMovesOn(t *testing.T) {
	m, _ := categoriesModel(t)
	m = pressTab(press(m, "a"))
	f := form(t, m)
	if f.field != scopeField {
		t.Fatal("tab did not reach the category field")
	}
	m = typeText(m, "a")
	if got := ansi.Strip(f.scope.View()); !strings.HasPrefix(got, "auth") {
		t.Fatalf("category field does not suggest auth: %q", got)
	}
	if footer := ansi.Strip(f.footer(m.theme, 80)); !strings.HasPrefix(footer, "tab complete") {
		t.Fatalf("form footer = %q", footer)
	}
	m = pressTab(m)
	if f.scope.Value() != "auth" || f.field != scopeField {
		t.Fatalf("tab gave %q in field %d, want auth still in the category field", f.scope.Value(), f.field)
	}
	if footer := ansi.Strip(f.footer(m.theme, 80)); !strings.Contains(footer, "tab next field") {
		t.Fatalf("completed form footer = %q", footer)
	}
	m = pressTab(m)
	if f.field != detailsField {
		t.Fatalf("second tab did not move on: field %d", f.field)
	}

	m = pressTab(press(press(m, "esc"), "a"))
	m = pressTab(typeText(m, "x"))
	if f := form(t, m); f.scope.Value() != "x" || f.field != detailsField {
		t.Fatalf("tab without a completion gave %q in field %d", f.scope.Value(), f.field)
	}
}

func TestEditFormDoesNotCompleteAFilledCategory(t *testing.T) {
	m, _ := categoriesModel(t)
	m.general.cursor = 1
	m = press(press(m, "enter"), "e")
	f := form(t, m)
	if f.scope.Value() != "my category" {
		t.Fatalf("edit form category = %q", f.scope.Value())
	}
	pressTab(pressTab(m))
	if f.field != detailsField || f.scope.Value() != "my category" {
		t.Fatalf("tab completed a category already filled in: %q, field %d", f.scope.Value(), f.field)
	}
}
