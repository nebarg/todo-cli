package store

import (
	"strings"
	"testing"
)

func TestParsePriority(t *testing.T) {
	for _, item := range []struct {
		input string
		want  Priority
		err   bool
	}{
		{"", PriorityNone, false},
		{" High ", PriorityHigh, false},
		{"MEDIUM", PriorityMedium, false},
		{"low", PriorityLow, false},
		{"urgent", PriorityNone, true},
	} {
		t.Run(item.input, func(t *testing.T) {
			got, err := ParsePriority(item.input)
			if got != item.want || (err != nil) != item.err {
				t.Fatalf("parsePriority(%q) = %q, %v", item.input, got, err)
			}
		})
	}
}

func TestPriorityCyclesAndTitles(t *testing.T) {
	p := PriorityNone
	var seen []string
	for range 4 {
		p = p.Next()
		seen = append(seen, p.Title())
	}
	if got := strings.Join(seen, ","); got != "High,Medium,Low," {
		t.Fatalf("priority cycle = %q", got)
	}
	if Priority("urgent").Next() != PriorityNone || Priority("urgent").Rank() != PriorityNone.Rank() {
		t.Fatal("unknown priority should behave like no priority")
	}
}

func TestSplitPriority(t *testing.T) {
	for _, item := range []struct {
		input, title string
		want         Priority
	}{
		{"Fix login !high", "Fix login", PriorityHigh},
		{"Fix login !MEDIUM  ", "Fix login", PriorityMedium},
		{"Fix login\t!low", "Fix login", PriorityLow},
		{"Ship it!", "Ship it!", PriorityNone},
		{"Fix !important CSS", "Fix !important CSS", PriorityNone},
		{"Fix !important", "Fix !important", PriorityNone},
		{"Fix !", "Fix !", PriorityNone},
		{"!high", "!high", PriorityNone},
		{"Fix!high", "Fix!high", PriorityNone},
		{"Keep !high in title !low", "Keep !high in title", PriorityLow},
	} {
		t.Run(item.input, func(t *testing.T) {
			if title, p := SplitPriority(item.input); title != item.title || p != item.want {
				t.Fatalf("SplitPriority(%q) = %q, %q", item.input, title, p)
			}
		})
	}
}
