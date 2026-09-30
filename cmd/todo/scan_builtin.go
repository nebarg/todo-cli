package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// scanBuiltIn keeps file TODOs usable when ripgrep is not installed.
func scanBuiltIn(ctx context.Context, dir string, limit int, allFiles bool) ([]sourceTodo, error) {
	files, err := sourceFiles(ctx, dir, allFiles)
	if err != nil {
		return nil, err
	}
	jobs := make(chan string, 128)
	found := make(chan sourceTodo, 128)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), 8) {
		wg.Go(func() {
			for path := range jobs {
				if ctx.Err() != nil {
					return
				}
				scanFile(ctx, dir, path, allFiles, found)
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

	var matches []sourceTodo
	for match := range found {
		matches = append(matches, match)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sortedMatches(matches, limit), nil
}

func sourceFiles(ctx context.Context, dir string, allFiles bool) ([]string, error) {
	if !allFiles {
		cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
		if output, err := cmd.Output(); err == nil {
			var files []string
			for raw := range bytes.SplitSeq(output, []byte{0}) {
				if len(raw) == 0 {
					continue
				}
				path := filepath.Clean(string(raw))
				if shouldScanPath(path, false) {
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
			if entry.Name() == ".git" || (!allFiles && skipDefaultDir(entry.Name())) {
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

func skipDefaultDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "dist", "build", "target", "coverage", "venv", "__pycache__":
		return true
	}
	return false
}

func shouldScanPath(path string, allFiles bool) bool {
	slash := filepath.ToSlash(path)
	if slash == ".git" || strings.HasPrefix(slash, ".git/") {
		return false
	}
	if allFiles {
		return true
	}
	for part := range strings.SplitSeq(slash, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	extension := strings.ToLower(filepath.Ext(path))
	return extension != ".md" && extension != ".markdown"
}

func scanFile(ctx context.Context, dir, relative string, allFiles bool, found chan<- sourceTodo) {
	path := filepath.Join(dir, relative)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }() // Read-only, so a close error cannot lose data.
	probe := make([]byte, 8192)
	n, _ := f.Read(probe)
	if bytes.IndexByte(probe[:n], 0) >= 0 {
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		if ctx.Err() != nil {
			return
		}
		text := scanner.Text()
		if !todoMarker.MatchString(text) || (!allFiles && !hasTodoComment(text)) {
			continue
		}
		match := sourceTodo{path: filepath.ToSlash(relative), line: line, text: strings.TrimSpace(text)}
		select {
		case found <- match:
		case <-ctx.Done():
			return
		}
	}
}
