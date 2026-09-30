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
