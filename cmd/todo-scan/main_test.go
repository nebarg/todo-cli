package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nebarg/todo-cli/internal/scan"
	"github.com/spf13/pflag"
)

func TestParseArgs(t *testing.T) {
	cwd := t.TempDir()
	sub := filepath.Join(cwd, "web")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cwd, "notes.txt")
	if err := os.WriteFile(file, nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		argv    string
		dir     string
		exclude scan.Exclude
	}{
		{"", cwd, scan.Exclude{Names: []string{"node_modules", "vendor"}}},
		{"web", sub, scan.Exclude{Names: []string{"node_modules", "vendor"}}},
		{"-e dist --exclude web/gen web", sub, scan.Exclude{Names: []string{"dist"}, Paths: []string{"gen"}}},
		{sub, sub, scan.Exclude{Names: []string{"node_modules", "vendor"}}},
	} {
		t.Run(c.argv, func(t *testing.T) {
			dir, exclude, err := parseArgs(cwd, strings.Fields(c.argv))
			if err != nil || dir != c.dir || !reflect.DeepEqual(exclude, c.exclude) {
				t.Fatalf("parseArgs = %q, %+v, %v; want %q, %+v", dir, exclude, err, c.dir, c.exclude)
			}
		})
	}
	for _, c := range []struct {
		argv  string
		usage bool
	}{
		{"a b", true},
		{"--wat", true},
		{"missing", false},
		{"notes.txt", false},
	} {
		_, _, err := parseArgs(cwd, strings.Fields(c.argv))
		if _, isUsage := errors.AsType[usageError](err); err == nil || isUsage != c.usage {
			t.Errorf("parseArgs(%q) = %v, usage error %v", c.argv, err, isUsage)
		}
	}
	if _, _, err := parseArgs(cwd, []string{"--help"}); !errors.Is(err, pflag.ErrHelp) {
		t.Fatalf("--help = %v", err)
	}
}
