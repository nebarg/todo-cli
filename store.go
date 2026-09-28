package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var taskLine = regexp.MustCompile(`^(\s*[-*+] \[)([ xX])(\] +)(.*)$`)
var errTaskChanged = errors.New("task changed on disk; press r to reload")

type task struct {
	line         int
	raw          string
	text         string
	done         bool
	branch       string
	priority     string
	labels       []string
	headingLabel bool
	meta         []string
	details      string
	bodyStart    int
	bodyEnd      int
	bodyRaw      []string
}

func loadTasks(path string) ([]task, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var tasks []task
	inBranches, branch, label := false, "", ""
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(raw, "## ") {
			inBranches = trimmed == "## Branches"
			branch = ""
			label = ""
			continue
		}
		if strings.HasPrefix(raw, "### ") {
			if inBranches {
				branch = strings.TrimSpace(strings.TrimPrefix(raw, "### "))
				label = ""
			} else {
				label = headingLabel(raw, "### ")
			}
			continue
		}
		if strings.HasPrefix(raw, "#### ") && inBranches {
			label = headingLabel(raw, "#### ")
			continue
		}
		parts := taskLine.FindStringSubmatch(raw)
		if parts == nil || (inBranches && branch == "") {
			continue
		}
		t := task{
			line: i, raw: raw, text: strings.TrimSuffix(parts[4], "\r"),
			done: parts[2] == "x" || parts[2] == "X", branch: branch, headingLabel: label != "",
		}
		if label != "" {
			t.labels = []string{label}
		}
		indent := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		j := i + 1
		for j < len(lines) {
			line := strings.TrimSuffix(lines[j], "\r")
			if strings.HasPrefix(line, indent+"  - Priority: ") {
				t.priority = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, indent+"  - Priority: ")))
			} else if strings.HasPrefix(line, indent+"  - Labels: ") && label == "" {
				t.labels = parseLabels(strings.TrimPrefix(line, indent+"  - Labels: "))
			} else if strings.HasPrefix(line, indent+"  - Labels: ") {
				// Read old metadata without overriding the heading's label.
			} else {
				break
			}
			t.meta = append(t.meta, lines[j])
			j++
		}
		t.bodyStart = j
		start := j
		for ; j < len(lines); j++ {
			line := strings.TrimSuffix(lines[j], "\r")
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, indent+"  ") {
				break
			}
		}
		t.bodyEnd = j
		t.bodyRaw = append([]string(nil), lines[t.bodyStart:t.bodyEnd]...)
		end := j
		for start < end && strings.TrimSpace(lines[start]) == "" {
			start++
		}
		for end > start && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		var body []string
		for _, line := range lines[start:end] {
			line = strings.TrimSuffix(line, "\r")
			body = append(body, strings.TrimPrefix(line, indent+"  "))
		}
		t.details = strings.Join(body, "\n")
		tasks = append(tasks, t)
		i = j - 1
	}
	return tasks, nil
}

func headingLabel(line, prefix string) string {
	name := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !strings.HasPrefix(name, "@") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(name, "@"))
}

func parseLabels(s string) []string {
	var labels []string
	seen := make(map[string]bool)
	for _, raw := range strings.Split(s, ",") {
		label := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(raw), "#@"))
		key := strings.ToLower(label)
		if label != "" && !seen[key] {
			labels = append(labels, label)
			seen[key] = true
		}
	}
	return labels
}

func validPriority(value string) bool {
	return value == "" || value == "high" || value == "medium" || value == "low"
}

func addTask(path, title string) error {
	return addTaskWithOptions(path, title, "", nil, "")
}

func addTaskWithOptions(path, title, priority string, labels []string, branch string) error {
	return addTaskWithDetails(path, title, "", priority, labels, branch)
}

func addTaskWithDetails(path, title, details, priority string, labels []string, branch string) error {
	title = strings.TrimSpace(title)
	priority = strings.ToLower(strings.TrimSpace(priority))
	branch = strings.TrimSpace(branch)
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return errors.New("enter a single-line task")
	}
	if !validPriority(priority) {
		return errors.New("priority must be high, medium, or low")
	}
	if strings.ContainsAny(branch, "\r\n") {
		return errors.New("branch name must be one line")
	}
	if len(labels) > 1 {
		return errors.New("a task can have only one label")
	}
	label := ""
	if len(labels) > 0 {
		label = strings.TrimSpace(strings.TrimLeft(labels[0], "#@"))
		if err := validateLabel(label); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	block := []string{"- [ ] " + title}
	block = append(block, metadataLines(priority)...)
	if body := formattedDetails(details); len(body) > 0 {
		block = append(block, "")
		block = append(block, body...)
	}
	updated := insertTaskBlock(string(data), block, branch, label)
	return replaceFile(path, []byte(updated), mode)
}

func formattedDetails(details string) []string {
	details = strings.ReplaceAll(details, "\r\n", "\n")
	rows := strings.Split(details, "\n")
	for len(rows) > 0 && strings.TrimSpace(rows[0]) == "" {
		rows = rows[1:]
	}
	for len(rows) > 0 && strings.TrimSpace(rows[len(rows)-1]) == "" {
		rows = rows[:len(rows)-1]
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row) == "" {
			result = append(result, "")
		} else {
			result = append(result, "  "+strings.TrimSuffix(row, "\r"))
		}
	}
	return result
}

func editTaskContent(path string, selected task, title, details string) error {
	title = strings.TrimSpace(title)
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return errors.New("enter a single-line task title")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if selected.line < 0 || selected.line >= len(lines) || lines[selected.line] != selected.raw || selected.bodyEnd > len(lines) {
		return errTaskChanged
	}
	for i, raw := range selected.meta {
		if selected.line+1+i >= len(lines) || lines[selected.line+1+i] != raw {
			return errTaskChanged
		}
	}
	for i, raw := range selected.bodyRaw {
		if selected.bodyStart+i >= len(lines) || lines[selected.bodyStart+i] != raw {
			return errTaskChanged
		}
	}
	indices := taskLine.FindStringSubmatchIndex(selected.raw)
	if indices == nil {
		return errTaskChanged
	}
	updated := append([]string{}, lines[:selected.line]...)
	updated = append(updated, selected.raw[:indices[8]]+title)
	updated = append(updated, selected.meta...)
	if strings.Join(formattedDetails(details), "\n") == strings.Join(formattedDetails(selected.details), "\n") {
		updated = append(updated, selected.bodyRaw...)
	} else if body := formattedDetails(details); len(body) > 0 {
		updated = append(updated, "")
		updated = append(updated, body...)
		updated = append(updated, "")
	} else if selected.bodyEnd < len(lines) {
		updated = append(updated, "")
	}
	updated = append(updated, lines[selected.bodyEnd:]...)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return replaceFile(path, []byte(strings.Join(updated, "\n")), info.Mode().Perm())
}

func validateLabel(label string) error {
	if label == "" || strings.ContainsAny(label, ",\r\n") || strings.Contains(label, "#") {
		return errors.New("enter one label without commas or heading markers")
	}
	return nil
}

func metadataLines(priority string) []string {
	var lines []string
	if priority != "" {
		lines = append(lines, "  - Priority: "+strings.ToUpper(priority[:1])+priority[1:])
	}
	return lines
}

func insertTaskBlock(data string, block []string, branch, label string) string {
	if data == "" {
		if branch == "" {
			if label != "" {
				return "## General\n\n### @" + label + "\n\n" + strings.Join(block, "\n") + "\n"
			}
			return "## General\n\n" + strings.Join(block, "\n") + "\n"
		}
		if label != "" {
			return "## Branches\n\n### " + branch + "\n\n#### @" + label + "\n\n" + strings.Join(block, "\n") + "\n"
		}
		return "## Branches\n\n### " + branch + "\n\n" + strings.Join(block, "\n") + "\n"
	}
	lines := strings.Split(data, "\n")
	if branch == "" {
		start, end := findSection(lines, "## General")
		if start < 0 {
			branchesStart, _ := findSection(lines, "## Branches")
			if branchesStart >= 0 {
				addition := []string{"## General", ""}
				if label != "" {
					addition = append(addition, "### @"+label, "")
				}
				addition = append(addition, block...)
				addition = append(addition, "")
				if branchesStart > 0 && strings.TrimSpace(lines[branchesStart-1]) != "" {
					addition = append([]string{""}, addition...)
				}
				return strings.Join(insertLines(lines, branchesStart, addition), "\n")
			}
			return appendSection(data, insertTaskBlock("", block, branch, label))
		}
		return insertInLabeledSection(lines, start, end, block, "### ", label)
	}
	branchesStart, branchesEnd := findSection(lines, "## Branches")
	if branchesStart < 0 {
		return appendSection(data, insertTaskBlock("", block, branch, label))
	}
	for i := branchesStart + 1; i < branchesEnd; i++ {
		if lines[i] != "### "+branch {
			continue
		}
		end := branchesEnd
		for j := i + 1; j < branchesEnd; j++ {
			if strings.HasPrefix(lines[j], "### ") {
				end = j
				break
			}
		}
		return insertInLabeledSection(lines, i, end, block, "#### ", label)
	}
	idx := trimBlankEnd(lines, branchesEnd, branchesStart+1)
	addition := []string{"", "### " + branch, ""}
	if label != "" {
		addition = append(addition, "#### @"+label, "")
	}
	addition = append(addition, block...)
	return strings.Join(insertLines(lines, idx, addition), "\n")
}

func insertInLabeledSection(lines []string, start, end int, block []string, prefix, label string) string {
	if label == "" {
		for i := start + 1; i < end; i++ {
			if strings.HasPrefix(lines[i], prefix) {
				return insertInSection(lines, start, i, block)
			}
		}
		return insertInSection(lines, start, end, block)
	}
	for i := start + 1; i < end; i++ {
		if !strings.HasPrefix(lines[i], prefix) || !strings.EqualFold(headingLabel(lines[i], prefix), label) {
			continue
		}
		groupEnd := end
		for j := i + 1; j < end; j++ {
			if strings.HasPrefix(lines[j], prefix) {
				groupEnd = j
				break
			}
		}
		return insertInSection(lines, i, groupEnd, block)
	}
	idx := trimBlankEnd(lines, end, start+1)
	return strings.Join(insertLines(lines, idx, append([]string{"", prefix + "@" + label, ""}, block...)), "\n")
}

func appendSection(data, section string) string {
	if !strings.HasSuffix(data, "\n") {
		data += "\n"
	}
	if !strings.HasSuffix(data, "\n\n") {
		data += "\n"
	}
	return data + section
}

func findSection(lines []string, heading string) (int, int) {
	for i, line := range lines {
		if line != heading {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "## ") {
				return i, j
			}
		}
		return i, len(lines)
	}
	return -1, -1
}

func trimBlankEnd(lines []string, end, minimum int) int {
	for end > minimum && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

func insertInSection(lines []string, start, end int, block []string) string {
	idx := trimBlankEnd(lines, end, start+1)
	block = append([]string{""}, block...)
	return strings.Join(insertLines(lines, idx, block), "\n")
}

func insertLines(lines []string, index int, addition []string) []string {
	result := make([]string, 0, len(lines)+len(addition))
	result = append(result, lines[:index]...)
	result = append(result, addition...)
	result = append(result, lines[index:]...)
	return result
}

func toggleTask(path string, selected task) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if selected.line >= len(lines) || lines[selected.line] != selected.raw {
		return errTaskChanged
	}
	indices := taskLine.FindStringSubmatchIndex(selected.raw)
	if indices == nil {
		return errTaskChanged
	}
	mark := "x"
	if selected.done {
		mark = " "
	}
	start, end := indices[4], indices[5]
	lines[selected.line] = selected.raw[:start] + mark + selected.raw[end:]
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return replaceFile(path, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}

func setTaskPriority(path string, selected task, priority string) error {
	if !validPriority(priority) {
		return errors.New("priority must be high, medium, or low")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	end := selected.line + 1 + len(selected.meta)
	if end > len(lines) || lines[selected.line] != selected.raw {
		return errTaskChanged
	}
	for i, raw := range selected.meta {
		if lines[selected.line+1+i] != raw {
			return errTaskChanged
		}
	}
	updated := append([]string{}, lines[:selected.line+1]...)
	indent := selected.raw[:len(selected.raw)-len(strings.TrimLeft(selected.raw, " \t"))]
	for _, line := range metadataLines(priority) {
		updated = append(updated, indent+line)
	}
	for _, line := range selected.meta {
		if strings.HasPrefix(strings.TrimSpace(line), "- Labels: ") {
			updated = append(updated, line)
		}
	}
	updated = append(updated, lines[end:]...)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return replaceFile(path, []byte(strings.Join(updated, "\n")), info.Mode().Perm())
}

func setTaskLabel(path string, selected task, label string) error {
	label = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(label), "#@"))
	if label != "" {
		if err := validateLabel(label); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if selected.line < 0 || selected.line >= len(lines) || lines[selected.line] != selected.raw || selected.bodyEnd > len(lines) {
		return errTaskChanged
	}
	for i, raw := range selected.meta {
		if selected.line+1+i >= len(lines) || lines[selected.line+1+i] != raw {
			return errTaskChanged
		}
	}
	for i, raw := range selected.bodyRaw {
		if selected.bodyStart+i >= len(lines) || lines[selected.bodyStart+i] != raw {
			return errTaskChanged
		}
	}
	if selected.headingLabel && strings.EqualFold(taskLabel(selected), label) {
		return nil
	}
	indent := selected.raw[:len(selected.raw)-len(strings.TrimLeft(selected.raw, " \t"))]
	block := []string{selected.raw}
	for _, line := range selected.meta {
		if !strings.HasPrefix(line, indent+"  - Labels: ") {
			block = append(block, line)
		}
	}
	body := append([]string(nil), selected.bodyRaw...)
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	block = append(block, body...)
	remaining := append([]string{}, lines[:selected.line]...)
	remaining = append(remaining, lines[selected.bodyEnd:]...)
	remaining = removeEmptyLabelHeading(remaining, selected)
	updated := insertTaskBlock(strings.Join(remaining, "\n"), block, selected.branch, label)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return replaceFile(path, []byte(updated), info.Mode().Perm())
}

func removeEmptyLabelHeading(lines []string, selected task) []string {
	if !selected.headingLabel {
		return lines
	}
	prefix := "### "
	if selected.branch != "" {
		prefix = "#### "
	}
	start := -1
	for i := min(selected.line-1, len(lines)-1); i >= 0; i-- {
		if headingLevel(lines[i]) < len(prefix)-1 {
			break
		}
		if strings.HasPrefix(lines[i], prefix) && strings.EqualFold(headingLabel(lines[i], prefix), taskLabel(selected)) {
			start = i
			break
		}
	}
	if start < 0 {
		return lines
	}
	end := start + 1
	for end < len(lines) && headingLevel(lines[end]) > len(prefix)-1 {
		if strings.TrimSpace(lines[end]) != "" {
			return lines
		}
		end++
	}
	return append(append([]string{}, lines[:start]...), lines[end:]...)
}

func headingLevel(line string) int {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n > 0 && n < len(line) && line[n] == ' ' {
		return n
	}
	return 99
}

func replaceFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".todo-cli-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func taskSummary(t task) string {
	var details []string
	if t.priority != "" {
		details = append(details, strings.ToUpper(t.priority[:1]))
	}
	for _, label := range t.labels {
		details = append(details, "#"+label)
	}
	if t.branch != "" {
		details = append(details, "@"+t.branch)
	}
	if len(details) == 0 {
		return t.text
	}
	return t.text + "  " + strings.Join(details, " ")
}

func taskLocation(t task) string {
	if t.branch == "" {
		return "General"
	}
	return fmt.Sprintf("Branch %s", t.branch)
}
