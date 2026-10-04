package store

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/nebarg/todo-cli/internal/level"
)

var readmeLevel = regexp.MustCompile(`(?i)^todo([0-9]+):?$`)

// ReadmeTask is a list item in a README's to-dos. LoadReadme reads it and
// ToggleReadme marks it done or reopens it; the task file's write functions
// don't take it. The unexported field records the line as read, so a write
// can refuse a line that changed on disk.
type ReadmeTask struct {
	// Line is the index of the task's line in the README, so the first line is 0.
	Line int
	Text string
	Done bool
	// Level is the task's todo-system level, such as "0" for todo0.
	Level string
	// Heading is the heading the task is under inside its TODO section, or
	// "" for a task directly under the TODO heading.
	Heading string

	raw string
}

// task is t in the shape rewriteTask checks: one line with no body.
func (t ReadmeTask) task() Task {
	return Task{Line: t.Line, Done: t.Done, raw: t.raw, bodyStart: t.Line + 1, bodyEnd: t.Line + 1}
}

// LoadReadme reads a README's to-dos as todo-system does: every list item
// under a "TODO" or "TODOs" heading, up to the next heading at its level or
// above. A deeper heading inside it sets the Heading of the tasks after it.
// A todo0-style word sets the task's Level. A missing file has no tasks.
func LoadReadme(path string) ([]ReadmeTask, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, text := splitBOM(data)
	var tasks []ReadmeTask
	todoLevel, heading, inCode := 0, "", false // todoLevel is 0 outside a TODO section
	for i, raw := range strings.Split(text, "\n") {
		if strings.HasPrefix(raw, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		// Like todo-system, only unindented headings count, so a "# comment"
		// in an indented code block doesn't end the section.
		if level, name, ok := parseHeading(raw); ok && strings.HasPrefix(raw, "#") {
			switch {
			case todoLevel > 0 && level > todoLevel:
				heading = name
			case isTodoHeading(name):
				todoLevel, heading = level, ""
			default:
				todoLevel = 0
			}
			continue
		}
		parts := taskLine.FindStringSubmatch(raw)
		if todoLevel == 0 || parts == nil || strings.TrimSpace(parts[3]) == "" {
			continue
		}
		t := ReadmeTask{Line: i, Heading: heading, raw: raw, Done: parts[2] == "x" || parts[2] == "X"}
		t.Text, t.Level = splitReadmeLevel(strings.TrimSpace(parts[3]))
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// isTodoHeading reports whether a heading's name reads "TODO" or "TODOs",
// with or without a colon, in any case.
func isTodoHeading(name string) bool {
	name = strings.ToLower(strings.TrimSpace(strings.TrimRight(name, ":")))
	return name == "todo" || name == "todos"
}

// splitReadmeLevel takes the first todo-system level word out of text. As
// with file TODOs, a level todo-system rejects, such as todo12, stays text.
func splitReadmeLevel(text string) (string, string) {
	words := strings.Fields(text)
	for i, word := range words {
		parts := readmeLevel.FindStringSubmatch(word)
		if parts == nil {
			continue
		}
		if l := parts[1]; level.Valid(l) {
			rest := strings.Join(append(words[:i:i], words[i+1:]...), " ")
			if rest == "" {
				return text, l
			}
			return rest, l
		}
		return text, ""
	}
	return text, ""
}

// ToggleReadme flips a README task between open and done, adding the
// checkbox a plain list item lacks. The rest of the line, and the README's
// order, are left alone.
func ToggleReadme(path string, selected ReadmeTask) error {
	return rewriteTask(path, selected.task(), func(lines []string) string {
		parts := taskLine.FindStringSubmatch(selected.raw)
		mark := "x"
		if selected.Done {
			mark = " "
		}
		lines[selected.Line] = parts[1] + "[" + mark + "] " + parts[3]
		return strings.Join(lines, "\n")
	})
}
