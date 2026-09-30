package scan

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
	matches, err := Source(dir, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].Path != "code.go" || matches[0].Line != 1 || matches[1].Line != 3 {
		t.Fatalf("matches: %+v", matches)
	}
	all, err := Source(dir, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("all-file matches: %+v", all)
	}
}

func TestSortedMatchesOrderBeforeLimit(t *testing.T) {
	matches := []Match{{Path: "b.go", Line: 1}, {Path: "a.go", Line: 9}, {Path: "a.go", Line: 2}, {Path: "c.go", Line: 1}}
	got := sortedMatches(matches, 3)
	if len(got) != 3 || got[0] != (Match{Path: "a.go", Line: 2}) || got[1].Line != 9 || got[2].Path != "b.go" {
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
		matches, err := Source(dir, 3, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 3 || matches[0].Path != "f00.go" || matches[2].Path != "f02.go" {
			t.Fatalf("limited scan picked arbitrary files: %+v", matches)
		}
	}
}
