// Package scan finds to-do marker comments in source code.
package scan

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var todoMarker = regexp.MustCompile(`(?i)(?:^|[^[:alnum:]_-])@?todo(?:[0-9]*@[[:alnum:]_-]+|[0-9]+)?(?:$|[[:space:]:({])`)

// Match is a to-do marker comment found in a source file, with its path relative
// to the scanned directory. Category and Level come from todo-system markers:
// "boundary" for todo@boundary, "00" for todo00. As in todo-system, a to-do
// has one or the other: todo1@boundary is in the boundary category.
type Match struct {
	Path     string
	Line     int
	Text     string
	Note     string
	Category string
	Level    string
}

// newMatch reads the to-do out of a matched line. Note is its text without
// the comment syntax and marker, or the whole line when that leaves nothing.
func newMatch(path string, line int, text string) Match {
	m := Match{Path: path, Line: line, Text: strings.TrimSpace(text)}
	m.Note = m.Text
	if c, ok := commentTodo(m.Text); ok {
		m.Category, m.Level = c.category, c.level
		if c.note != "" {
			m.Note = c.note
		}
	}
	return m
}

// LevelRank orders todo-system priorities: more zeros is more urgent, then
// todo1, todo2 and so on, then to-dos without a level.
func LevelRank(level string) int {
	if level == "" {
		return math.MaxInt
	}
	if strings.Trim(level, "0") == "" {
		return -len(level)
	}
	n, err := strconv.Atoi(level)
	if err != nil {
		return math.MaxInt - 1
	}
	return n
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

// Source finds to-do marker comments under dir, using ripgrep when it is installed.
// Results are sorted by path and line; limit caps them, and 0 means no limit.
// allFiles also searches Markdown, hidden and ignored files. Directories
// in exclude, and those starting with a dot, are always skipped.
func Source(dir string, limit int, allFiles bool, exclude Exclude) ([]Match, error) {
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
		matches, scanErr := scanWithRipgrep(ctx, dir, limit, allFiles, exclude)
		if scanErr == nil {
			return matches, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return scanBuiltIn(ctx, dir, limit, allFiles, exclude)
}

func scanWithRipgrep(ctx context.Context, dir string, limit int, allFiles bool, exclude Exclude) ([]Match, error) {
	args := append([]string{"--json", "--line-number"}, exclude.ripgrepGlobs()...)
	if allFiles {
		args = append(args, "--hidden", "--no-ignore")
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
	var matches []Match
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
		matches = append(matches, newMatch(strings.TrimPrefix(event.Data.Path.Text, "./"), event.Data.LineNumber, event.Data.Lines.Text))
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

// sortedMatches orders matches by level, then file and line, before applying
// limit, so parallel scans always return the same results and a limit keeps
// the most urgent.
func sortedMatches(matches []Match, limit int) []Match {
	slices.SortFunc(matches, func(a, b Match) int {
		return cmp.Or(cmp.Compare(LevelRank(a.Level), LevelRank(b.Level)), strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

var (
	commentMarker = regexp.MustCompile(`(?i)(^|[^[:alnum:]_-])(@?todo([0-9]*@[[:alnum:]_-]+|[0-9]+)?)($|[^[:alnum:]_-])`)
	markerEnd     = regexp.MustCompile(`^(?:\([^)]*\))?[\s:\-–—]*`)
)

type todoComment struct {
	note, category, level string
}

func hasTodoComment(line string) bool {
	_, ok := commentTodo(line)
	return ok
}

// commentTodo finds a to-do marker in the line's comment. It counts when it
// opens the comment, carries a todo-system level or category, or is followed
// by ':' or '(', so prose that merely mentions a todo is skipped.
func commentTodo(line string) (todoComment, bool) {
	start, ok := commentStart(line)
	if !ok {
		return todoComment{}, false
	}
	body := strings.TrimLeft(line[start:], "/*#;%!-<> \t")
	for _, loc := range commentMarker.FindAllStringSubmatchIndex(body, -1) {
		markerStart, end, tagStart := loc[4], loc[5], loc[6]
		next := body[end:]
		if markerStart != 0 && tagStart < 0 && !strings.HasPrefix(next, ":") && !strings.HasPrefix(next, "(") {
			continue
		}
		var c todoComment
		if tagStart >= 0 {
			tag := body[tagStart:loc[7]]
			if _, category, ok := strings.Cut(tag, "@"); ok {
				c.category = category
			} else if validLevel(tag) {
				c.level = tag
			}
		}
		c.note = noteAfterMarker(next)
		return c, true
	}
	return todoComment{}, false
}

// validLevel accepts todo-system's levels, one digit or only zeros; like
// todo11, anything else is left as a generic to-do.
func validLevel(level string) bool {
	return len(level) == 1 || strings.Trim(level, "0") == ""
}

func noteAfterMarker(rest string) string {
	rest = rest[len(markerEnd.FindString(rest)):]
	rest = strings.TrimSpace(rest)
	for _, closer := range []string{"*/", "-->", "--}}"} {
		rest = strings.TrimSuffix(rest, closer)
	}
	return strings.TrimSpace(rest)
}

// commentStart finds where the line's comment begins, skipping anything
// inside string literals.
func commentStart(line string) (int, bool) {
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
			return i, true
		}
	}
	return 0, false
}
