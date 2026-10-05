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
