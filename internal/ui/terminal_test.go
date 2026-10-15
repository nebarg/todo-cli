package ui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type quitModel struct{}

func (quitModel) Init() tea.Cmd                       { return tea.Quit }
func (quitModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return quitModel{}, nil }
func (quitModel) View() tea.View                      { return tea.NewView("") }

func TestRunNeedsATerminal(t *testing.T) {
	_, pipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pipe.Close() }()
	file, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	for name, out := range map[string]io.Writer{"pipe": pipe, "file": file, "buffer": &bytes.Buffer{}} {
		if err := Run(quitModel{}, out); !errors.Is(err, ErrNoTerminal) {
			t.Errorf("Run to a %s = %v", name, err)
		}
	}
	if info, err := file.Stat(); err != nil || info.Size() != 0 {
		t.Fatalf("Run wrote to the file: %v, %v", info, err)
	}
}

func TestATerminalIsTooSmallWhenEitherSideIsBelowTheMinimum(t *testing.T) {
	for _, c := range []struct {
		width, height int
		want          bool
	}{
		{56, 16, false},
		{120, 40, false},
		{55, 16, true},
		{56, 15, true},
		{55, 15, true},
		{200, 15, true},
		{55, 60, true},
		{0, 0, true},
	} {
		if got := TooSmall(c.width, c.height); got != c.want {
			t.Errorf("TooSmall(%d, %d) = %v, want %v", c.width, c.height, got, c.want)
		}
	}
}
