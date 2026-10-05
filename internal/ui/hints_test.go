package ui

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestHintShowsABindingsFirstKey(t *testing.T) {
	for _, c := range []struct {
		keys []string
		want string
	}{
		{[]string{"d", "space"}, "d"},
		{[]string{"left", "esc"}, "←"},
		{[]string{"right"}, "→"},
		{[]string{"up", "k"}, "↑"},
		{[]string{"down", "j"}, "↓"},
		{[]string{"backspace"}, "⌫"},
		{[]string{"shift+tab"}, "⇧tab"},
		{[]string{"tab"}, "tab"},
		{[]string{"ctrl+enter"}, "ctrl+enter"},
		{[]string{"X"}, "X"},
	} {
		got := Hint(key.NewBinding(key.WithKeys(c.keys...)), "do it")
		if got != (KeyHint{Key: c.want, Label: "do it"}) {
			t.Errorf("Hint(%q) = %+v, want key %q", c.keys, got, c.want)
		}
	}
}

func TestKeyTextShowsEveryKeyOfTheBindings(t *testing.T) {
	binding := func(keys ...string) key.Binding { return key.NewBinding(key.WithKeys(keys...)) }
	for _, c := range []struct {
		name     string
		bindings []key.Binding
		want     string
	}{
		{"none", nil, ""},
		{"one binding's keys", []key.Binding{binding("d", "space")}, "d space"},
		{"symbols", []key.Binding{binding("left", "esc")}, "← esc"},
		{"several bindings", []key.Binding{binding("tab"), binding("shift+tab")}, "tab ⇧tab"},
		{"a symbol then a letter", []key.Binding{binding("right"), binding("a")}, "→ a"},
		{"three bindings", []key.Binding{binding("1"), binding("2"), binding("3")}, "1 2 3"},
	} {
		if got := KeyText(c.bindings...); got != c.want {
			t.Errorf("%s: KeyText = %q, want %q", c.name, got, c.want)
		}
	}
}
