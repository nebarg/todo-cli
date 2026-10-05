package filesui

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestEachKeyIsBoundOnce(t *testing.T) {
	boundTo := map[string]string{}
	bindings := reflect.ValueOf(keys)
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

func TestHandlesTheKeysTheBrowserActsOn(t *testing.T) {
	letter := func(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }
	for _, c := range []struct {
		name string
		msg  tea.KeyPressMsg
		want bool
	}{
		{"up", tea.KeyPressMsg{Code: tea.KeyUp}, true},
		{"k", letter('k'), true},
		{"down", tea.KeyPressMsg{Code: tea.KeyDown}, true},
		{"j", letter('j'), true},
		{"right", tea.KeyPressMsg{Code: tea.KeyRight}, true},
		{"left", tea.KeyPressMsg{Code: tea.KeyLeft}, true},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEsc}, true},
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}, true},
		{"e", letter('e'), true},
		{"tab", tea.KeyPressMsg{Code: tea.KeyTab}, false},
		{"shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, false},
		{"space", tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, false},
		{"d", letter('d'), false},
		{"i", letter('i'), false},
		{"r", letter('r'), false},
		{"q", letter('q'), false},
		{"1", letter('1'), false},
		{"3", letter('3'), false},
	} {
		if got := Handles(c.msg); got != c.want {
			t.Errorf("Handles(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
