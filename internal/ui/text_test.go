package ui

import "testing"

func TestPluralWord(t *testing.T) {
	for _, item := range []struct {
		n    int
		want string
	}{
		{0, "tasks"},
		{1, "task"},
		{2, "tasks"},
		{3, "tasks"},
	} {
		if got := PluralWord(item.n, "task", "tasks"); got != item.want {
			t.Errorf("PluralWord(%d) = %q, want %q", item.n, got, item.want)
		}
	}
}

func TestPlural(t *testing.T) {
	for _, item := range []struct {
		n    int
		want string
	}{
		{0, "0 categories"},
		{1, "1 category"},
		{2, "2 categories"},
		{3, "3 categories"},
	} {
		if got := Plural(item.n, "category", "categories"); got != item.want {
			t.Errorf("Plural(%d) = %q, want %q", item.n, got, item.want)
		}
	}
}

func TestJoinNames(t *testing.T) {
	for _, item := range []struct {
		name  string
		names []string
		want  string
	}{
		{"none", nil, ""},
		{"an empty list", []string{}, ""},
		{"one", []string{"a"}, "a"},
		{"two", []string{"a", "b"}, "a and b"},
		{"three", []string{"a", "b", "c"}, "a, b and c"},
		{"four", []string{"a", "b", "c", "d"}, "a, b, c and d"},
	} {
		t.Run(item.name, func(t *testing.T) {
			if got := JoinNames(item.names); got != item.want {
				t.Fatalf("JoinNames(%q) = %q, want %q", item.names, got, item.want)
			}
		})
	}
}
