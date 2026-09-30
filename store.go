package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var taskLine = regexp.MustCompile(`^([ \t]*- )(?:\[([ xX])\] +)?(.*)$`)
var markdownHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.+?)\s*\r?$`)
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
	headingLine  int
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
	branchSectionLevel, branch, label, labelLine := 0, "", "", -1
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if heading := markdownHeading.FindStringSubmatch(raw); heading != nil {
			level, name := len(heading[1]), strings.TrimSpace(strings.TrimRight(strings.TrimSpace(heading[2]), "#"))
			if strings.EqualFold(name, "Branches") && (level == 1 || (level == 2 && branchSectionLevel == 0)) {
				branchSectionLevel, branch, label, labelLine = level, "", "", -1
				continue
			}
			if branchSectionLevel != 0 && level > branchSectionLevel {
				if level == branchSectionLevel+1 {
					branch = name
				}
				label, labelLine = "", -1
				continue
			}
			branchSectionLevel, branch = 0, ""
			label, labelLine = strings.TrimPrefix(name, "@"), i
			continue
		}
		parts := taskLine.FindStringSubmatch(raw)
		if parts == nil || strings.TrimSpace(parts[3]) == "" {
			continue
		}
		t := task{
			line: i, raw: raw, text: strings.TrimSuffix(parts[3], "\r"),
			done: parts[2] == "x" || parts[2] == "X", branch: branch, headingLabel: label != "" && branch == "", headingLine: labelLine,
		}
		if label != "" && branch == "" {
			t.labels = []string{label}
		}
		indent := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		j := i + 1
		for j < len(lines) {
			line := strings.TrimSuffix(lines[j], "\r")
			if strings.HasPrefix(strings.ToLower(line), strings.ToLower(indent+"  - Priority:")) {
				value := strings.TrimSpace(line[len(indent+"  - Priority:"):])
				value = strings.ToLower(value)
				if validPriority(value) {
					t.priority = value
				}
			} else if strings.HasPrefix(line, indent+"  - Labels: ") && label == "" && branch == "" {
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
			if _, _, heading := parseHeading(line); heading {
				break
			}
			if parts := taskLine.FindStringSubmatch(line); parts != nil && strings.TrimSpace(parts[3]) != "" && !strings.HasPrefix(line, indent+"  ") {
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
		return errors.New("a task can have only one category")
	}
	label := ""
	if len(labels) > 0 {
		label = normalizeLabelInput(labels[0])
		if err := validateLabel(label); err != nil {
			return err
		}
	}
	if branch != "" && label != "" {
		return errors.New("branch tasks cannot have a category")
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
	if !taskUnchanged(lines, selected) {
		return errTaskChanged
	}
	updated := append([]string{}, lines[:selected.line]...)
	updated = append(updated, normalizedTaskLine(selected, title, selected.done))
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

func taskUnchanged(lines []string, selected task) bool {
	if selected.line < 0 || selected.line >= len(lines) || lines[selected.line] != selected.raw || selected.bodyEnd > len(lines) || !taskLine.MatchString(selected.raw) {
		return false
	}
	for i, raw := range selected.meta {
		if selected.line+1+i >= len(lines) || lines[selected.line+1+i] != raw {
			return false
		}
	}
	for i, raw := range selected.bodyRaw {
		if selected.bodyStart+i >= len(lines) || lines[selected.bodyStart+i] != raw {
			return false
		}
	}
	return true
}

func trimTaskBlock(block []string) []string {
	for len(block) > 1 && strings.TrimSpace(block[len(block)-1]) == "" {
		block = block[:len(block)-1]
	}
	return block
}

func validateLabel(label string) error {
	if label == "" {
		return errors.New("category must be one word of letters and numbers")
	}
	if strings.EqualFold(label, "Branches") {
		return errors.New("Branches is reserved for the branch section")
	}
	for _, r := range label {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return errors.New("category must be one word of letters and numbers")
		}
	}
	return nil
}

func normalizeLabelInput(raw string) string {
	label := strings.TrimSpace(raw)
	if strings.HasPrefix(label, "@") || strings.HasPrefix(label, "#") {
		label = label[1:]
	}
	return label
}

func metadataLines(priority string) []string {
	var lines []string
	if priority != "" {
		lines = append(lines, "  - Priority: "+strings.ToUpper(priority[:1])+priority[1:])
	}
	return lines
}

func insertTaskBlock(data string, block []string, branch, label string) string {
	priority := priorityFromBlock(block)
	if strings.TrimSpace(data) == "" {
		if branch != "" {
			return "# Branches\n\n## " + branch + "\n\n" + strings.Join(block, "\n") + "\n"
		}
		if label != "" {
			return "# " + label + "\n\n" + strings.Join(block, "\n") + "\n"
		}
		return strings.Join(block, "\n") + "\n"
	}
	lines := strings.Split(data, "\n")
	if branch != "" {
		start, end, level := findBranchesSection(lines)
		if start < 0 {
			return appendSection(data, insertTaskBlock("", block, branch, ""))
		}
		branchStart, branchEnd := findBranchSection(lines, start, end, level, branch)
		if branchStart < 0 {
			return insertBlockAt(lines, trimBlankEnd(lines, end, start+1), append([]string{strings.Repeat("#", level+1) + " " + branch, ""}, block...))
		}
		for i := branchStart + 1; i < branchEnd; i++ {
			if _, _, ok := parseHeading(lines[i]); ok {
				branchEnd = i // New branch tasks stay outside old nested headings.
				break
			}
		}
		return insertBlockSorted(lines, branchStart+1, branchEnd, block, priority)
	}
	if label == "" {
		end := len(lines)
		for i, line := range lines {
			if _, _, ok := parseHeading(line); ok {
				end = i
				break
			}
		}
		return insertBlockSorted(lines, 0, end, block, priority)
	}
	if start, end := findCategorySection(lines, label); start >= 0 {
		return insertBlockSorted(lines, start+1, end, block, priority)
	}
	idx := len(lines)
	if branchesStart, _, _ := findBranchesSection(lines); branchesStart >= 0 {
		idx = branchesStart
	}
	return insertBlockAt(lines, trimBlankEnd(lines, idx, 0), append([]string{"# " + label, ""}, block...))
}

func parseHeading(line string) (int, string, bool) {
	parts := markdownHeading.FindStringSubmatch(line)
	if parts == nil {
		return 0, "", false
	}
	return len(parts[1]), strings.TrimSpace(strings.TrimRight(strings.TrimSpace(parts[2]), "#")), true
}

func findBranchesSection(lines []string) (int, int, int) {
	for i, line := range lines {
		level, name, ok := parseHeading(line)
		if !ok || !strings.EqualFold(name, "Branches") || (level != 1 && level != 2) {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if nextLevel, _, ok := parseHeading(lines[j]); ok && nextLevel <= level {
				end = j
				break
			}
		}
		return i, end, level
	}
	return -1, -1, 0
}

func findBranchSection(lines []string, start, end, level int, branch string) (int, int) {
	for i := start + 1; i < end; i++ {
		currentLevel, name, ok := parseHeading(lines[i])
		if !ok || currentLevel != level+1 || name != branch {
			continue
		}
		branchEnd := end
		for j := i + 1; j < end; j++ {
			if nextLevel, _, ok := parseHeading(lines[j]); ok && nextLevel <= currentLevel {
				branchEnd = j
				break
			}
		}
		return i, branchEnd
	}
	return -1, -1
}

func findCategorySection(lines []string, category string) (int, int) {
	branchesEnd := -1
	for i, line := range lines {
		if i < branchesEnd {
			continue
		}
		level, name, ok := parseHeading(line)
		if !ok {
			continue
		}
		if strings.EqualFold(name, "Branches") && (level == 1 || level == 2) {
			_, branchesEnd, _ = findBranchesSection(lines[i:])
			branchesEnd += i
			continue
		}
		if !strings.EqualFold(strings.TrimPrefix(name, "@"), category) {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if _, _, ok := parseHeading(lines[j]); ok {
				end = j
				break
			}
		}
		return i, end
	}
	return -1, -1
}

func priorityFromBlock(block []string) string {
	for _, line := range block[1:] {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(trimmed), "- priority:") {
			break
		}
		value := strings.ToLower(strings.TrimSpace(trimmed[len("- priority:"):]))
		if validPriority(value) {
			return value
		}
	}
	return ""
}

func insertBlockSorted(lines []string, start, end int, block []string, priority string) string {
	idx := trimBlankEnd(lines, end, start)
	for i := start; i < end; i++ {
		parts := taskLine.FindStringSubmatch(lines[i])
		if parts == nil || strings.TrimSpace(parts[3]) == "" || strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "\t") {
			continue
		}
		oldPriority := ""
		for j := i + 1; j < end; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(strings.ToLower(trimmed), "- priority:") {
				break
			}
			oldPriority = strings.ToLower(strings.TrimSpace(trimmed[len("- priority:"):]))
		}
		if priorityRank(oldPriority) > priorityRank(priority) {
			idx = i
			break
		}
	}
	return insertBlockAt(lines, idx, block)
}

func insertBlockAt(lines []string, idx int, block []string) string {
	addition := append([]string(nil), block...)
	if idx > 0 && strings.TrimSpace(lines[idx-1]) != "" {
		addition = append([]string{""}, addition...)
	}
	if idx < len(lines) && strings.TrimSpace(lines[idx]) != "" {
		addition = append(addition, "")
	}
	updated := strings.Join(insertLines(lines, idx, addition), "\n")
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return updated
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

func trimBlankEnd(lines []string, end, minimum int) int {
	for end > minimum && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

func insertLines(lines []string, index int, addition []string) []string {
	result := make([]string, 0, len(lines)+len(addition))
	result = append(result, lines[:index]...)
	result = append(result, addition...)
	result = append(result, lines[index:]...)
	return result
}

func normalizedTaskLine(selected task, title string, done bool) string {
	indent := selected.raw[:len(selected.raw)-len(strings.TrimLeft(selected.raw, " \t"))]
	mark := " "
	if done {
		mark = "x"
	}
	ending := ""
	if strings.HasSuffix(selected.raw, "\r") {
		ending = "\r"
	}
	return indent + "- [" + mark + "] " + title + ending
}

func toggleTask(path string, selected task) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if !taskUnchanged(lines, selected) {
		return errTaskChanged
	}
	lines[selected.line] = normalizedTaskLine(selected, selected.text, !selected.done)
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
	if !taskUnchanged(lines, selected) {
		return errTaskChanged
	}
	updated := append([]string{}, lines[:selected.line]...)
	updated = append(updated, normalizedTaskLine(selected, selected.text, selected.done))
	indent := selected.raw[:len(selected.raw)-len(strings.TrimLeft(selected.raw, " \t"))]
	for _, line := range metadataLines(priority) {
		updated = append(updated, indent+line)
	}
	for _, line := range selected.meta {
		if !strings.HasPrefix(strings.ToLower(line), strings.ToLower(indent+"  - Priority:")) {
			updated = append(updated, line)
		}
	}
	updated = append(updated, lines[selected.line+1+len(selected.meta):]...)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return replaceFile(path, []byte(strings.Join(updated, "\n")), info.Mode().Perm())
}

func setTaskLabel(path string, selected task, label string) error {
	if selected.branch != "" {
		return errors.New("branch tasks do not have categories")
	}
	blank := strings.TrimSpace(label) == ""
	label = normalizeLabelInput(label)
	if !blank {
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
	block := []string{normalizedTaskLine(selected, selected.text, selected.done)}
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
	if !selected.headingLabel || selected.headingLine < 0 || selected.headingLine >= len(lines) {
		return lines
	}
	start := selected.headingLine
	_, name, ok := parseHeading(lines[start])
	if !ok || !strings.EqualFold(strings.TrimPrefix(name, "@"), taskLabel(selected)) {
		return lines
	}
	end := start + 1
	for end < len(lines) {
		if _, _, ok := parseHeading(lines[end]); ok {
			break
		}
		if strings.TrimSpace(lines[end]) != "" {
			return lines
		}
		end++
	}
	return append(append([]string{}, lines[:start]...), lines[end:]...)
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
