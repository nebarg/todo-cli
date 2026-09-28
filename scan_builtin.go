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
	"sort"
	"strings"
	"sync"
)

// scanBuiltIn keeps file TODOs usable when ripgrep is not installed.
func scanBuiltIn(parent context.Context, dir string, limit int, allFiles bool) ([]sourceTodo, error) {
	files, err := sourceFiles(parent, dir, allFiles)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	jobs := make(chan string, 128)
	found := make(chan sourceTodo, 128)
	workers := runtime.GOMAXPROCS(0)
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if ctx.Err() != nil {
					return
				}
				scanFile(ctx, dir, path, allFiles, found)
			}
		}()
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
	capped := false
	for match := range found {
		if limit == 0 || len(matches) < limit {
			matches = append(matches, match)
		}
		if limit > 0 && len(matches) >= limit && !capped {
			capped = true
			cancel()
		}
	}
	if parent.Err() != nil && !capped {
		return nil, parent.Err()
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].path == matches[j].path {
			return matches[i].line < matches[j].line
		}
		return matches[i].path < matches[j].path
	})
	return matches, nil
}

func sourceFiles(ctx context.Context, dir string, allFiles bool) ([]string, error) {
	if !allFiles {
		cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
		if output, err := cmd.Output(); err == nil {
			var files []string
			for _, raw := range bytes.Split(output, []byte{0}) {
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
	for _, part := range strings.Split(slash, "/") {
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
	defer f.Close()
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
