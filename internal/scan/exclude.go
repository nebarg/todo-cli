package scan

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Exclude lists directories a scan skips, on top of every directory whose
// name starts with a dot. Names skip a directory at any depth; Paths are
// slash-separated and relative to the scanned directory.
type Exclude struct {
	Names []string
	Paths []string
}

func (e Exclude) skipsDir(rel string) bool {
	rel = filepath.ToSlash(rel)
	name := path.Base(rel)
	return strings.HasPrefix(name, ".") || slices.Contains(e.Names, name) || slices.Contains(e.Paths, rel)
}
