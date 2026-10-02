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
func scanBuiltIn(ctx context.Context, dir string, exclude Exclude) ([]Match, error) {
	files, err := sourceFiles(ctx, dir, exclude)
	if err != nil {
		return nil, err
	}
	return scanFiles(ctx, dir, files)
}

// scanFiles reads files, relative to dir, in parallel for their to-dos.
func scanFiles(ctx context.Context, dir string, files []string) ([]Match, error) {
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
	return sortedMatches(matches), nil
}

func sourceFiles(ctx context.Context, dir string, exclude Exclude) ([]string, error) {
	if files, ok := gitFiles(ctx, dir, exclude); ok {
		return files, nil
	}
	return walkFiles(ctx, dir, exclude)
}

// gitFiles lists the files under dir that Git doesn't ignore. ok is false
// outside a repository, and when dir is itself ignored, as Git would then
// list nothing in a directory the user asked to scan.
func gitFiles(ctx context.Context, dir string, exclude Exclude) (files []string, ok bool) {
	// check-ignore exits 0 only when dir is ignored.
	if exec.CommandContext(ctx, "git", "-C", dir, "check-ignore", "-q", ".").Run() == nil {
		return nil, false
	}
	output, err := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".").Output()
	if err != nil {
		return nil, false
	}
	for raw := range bytes.SplitSeq(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		path := filepath.Clean(string(raw))
		if !isMarkdown(path) && !exclude.skipsFileIn(path) {
			files = append(files, path)
		}
	}
	return files, true
}

func walkFiles(ctx context.Context, dir string, exclude Exclude) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			// A directory that can't be read is skipped, as unreadable files
			// are; only the scanned directory itself must be readable.
			if path != dir && entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
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
		if !isMarkdown(rel) {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func isMarkdown(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".md" || extension == ".markdown"
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
