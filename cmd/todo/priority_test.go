package main

import (
	"strings"
	"testing"
)

func TestParsePriority(t *testing.T) {
	for _, item := range []struct {
		input string
		want  priority
		err   bool
	}{
		{"", priorityNone, false},
		{" High ", priorityHigh, false},
		{"MEDIUM", priorityMedium, false},
		{"low", priorityLow, false},
		{"urgent", priorityNone, true},
	} {
		t.Run(item.input, func(t *testing.T) {
			got, err := parsePriority(item.input)
			if got != item.want || (err != nil) != item.err {
				t.Fatalf("parsePriority(%q) = %q, %v", item.input, got, err)
			}
		})
	}
}

func TestPriorityCyclesAndTitles(t *testing.T) {
	p := priorityNone
	var seen []string
	for range 4 {
		p = p.next()
		seen = append(seen, p.title())
	}
	if got := strings.Join(seen, ","); got != "High,Medium,Low," {
		t.Fatalf("priority cycle = %q", got)
	}
	if priority("urgent").next() != priorityNone || priority("urgent").rank() != priorityNone.rank() {
		t.Fatal("unknown priority should behave like no priority")
	}
}
