package scan

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// scanBuiltIn keeps file TODOs usable when ripgrep is not installed.
func scanBuiltIn(ctx context.Context, dir string, limit int, allFiles bool, exclude Exclude) ([]Match, error) {
	files, err := sourceFiles(ctx, dir, allFiles, exclude)
	if err != nil {
		return nil, err
	}
	return scanFiles(ctx, dir, files, limit)
}

// scanFiles reads files, relative to dir, in parallel for their to-dos.
func scanFiles(ctx context.Context, dir string, files []string, limit int) ([]Match, error) {
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
	go func() {
		defer close(jobs)
		for _, path := range files {
			select {
			case jobs <- path:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(found) }()

	var matches []Match
	for fileMatches := range found {
		matches = append(matches, fileMatches...)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sortedMatches(matches, limit), nil
}

func sourceFiles(ctx context.Context, dir string, allFiles bool, exclude Exclude) ([]string, error) {
	if !allFiles {
		cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
		if output, err := cmd.Output(); err == nil {
			var files []string
			for raw := range bytes.SplitSeq(output, []byte{0}) {
				if len(raw) == 0 {
					continue
				}
				path := filepath.Clean(string(raw))
				if shouldScanPath(path, false) && !exclude.skipsFileIn(path) {
					files = append(files, path)
				}
			}
			return files, nil
		}
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if exclude.skipsDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if shouldScanPath(rel, allFiles) {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func shouldScanPath(path string, allFiles bool) bool {
	if allFiles {
		return true
	}
	for part := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	extension := strings.ToLower(filepath.Ext(path))
	return extension != ".md" && extension != ".markdown"
}

// scanFile finds the to-dos in one file, skipping anything that isn't a
// regular text file.
func scanFile(dir, relative string) []Match {
	path := filepath.Join(dir, relative)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(content[:min(len(content), 8192)], 0) >= 0 {
		return nil
	}
	return fileTodos(filepath.ToSlash(relative), string(content))
}
