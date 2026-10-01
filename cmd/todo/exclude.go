package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/nebarg/todo-cli/internal/scan"
)

var defaultExcludes = []string{"node_modules", "vendor"}

// scanExclude turns --exclude values into directories to skip under dir. A
// bare name skips that directory at any depth; anything with a slash is a
// path from cwd, and one outside dir has nothing to skip. Without any
// values, the defaults apply.
func scanExclude(cwd, dir string, values []string) (scan.Exclude, error) {
	if len(values) == 0 {
		values = defaultExcludes
	}
	var exclude scan.Exclude
	for _, value := range values {
		name := strings.TrimRight(filepath.ToSlash(value), "/")
		if name == "" {
			return scan.Exclude{}, errors.New("--exclude needs a directory name or path")
		}
		if !strings.Contains(name, "/") && name != "." && name != ".." {
			exclude.Names = append(exclude.Names, name)
			continue
		}
		target := value
		if !filepath.IsAbs(target) {
			target = filepath.Join(cwd, target)
		}
		rel, err := filepath.Rel(dir, target)
		if err != nil || rel == "." || !filepath.IsLocal(rel) {
			continue
		}
		exclude.Paths = append(exclude.Paths, filepath.ToSlash(rel))
	}
	return exclude, nil
}
