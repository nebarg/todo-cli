package store

import (
	"errors"
	"os"
	"regexp"
	"strings"
)

var readmeLevel = regexp.MustCompile(`(?i)^todo([0-9]+):?$`)

// LoadReadme reads a README's to-dos as todo-system does: every list item
// under a "TODO" or "TODOs" heading, up to the next heading. A todo0-style
// word sets the task's Level. A missing file has no tasks.
func LoadReadme(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks []Task
	inTodos, inCode := false, false
	for i, raw := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(raw, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		// Like todo-system, only unindented headings count, so a "# comment"
		// in an indented code block doesn't end the section.
		if _, name, ok := parseHeading(raw); ok && strings.HasPrefix(raw, "#") {
			name = strings.ToLower(strings.TrimSpace(strings.TrimRight(name, ":")))
			inTodos = name == "todo" || name == "todos"
			continue
		}
		parts := taskLine.FindStringSubmatch(raw)
		if !inTodos || parts == nil || strings.TrimSpace(parts[3]) == "" {
			continue
		}
		t := Task{Line: i, raw: raw, Done: parts[2] == "x" || parts[2] == "X", bodyStart: i + 1, bodyEnd: i + 1}
		t.Text, t.Level = splitReadmeLevel(strings.TrimSpace(parts[3]))
		tasks = append(tasks, t)
	}
	return tasks, nil
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
		if level := parts[1]; len(level) == 1 || strings.Trim(level, "0") == "" {
			rest := strings.Join(append(words[:i:i], words[i+1:]...), " ")
			if rest == "" {
				return text, level
			}
			return rest, level
		}
		return text, ""
	}
	return text, ""
}

// ToggleReadme flips a README task between open and done, adding the
// checkbox a plain list item lacks. The rest of the line, and the README's
// order, are left alone.
func ToggleReadme(path string, selected Task) error {
	return rewriteTask(path, selected, func(lines []string) string {
		parts := taskLine.FindStringSubmatch(selected.raw)
		mark := "x"
		if selected.Done {
			mark = " "
		}
		lines[selected.Line] = parts[1] + "[" + mark + "] " + parts[3]
		return strings.Join(lines, "\n")
	})
}
