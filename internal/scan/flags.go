package scan

import (
	"errors"
	"path/filepath"
	"strings"
)

// DefaultExcludes are skipped when no --exclude is given.
var DefaultExcludes = []string{"node_modules", "vendor"}

// ParseExclude turns --exclude values into directories to skip under dir. A
// bare name skips that directory at any depth; anything with a slash is a
// path from cwd, and one outside dir has nothing to skip. Without any
// values, the defaults apply.
func ParseExclude(cwd, dir string, values []string) (Exclude, error) {
	if len(values) == 0 {
		values = DefaultExcludes
	}
	var exclude Exclude
	for _, value := range values {
		name := strings.TrimRight(filepath.ToSlash(value), "/")
		if name == "" {
			return Exclude{}, errors.New("--exclude needs a directory name or path")
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
