package ui

import "testing"

func TestWrap(t *testing.T) {
	for _, item := range []struct {
		cursor, step, n, want int
	}{
		{1, 1, 4, 2},
		{2, -1, 4, 1},
		{3, 1, 4, 0},
		{0, -1, 4, 3},
		{0, 1, 1, 0},
		{0, -1, 1, 0},
		{0, 1, 0, 0},
	} {
		if got := Wrap(item.cursor, item.step, item.n); got != item.want {
			t.Errorf("Wrap(%d, %d, %d) = %d, want %d", item.cursor, item.step, item.n, got, item.want)
		}
	}
}
