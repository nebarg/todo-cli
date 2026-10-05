package scan

import (
	"bytes"
	"cmp"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// scanFiles reads each file walkFiles finds under dir for its to-dos. Files
// are read in parallel while the walk is still finding them.
func scanFiles(ctx context.Context, dir string, exclude Exclude) ([]Match, error) {
	jobs := make(chan string, 128)
	found := make(chan []Match, 128)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), 8) {
		wg.Go(func() {
			for path := range jobs {
				if ctx.Err() != nil {
					return
				}
				if matches := scanFile(dir, path); len(matches) > 0 {
					found <- matches
				}
			}
		})
	}
	var walkErr error
	wg.Go(func() {
		defer close(jobs)
		walkErr = walkFiles(ctx, dir, exclude, func(path string) {
			select {
			case jobs <- path:
			case <-ctx.Done():
			}
		})
	})
	go func() { wg.Wait(); close(found) }()

	var matches []Match
	for fileMatches := range found {
		matches = append(matches, fileMatches...)
	}
	if err := cmp.Or(ctx.Err(), walkErr); err != nil {
		return nil, err
	}
	return sortedMatches(matches), nil
}

// walkFiles adds each file under dir to read, relative to dir. It skips
// excluded directories, Markdown, and whatever a repository's gitignores,
// .git/info/exclude or the user's global gitignore ignore.
func walkFiles(ctx context.Context, dir string, exclude Exclude, add func(string)) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	w := walker{ctx: ctx, dir: dir, exclude: exclude, add: add}
	if content, err := os.ReadFile(globalIgnoreFile()); err == nil {
		w.global = parseIgnore(string(content), 0)
	}
	return w.walk(".", w.enclosingScope())
}

// walker finds the files to read under dir.
type walker struct {
	ctx     context.Context
	dir     string
	exclude Exclude
	global  []ignoreRule // the user's global gitignore, which applies in every repository
	add     func(path string)
}

// walk adds the files in rel, a directory, and below. scope is the
// directory's place in its repository, before its own entries are seen.
func (w *walker) walk(rel string, scope ignoreScope) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(w.dir, rel)
	entries, err := os.ReadDir(path)
	if err != nil {
		if rel == "." {
			return err
		}
		return nil // An unreadable directory is skipped, as unreadable files are.
	}
	if hasEntry(entries, ".git") {
		scope = w.repoScope(path)
	}
	if scope.inRepo && hasEntry(entries, ".gitignore") {
		scope.rules = withIgnoreFile(scope.rules, filepath.Join(path, ".gitignore"), len(scope.segments))
	}
	segments := slices.Concat(scope.segments, []string{""})
	for _, entry := range entries {
		name := entry.Name()
		segments[len(segments)-1] = name
		switch child := filepath.Join(rel, name); {
		case entry.IsDir():
			if w.exclude.skipsDir(child) || scope.ignores(segments, true) {
				continue
			}
			sub := scope
			sub.segments = segments
			if err := w.walk(child, sub); err != nil {
				return err
			}
		case entry.Type().IsRegular() && !isMarkdown(name) && !scope.ignores(segments, false):
			w.add(child)
		}
	}
	return nil
}

// repoScope starts the scope of a repository at its root.
func (w *walker) repoScope(root string) ignoreScope {
	return ignoreScope{
		inRepo:   true,
		foldCase: ignoresCase(root),
		rules:    withIgnoreFile(w.global, filepath.Join(root, ".git", "info", "exclude"), 0),
	}
}

// enclosingScope finds the repository the scanned directory is in, with the
// rules of the gitignores above it. The directory itself is scanned even if
// they ignore it.
func (w *walker) enclosingScope() ignoreScope {
	var segments []string // the scanned directory's path from parent
	for child, parent := w.dir, filepath.Dir(w.dir); parent != child; child, parent = parent, filepath.Dir(parent) {
		segments = slices.Insert(segments, 0, filepath.Base(child))
		if _, err := os.Lstat(filepath.Join(parent, ".git")); err != nil {
			continue
		}
		scope := w.repoScope(parent)
		for depth := range len(segments) {
			gitignore := filepath.Join(parent, filepath.Join(segments[:depth]...), ".gitignore")
			scope.rules = withIgnoreFile(scope.rules, gitignore, depth)
		}
		scope.segments = segments
		return scope
	}
	return ignoreScope{}
}

// hasEntry reports whether a directory's entries, sorted as os.ReadDir
// sorts them, include name.
func hasEntry(entries []os.DirEntry, name string) bool {
	_, found := slices.BinarySearchFunc(entries, name, func(e os.DirEntry, name string) int {
		return strings.Compare(e.Name(), name)
	})
	return found
}

func isMarkdown(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".md" || extension == ".markdown"
}

// maxFileSize is the largest file read for to-dos. Larger files are data,
// bundles or generated code, which are read whole, so they are skipped.
const maxFileSize = 1 << 20

// binaryCheckLen is how much of a file is checked for a NUL byte, which
// marks it as binary.
const binaryCheckLen = 8192

// scanFile finds the to-dos in one file, skipping anything that isn't a
// regular text file of at most maxFileSize bytes, or that can't be read.
func scanFile(dir, relative string) []Match {
	path := filepath.Join(dir, relative)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileSize {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(content[:min(len(content), binaryCheckLen)], 0) >= 0 {
		return nil
	}
	return fileTodos(filepath.ToSlash(relative), string(content))
}
