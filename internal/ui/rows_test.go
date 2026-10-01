package ui

import "testing"

func TestLongZeroLevelsAreShortened(t *testing.T) {
	for level, want := range map[string]string{"": "", "1": "1", "0": "0", "000": "000", "0000": "0x4", "000000000": "0x9", "0000000000": "0x9+"} {
		if got := LevelLabel(level); got != want {
			t.Errorf("LevelLabel(%q) = %q, want %q", level, got, want)
		}
	}
}
