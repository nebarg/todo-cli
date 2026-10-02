package ui

import (
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// ErrNoTerminal is returned by Run when out isn't a terminal.
var ErrNoTerminal = errors.New("not a terminal")

// Run runs m full screen on out. It refuses when out isn't a terminal, as
// Bubble Tea would otherwise draw into a pipe or file while waiting for keys.
func Run(m tea.Model, out *os.File) error {
	if !term.IsTerminal(out.Fd()) {
		return ErrNoTerminal
	}
	_, err := tea.NewProgram(m, tea.WithOutput(out)).Run()
	return err
}
