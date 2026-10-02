package scan

import (
	"slices"
	"testing"
)

func TestLevelFilter(t *testing.T) {
	matches := []Match{{Note: "000", Level: "000"}, {Note: "0", Level: "0"}, {Note: "1", Level: "1"}, {Note: "plain"}, {Note: "category", Category: "ui"}}
	if LevelFilter(false, nil) != nil {
		t.Fatal("no levels should keep everything")
	}
	for _, c := range []struct {
		anyLevel bool
		levels   []string
		want     []string
	}{
		{true, nil, []string{"000", "0", "1"}},
		{false, []string{"0"}, []string{"0"}},
		{false, []string{"0+"}, []string{"000", "0"}},
		{false, []string{"00", "1"}, []string{"1"}},
		{true, []string{"1"}, []string{"1"}},
	} {
		keep := LevelFilter(c.anyLevel, c.levels)
		var got []string
		for _, match := range matches {
			if keep(match) {
				got = append(got, match.Note)
			}
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("LevelFilter(%v, %q) kept %q, want %q", c.anyLevel, c.levels, got, c.want)
		}
	}
}

func TestValidLevel(t *testing.T) {
	for _, level := range []string{"0", "9", "00", "0000", "0+"} {
		if !ValidLevel(level) {
			t.Errorf("ValidLevel(%q) = false", level)
		}
	}
	for _, level := range []string{"", "10", "01", "x", "1+", "0*", "+"} {
		if ValidLevel(level) {
			t.Errorf("ValidLevel(%q) = true", level)
		}
	}
}
