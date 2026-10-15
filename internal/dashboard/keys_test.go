package dashboard

import (
	"reflect"
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/nebarg/todo-cli/internal/ui"
)

func TestEachKeyIsBoundOnce(t *testing.T) {
	for _, keymap := range []any{keys, formKeys} {
		boundTo := map[string]string{}
		bindings := reflect.ValueOf(keymap)
		for i := range bindings.NumField() {
			name, binding := bindings.Type().Field(i).Name, bindings.Field(i).Interface().(key.Binding)
			if len(binding.Keys()) == 0 {
				t.Errorf("%s has no keys", name)
			}
			for _, k := range binding.Keys() {
				if other, bound := boundTo[k]; bound {
					t.Errorf("%q is bound to both %s and %s", k, other, name)
				}
				boundTo[k] = name
			}
		}
	}
}

// TestHelpRowsWrittenByHandMatchTheirBindings pins the rows of the help that
// show several keys compactly, so can't be built from their bindings, to the
// keys those bindings have. Rebinding a key fails it until the row shows the
// new key too.
func TestHelpRowsWrittenByHandMatchTheirBindings(t *testing.T) {
	rows := slices.Concat(helpNavigate.rows, helpTasks.rows, helpApp)
	for _, c := range []struct {
		shown string                 // the row's keys, as the help writes them
		bound map[string]key.Binding // each key the row shows, with the binding that has it
	}{
		{"↑↓", map[string]key.Binding{"up": keys.Up, "down": keys.Down}},
		{"⇧↑↓ K J", map[string]key.Binding{"shift+up": keys.PrevTask, "K": keys.PrevTask, "shift+down": keys.NextTask, "J": keys.NextTask}},
		{"→ enter", map[string]key.Binding{"right": keys.Open, "enter": keys.Enter}},
		{"← esc", map[string]key.Binding{"left": keys.Back, "esc": keys.Back}},
		{"h j k l", map[string]key.Binding{"h": keys.Back, "j": keys.Down, "k": keys.Up, "l": keys.Open}},
		{"→ a", map[string]key.Binding{"right": keys.Open, "a": keys.Add}},
		{"X / u", map[string]key.Binding{"X": keys.Clear, "u": keys.Undo}},
	} {
		if !slices.ContainsFunc(rows, func(row ui.KeyHint) bool { return row.Key == c.shown }) {
			t.Errorf("the help has no row of the keys %q", c.shown)
		}
		for k, binding := range c.bound {
			if !slices.Contains(binding.Keys(), k) {
				t.Errorf("the help row %q shows %q, which its binding no longer has: %q", c.shown, k, binding.Keys())
			}
		}
	}
}
