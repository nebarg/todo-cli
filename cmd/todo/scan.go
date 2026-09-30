package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
)

var todoMarker = regexp.MustCompile(`(?i)(?:^|[^[:alnum:]_-])@?todo(?:$|[[:space:]:({])`)

type sourceTodo struct {
	path string
	line int
	text string
}

type ripgrepEvent struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
	} `json:"data"`
}

func scanSource(dir string, limit int, allFiles bool) ([]sourceTodo, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := exec.LookPath("rg"); err == nil {
		matches, scanErr := scanWithRipgrep(ctx, dir, limit, allFiles)
		if scanErr == nil {
			return matches, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return scanBuiltIn(ctx, dir, limit, allFiles)
}

func scanWithRipgrep(ctx context.Context, dir string, limit int, allFiles bool) ([]sourceTodo, error) {
	args := []string{"--json", "--line-number"}
	if allFiles {
		args = append(args, "--hidden", "--no-ignore", "--glob", "!.git/")
	} else {
		// The default code view honors ignore rules and leaves out Markdown.
		args = append(args, "--glob", "!*.md", "--glob", "!*.markdown")
	}
	args = append(args, todoMarker.String(), ".")
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = dir
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var matches []sourceTodo
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var event ripgrepEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.Type != "match" || event.Data.Path.Text == "" {
			continue
		}
		if !allFiles && !hasTodoComment(event.Data.Lines.Text) {
			continue
		}
		matches = append(matches, sourceTodo{
			path: strings.TrimPrefix(event.Data.Path.Text, "./"),
			line: event.Data.LineNumber,
			text: strings.TrimSpace(event.Data.Lines.Text),
		})
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep: %s", strings.TrimSpace(stderr.String()))
	}
	return sortedMatches(matches, limit), nil
}

// sortedMatches orders matches by file and line before applying limit, so
// parallel scans always return the same results.
func sortedMatches(matches []sourceTodo, limit int) []sourceTodo {
	slices.SortFunc(matches, func(a, b sourceTodo) int {
		return cmp.Or(strings.Compare(a.path, b.path), cmp.Compare(a.line, b.line))
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func hasTodoComment(line string) bool {
	var quote byte
	escaped := false
	for i := 0; i < len(line); i++ {
		char := line[i]
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case char == '\\' && quote != '`':
				escaped = true
			case char == quote:
				quote = 0
			}
			continue
		}
		if char == '"' || char == '\'' || char == '`' {
			quote = char
			continue
		}
		if strings.HasPrefix(line[i:], "//") || strings.HasPrefix(line[i:], "/*") ||
			strings.HasPrefix(line[i:], "<!--") || strings.HasPrefix(line[i:], "--") || char == '#' ||
			(strings.TrimSpace(line[:i]) == "" && (char == '*' || char == ';' || char == '%')) {
			return todoMarker.MatchString(line[i:])
		}
	}
	return false
}
