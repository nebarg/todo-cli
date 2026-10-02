package level

import (
	"cmp"
	"slices"
	"testing"
)

func TestValid(t *testing.T) {
	for _, l := range []string{"0", "1", "9", "00", "0000"} {
		if !Valid(l) {
			t.Errorf("Valid(%q) = false", l)
		}
	}
	for _, l := range []string{"", "10", "01", "11", "x", "0+", "-1"} {
		if Valid(l) {
			t.Errorf("Valid(%q) = true", l)
		}
	}
}

func TestZeros(t *testing.T) {
	for l, want := range map[string]bool{"0": true, "000": true, "": false, "1": false, "01": false, "10": false} {
		if got := Zeros(l); got != want {
			t.Errorf("Zeros(%q) = %v, want %v", l, got, want)
		}
	}
}

func TestRankSortsMostUrgentFirst(t *testing.T) {
	levels := []string{"", "2", "0", "000", "1", "00", "9"}
	slices.SortFunc(levels, func(a, b string) int { return cmp.Compare(Rank(a), Rank(b)) })
	if want := []string{"000", "00", "0", "1", "2", "9", ""}; !slices.Equal(levels, want) {
		t.Fatalf("levels = %q, want %q", levels, want)
	}
}
