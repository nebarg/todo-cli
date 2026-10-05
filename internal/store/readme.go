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
	// Details is the text indented under a checkbox task, such as plain
	// list items, without the indent it shares.
	Details string
	// Subtask is true for a task nested under another, at any depth. Its
	// parent is the last task before it that isn't a subtask.
	Subtask bool

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
	text, _ := decode(data)
	return parseReadme(strings.Split(text, "\n")), nil
}

// readmeItem is a list item enclosing the line parseReadme is on.
type readmeItem struct {
	indent int
	owner  int // the task whose details take plain items nested here, or -1
}

// parseReadme reads the tasks in lines. Checkbox items are always tasks.
// Plain items are too, except under a checkbox task: there they're its
// details, with the rest of the text indented under it. A task nested under
// another is a subtask.
func parseReadme(lines []string) []ReadmeTask {
	var tasks []ReadmeTask
	var details [][]string                     // each task's detail lines, as written
	var parents []readmeItem                   // the list items enclosing a line, outermost first
	todoLevel, heading, inCode := 0, "", false // todoLevel is 0 outside a TODO section
	for i, raw := range lines {
		if strings.HasPrefix(raw, "```") {
			inCode, parents = !inCode, nil
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
			parents = nil
			continue
		}
		if todoLevel == 0 {
			continue
		}
		parts := taskLine.FindStringSubmatch(raw)
		isItem := parts != nil && strings.TrimSpace(parts[3]) != ""
		indent := len(taskIndent(raw))
		if strings.TrimSpace(raw) != "" {
			parents = enclosing(parents, indent)
		}
		owner := -1
		if len(parents) > 0 {
			owner = parents[len(parents)-1].owner
		}
		checkbox := isItem && parts[2] != ""
		if owner >= 0 && !checkbox {
			details[owner] = append(details[owner], raw)
			if isItem {
				parents = append(parents, readmeItem{indent: indent, owner: owner})
			}
			continue
		}
		if !isItem {
			continue
		}
		t := ReadmeTask{Line: i, Heading: heading, Subtask: len(parents) > 0, raw: raw, Done: parts[2] == "x" || parts[2] == "X"}
		t.Text, t.Level = splitReadmeLevel(strings.TrimSpace(parts[3]))
		tasks, details = append(tasks, t), append(details, nil)
		item := readmeItem{indent: indent, owner: -1}
		if checkbox {
			item.owner = len(tasks) - 1
		}
		parents = append(parents, item)
	}
	for i := range tasks {
		tasks[i].Details = dedent(details[i])
	}
	return tasks
}

// enclosing is parents without the items a line indented by indent is
// outside of, so an unindented line is outside every list.
func enclosing(parents []readmeItem, indent int) []readmeItem {
	for len(parents) > 0 && parents[len(parents)-1].indent >= indent {
		parents = parents[:len(parents)-1]
	}
	return parents
}

// dedent joins lines without the blank lines around them or the indent
// they share.
func dedent(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	shared := -1
	for _, line := range lines {
		if indent := len(taskIndent(line)); strings.TrimSpace(line) != "" && (shared < 0 || indent < shared) {
			shared = indent
		}
	}
	result := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			result[i] = line[shared:]
		}
	}
	return strings.Join(result, "\n")
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
