package main

import (
	"strings"
	"testing"
)

func TestSplitCategoryArg(t *testing.T) {
	for _, item := range []struct {
		name     string
		args     []string
		category string
		rest     string
	}{
		{"leading category", []string{"@boundary", "Fix", "the", "thing"}, "@boundary", "Fix the thing"},
		{"no category", []string{"Fix", "it"}, "", "Fix it"},
		{"later at sign stays in the title", []string{"Email", "@sam"}, "", "Email @sam"},
		{"bare at sign is text", []string{"@", "later"}, "", "@ later"},
		{"category only", []string{"@boundary"}, "@boundary", ""},
		{"empty", nil, "", ""},
	} {
		t.Run(item.name, func(t *testing.T) {
			category, rest := splitCategoryArg(item.args)
			if category != item.category || strings.Join(rest, " ") != item.rest {
				t.Fatalf("splitCategoryArg(%q) = %q, %q", item.args, category, rest)
			}
		})
	}
}
