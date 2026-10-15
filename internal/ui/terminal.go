package ui

import (
	"errors"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// ErrNoTerminal is returned by Run when out isn't a terminal.
var ErrNoTerminal = errors.New("not a terminal")

// Run runs m full screen on out. It refuses when out isn't a terminal, as
// Bubble Tea would otherwise draw into a pipe or file while waiting for keys.
func Run(m tea.Model, out io.Writer) error {
	if f, ok := out.(*os.File); !ok || !term.IsTerminal(f.Fd()) {
		return ErrNoTerminal
	}
	_, err := tea.NewProgram(m, tea.WithOutput(out)).Run()
	return err
}

// minWidth and minHeight are the smallest terminal, in cells, that the
// dashboard and the file scanner can lay their screens out in.
const (
	minWidth  = 56
	minHeight = 16
)

// TooSmall reports whether a terminal of width × height is smaller than the
// dashboard and the file scanner can lay their screens out in, so they ask
// for a larger one instead.
func TooSmall(width, height int) bool {
	return width < minWidth || height < minHeight
}
