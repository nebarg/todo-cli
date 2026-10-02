// Package scan finds to-do marker comments in source code.
package scan

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/nebarg/todo-cli/internal/level"
)

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

// newMatch records the to-do c found on a line. Note is its text without
// the comment syntax and marker, or the whole line when that leaves nothing.
func newMatch(path string, line int, text string, c todoComment) Match {
	m := Match{Path: path, Line: line, Text: strings.TrimSpace(text), Note: c.note, Category: c.category, Level: c.level}
	if m.Note == "" {
		m.Note = m.Text
	}
	return m
}

// Source finds to-do marker comments under dir, reading each file to tell
// its comments from its code. Results are sorted most urgent first,
// then by path and line. Ignored and Markdown files and those over 1 MB are
// skipped, as are directories in exclude and those starting with a dot;
// hidden files are read. Cancelling ctx stops the scan.
func Source(ctx context.Context, dir string, exclude Exclude) ([]Match, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	return scanFiles(ctx, dir, exclude)
}

// sortedMatches orders matches by level, then file and line, so parallel
// scans always return the same results.
func sortedMatches(matches []Match) []Match {
	slices.SortFunc(matches, func(a, b Match) int {
		return cmp.Or(cmp.Compare(level.Rank(a.Level), level.Rank(b.Level)), strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return matches
}

var (
	commentMarker = regexp.MustCompile(`(?i)(^|[^[:alnum:]_-])((?:@ ?)?todo([0-9]*@[[:alnum:]_-]+|[0-9]+)?)($|[^[:alnum:]_-])`)
	markerEnd     = regexp.MustCompile(`^(?:\([^)]*\))?[\s:!\-–—]*`)
)

type todoComment struct {
	note, category, level string
}

// commentTodo finds a to-do in a line of a file whose comment syntax is
// unknown, guessing where the line's comment starts.
func commentTodo(line string) (todoComment, bool) {
	start, ok := commentStart(line)
	if !ok {
		return todoComment{}, false
	}
	return todoInComment(line[start:])
}

// todoInComment finds a to-do marker in a comment's text. It counts when it
// opens the comment, carries a todo-system level or category, or is followed
// by ':' or '(', so prose that merely mentions a todo is skipped. "@ todo"
// counts as "@todo".
func todoInComment(comment string) (todoComment, bool) {
	if !containsTodo(comment) {
		return todoComment{}, false
	}
	body := strings.TrimLeft(comment, "/*#;%!-<> \t")
	if c, ok := markerIn(body); ok {
		return c, true
	}
	// Commented-out code can end with a comment of its own, as in
	// "// x = 1; // TODO: drop x", so an opener after a space starts a
	// comment too.
	for i := 1; i < len(body); i++ {
		if body[i-1] != ' ' && body[i-1] != '\t' {
			continue
		}
		if strings.HasPrefix(body[i:], "//") || strings.HasPrefix(body[i:], "/*") || body[i] == '#' {
			if c, ok := markerIn(strings.TrimLeft(body[i:], "/*#;%!-<> \t")); ok {
				return c, true
			}
		}
	}
	return todoComment{}, false
}

// markerIn finds the first marker in a comment's body that counts.
func markerIn(body string) (todoComment, bool) {
	for _, loc := range commentMarker.FindAllStringSubmatchIndex(body, -1) {
		markerStart, end, tagStart := loc[4], loc[5], loc[6]
		next := body[end:]
		var c todoComment
		if tagStart >= 0 {
			// A level todo-system doesn't accept, such as todo11, is left
			// as a plain marker.
			tag := body[tagStart:loc[7]]
			if _, category, ok := strings.Cut(tag, "@"); ok {
				c.category = category
			} else if level.Valid(tag) {
				c.level = tag
			}
		}
		tagged := c.category != "" || c.level != ""
		if markerStart != 0 && !tagged && !strings.HasPrefix(next, ":") && !strings.HasPrefix(next, "(") {
			continue
		}
		c.note = noteAfterMarker(next)
		return c, true
	}
	return todoComment{}, false
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
