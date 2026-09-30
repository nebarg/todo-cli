package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSource(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // Force the built-in scanner.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("// @todo fix this\nconst name = \"todo scan\"\n// TODO: test it\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TODO.md"), []byte("- [ ] TODO: stored task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "dep.js"), []byte("// TODO: dependency\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.go"), []byte("// TODO: hidden\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "binary.dat"), []byte{'T', 'O', 'D', 'O', 0, 'x'}, 0644); err != nil {
		t.Fatal(err)
	}
	matches, err := scanSource(dir, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].path != "code.go" || matches[0].line != 1 || matches[1].line != 3 {
		t.Fatalf("matches: %+v", matches)
	}
	all, err := scanSource(dir, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("all-file matches: %+v", all)
	}
}

func TestSortedMatchesOrderBeforeLimit(t *testing.T) {
	matches := []sourceTodo{{path: "b.go", line: 1}, {path: "a.go", line: 9}, {path: "a.go", line: 2}, {path: "c.go", line: 1}}
	got := sortedMatches(matches, 3)
	if len(got) != 3 || got[0] != (sourceTodo{path: "a.go", line: 2}) || got[1].line != 9 || got[2].path != "b.go" {
		t.Fatalf("matches were limited before sorting: %+v", got)
	}
	if got := sortedMatches(nil, 3); len(got) != 0 {
		t.Fatalf("empty scan = %+v", got)
	}
}

func TestBuiltInScanLimitIsDeterministic(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // Force the built-in scanner.
	dir := t.TempDir()
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("// TODO: item\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		matches, err := scanSource(dir, 3, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 3 || matches[0].path != "f00.go" || matches[2].path != "f02.go" {
			t.Fatalf("limited scan picked arbitrary files: %+v", matches)
		}
	}
}
