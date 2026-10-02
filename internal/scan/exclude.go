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

// ripgrepGlobs are the exclusions as ripgrep globs; a trailing slash matches
// only directories, and a leading one anchors a path to the scanned directory.
func (e Exclude) ripgrepGlobs() []string {
	args := []string{"--glob", "!.*/"}
	for _, name := range e.Names {
		args = append(args, "--glob", "!"+escapeGlob(name)+"/")
	}
	for _, p := range e.Paths {
		args = append(args, "--glob", "!/"+escapeGlob(p)+"/")
	}
	return args
}

func escapeGlob(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`*?[]{}\!`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
