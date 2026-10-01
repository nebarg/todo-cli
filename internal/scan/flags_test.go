package scan

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseExclude(t *testing.T) {
	root := filepath.FromSlash("/repo")
	sub := filepath.Join(root, "web")
	for _, c := range []struct {
		name     string
		cwd, dir string
		values   []string
		want     Exclude
	}{
		{"defaults without flags", root, root, nil, Exclude{Names: []string{"node_modules", "vendor"}}},
		{"flags replace the defaults", root, root, []string{"dist/", "build"}, Exclude{Names: []string{"dist", "build"}}},
		{"paths are from the working directory", sub, sub, []string{"./gen", "src/out/"}, Exclude{Paths: []string{"gen", "src/out"}}},
		{"paths are made relative to a scanned directory elsewhere", root, sub, []string{"web/gen", "other/gen", "..", "."}, Exclude{Paths: []string{"gen"}}},
		{"absolute paths", sub, sub, []string{filepath.Join(sub, "gen")}, Exclude{Paths: []string{"gen"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseExclude(c.cwd, c.dir, c.values)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
	if _, err := ParseExclude(root, root, []string{"/"}); err == nil {
		t.Fatal("an empty exclude was accepted")
	}
}
