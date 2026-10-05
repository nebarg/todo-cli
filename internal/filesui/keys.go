package filesui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// keyMap is the browser's keys. A key is in one binding only, so a key press
// matches at most one of them.
type keyMap struct {
	Up, Down key.Binding
	Open     key.Binding // opens a category or the detail page
	Enter    key.Binding // opens a category, or the file in an editor
	Edit     key.Binding // opens the file in an editor
	Back     key.Binding
}

var keys = keyMap{
	Up:    key.NewBinding(key.WithKeys("up", "k")),
	Down:  key.NewBinding(key.WithKeys("down", "j")),
	Open:  key.NewBinding(key.WithKeys("right")),
	Enter: key.NewBinding(key.WithKeys("enter")),
	Edit:  key.NewBinding(key.WithKeys("e")),
	Back:  key.NewBinding(key.WithKeys("left", "esc")),
}

// Handles reports whether the browser takes msg, in its list or on its
// detail page, so a caller showing it passes on exactly those keys.
func Handles(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, keys.Up, keys.Down, keys.Open, keys.Enter, keys.Edit, keys.Back)
}
