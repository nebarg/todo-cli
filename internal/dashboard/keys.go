package dashboard

import "charm.land/bubbles/v2/key"

// keyMap is the dashboard's keys, outside the task form and the category
// prompt. A key is in one binding only, so a key press matches at most one of
// them. The footers show a binding's first key.
type keyMap struct {
	Tab1, Tab2, Tab3 key.Binding
	NextTab, PrevTab key.Binding
	Up, Down         key.Binding
	Open             key.Binding // opens a group, or a task's details
	Enter            key.Binding // opens a group, or edits a task
	Back             key.Binding
	AllTasks         key.Binding // opens and closes the All tasks view
	Sort             key.Binding
	Done             key.Binding
	Edit             key.Binding
	Priority         key.Binding
	Category         key.Binding
	Delete           key.Binding
	Add              key.Binding
	AddBranch        key.Binding
	Clear            key.Binding
	Undo             key.Binding
	Reload           key.Binding
	Help             key.Binding
	Quit             key.Binding
}

var keys = keyMap{
	Tab1:      key.NewBinding(key.WithKeys("1")),
	Tab2:      key.NewBinding(key.WithKeys("2")),
	Tab3:      key.NewBinding(key.WithKeys("3")),
	NextTab:   key.NewBinding(key.WithKeys("tab")),
	PrevTab:   key.NewBinding(key.WithKeys("shift+tab")),
	Up:        key.NewBinding(key.WithKeys("up", "k")),
	Down:      key.NewBinding(key.WithKeys("down", "j")),
	Open:      key.NewBinding(key.WithKeys("right")),
	Enter:     key.NewBinding(key.WithKeys("enter")),
	Back:      key.NewBinding(key.WithKeys("left", "esc")),
	AllTasks:  key.NewBinding(key.WithKeys("i")),
	Sort:      key.NewBinding(key.WithKeys("s")),
	Done:      key.NewBinding(key.WithKeys("d", "space")),
	Edit:      key.NewBinding(key.WithKeys("e")),
	Priority:  key.NewBinding(key.WithKeys("p")),
	Category:  key.NewBinding(key.WithKeys("c")),
	Delete:    key.NewBinding(key.WithKeys("backspace")),
	Add:       key.NewBinding(key.WithKeys("a")),
	AddBranch: key.NewBinding(key.WithKeys("b")),
	Clear:     key.NewBinding(key.WithKeys("X")),
	Undo:      key.NewBinding(key.WithKeys("u")),
	Reload:    key.NewBinding(key.WithKeys("r")),
	Help:      key.NewBinding(key.WithKeys("?")),
	Quit:      key.NewBinding(key.WithKeys("q")),
}
