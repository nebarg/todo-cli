// Package store reads and edits tasks in a Markdown task file, preserving
// everything it does not change.
package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var taskLine = regexp.MustCompile(`^([ \t]*- )(?:\[([ xX])\] +)?(.*)$`)
var markdownHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.+?)\s*\r?$`)

// ErrTaskChanged means the file no longer matches the task that was read,
// so the edit was refused rather than risk changing the wrong lines.
var ErrTaskChanged = errors.New("task changed on disk")

// Task is one Markdown checklist item. The unexported fields record exactly
// what was read, so writes can refuse to touch a task that changed on disk.
type Task struct {
	Line     int
	Text     string
	Done     bool
	Branch   string
	Priority Priority
	Category string
	Details  string
	// Level is a README task's todo-system level, such as "0" for todo0.
	Level string

	raw          string
	categoryLine int
	bodyStart    int
	bodyEnd      int
	bodyRaw      []string
}

// Load reads every task in the Markdown file at path. A missing file has no tasks.
func Load(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseTasks(strings.Split(string(data), "\n")), nil
}

func parseTasks(lines []string) []Task {
	var tasks []Task
	branchSectionLevel, branch, category, categoryLine := 0, "", "", -1
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if level, name, ok := parseHeading(raw); ok {
			if strings.EqualFold(name, "Branches") && (level == 1 || (level == 2 && branchSectionLevel == 0)) {
				branchSectionLevel, branch, category, categoryLine = level, "", "", -1
				continue
			}
			if branchSectionLevel != 0 && level > branchSectionLevel {
				if level == branchSectionLevel+1 {
					branch = name
				}
				category, categoryLine = "", -1
				continue
			}
			branchSectionLevel, branch = 0, ""
			category, categoryLine = NormalizeCategory(name), i
			continue
		}
		parts := taskLine.FindStringSubmatch(raw)
		if parts == nil || strings.TrimSpace(parts[3]) == "" {
			continue
		}
		t := Task{
			Line: i, raw: raw, Done: parts[2] == "x" || parts[2] == "X",
			Branch: branch, Category: category, categoryLine: categoryLine,
		}
		t.Text, t.Priority = SplitPriority(strings.TrimSuffix(parts[3], "\r"))
		indent := taskIndent(raw)
		j := i + 1
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
		t.Details = strings.Join(body, "\n")
		tasks = append(tasks, t)
		i = j - 1
	}
	return tasks
}

func taskIndent(raw string) string {
	return raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
}

// Add writes a new open task, filed under branch or category (not both), and
// creates the file or section if needed. A trailing !priority in title is used
// when p is PriorityNone.
func Add(path, title, details string, p Priority, category, branch string) error {
	title, titlePriority := SplitPriority(strings.TrimSpace(title))
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return errors.New("enter a single-line task")
	}
	if titlePriority != PriorityNone && p != PriorityNone && titlePriority != p {
		return errors.New("give the priority once, as a flag or a trailing !priority")
	}
	if p == PriorityNone {
		p = titlePriority
	}
	category, branch, err := validScope(category, branch)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	block := []string{"- [ ] " + title + priorityToken(p)}
	if body := formattedDetails(details); len(body) > 0 {
		block = append(block, "")
		block = append(block, body...)
	}
	updated := sortSection(insertTaskBlock(string(data), block, branch, category), category, branch)
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

// Edit replaces the title and details of selected, keeping its priority
// unless title ends with a new !priority, and files it under category or
// branch (not both). A task that keeps its section is edited in place.
func Edit(path string, selected Task, title, details, category, branch string) error {
	title, p := SplitPriority(strings.TrimSpace(title))
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return errors.New("enter a single-line task title")
	}
	if p != PriorityNone {
		selected.Priority = p
	}
	category, branch, err := validScope(category, branch)
	if err != nil {
		return err
	}
	return rewriteTask(path, selected, func(lines []string) string {
		if branch == selected.Branch && strings.EqualFold(category, selected.Category) {
			return sortSection(strings.Join(editedTaskLines(lines, selected, title, details), "\n"), category, branch)
		}
		return sortSection(movedTaskLines(lines, selected, taskBlock(selected, title, details), category, branch), category, branch)
	})
}

// validScope normalises a task's destination section; a blank category is
// the general list.
func validScope(category, branch string) (string, string, error) {
	branch = strings.TrimSpace(branch)
	if strings.ContainsAny(branch, "\r\n") {
		return "", "", errors.New("branch name must be one line")
	}
	if strings.TrimSpace(category) == "" {
		return "", branch, nil
	}
	category = NormalizeCategory(category)
	if err := validateCategory(category); err != nil {
		return "", "", err
	}
	if branch != "" {
		return "", "", errors.New("branch tasks cannot have a category")
	}
	return category, branch, nil
}

// taskBlock is selected's lines with a new title and details, keeping the
// original details text when it has not changed.
func taskBlock(selected Task, title, details string) []string {
	block := []string{normalizedTaskLine(selected, title, selected.Done, selected.Priority)}
	if strings.Join(formattedDetails(details), "\n") == strings.Join(formattedDetails(selected.Details), "\n") {
		body := append([]string(nil), selected.bodyRaw...)
		for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
			body = body[:len(body)-1]
		}
		return append(block, body...)
	}
	if body := formattedDetails(details); len(body) > 0 {
		block = append(block, "")
		block = append(block, body...)
	}
	return block
}

// movedTaskLines removes selected from lines, drops a heading it leaves empty,
// and files block under category or branch.
func movedTaskLines(lines []string, selected Task, block []string, category, branch string) string {
	remaining := append([]string{}, lines[:selected.Line]...)
	remaining = append(remaining, lines[selected.bodyEnd:]...)
	remaining = removeEmptyCategoryHeading(remaining, selected)
	if selected.Branch != "" && selected.Branch != branch {
		remaining = removeEmptyBranchHeading(remaining, selected.Branch)
	}
	return insertTaskBlock(strings.Join(remaining, "\n"), block, branch, category)
}

func editedTaskLines(lines []string, selected Task, title, details string) []string {
	updated := append([]string{}, lines[:selected.Line]...)
	updated = append(updated, normalizedTaskLine(selected, title, selected.Done, selected.Priority))
	eol := lineEnding(lines)
	if strings.Join(formattedDetails(details), "\n") == strings.Join(formattedDetails(selected.Details), "\n") {
		updated = append(updated, selected.bodyRaw...)
	} else if body := formattedDetails(details); len(body) > 0 {
		updated = append(updated, eol)
		updated = append(updated, withEnding(body, eol)...)
		updated = append(updated, eol)
	} else if selected.bodyEnd < len(lines) {
		updated = append(updated, eol)
	}
	return append(updated, lines[selected.bodyEnd:]...)
}

// rewriteTask replaces the file with change(lines), refusing when selected no
// longer matches the file so edits never land on the wrong task.
func rewriteTask(path string, selected Task, change func(lines []string) string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if !taskUnchanged(lines, selected) {
		return ErrTaskChanged
	}
	return replaceFile(path, []byte(change(lines)), info.Mode().Perm())
}

func taskUnchanged(lines []string, selected Task) bool {
	if selected.Line < 0 || selected.Line >= len(lines) || lines[selected.Line] != selected.raw || selected.bodyEnd > len(lines) || !taskLine.MatchString(selected.raw) {
		return false
	}
	for i, raw := range selected.bodyRaw {
		if selected.bodyStart+i >= len(lines) || lines[selected.bodyStart+i] != raw {
			return false
		}
	}
	return true
}

func validateCategory(category string) error {
	if category == "" {
		return errors.New("category must be a single word without whitespace")
	}
	if strings.EqualFold(category, "Branches") {
		return errors.New(`"Branches" is reserved for the branch section`)
	}
	for _, r := range category {
		if unicode.IsSpace(r) {
			return errors.New("category must be a single word without whitespace")
		}
	}
	return nil
}

// NormalizeCategory drops one leading @ or # from a category name.
func NormalizeCategory(raw string) string {
	if len(raw) > 1 && (strings.HasPrefix(raw, "@") || strings.HasPrefix(raw, "#")) {
		return raw[1:]
	}
	return raw
}

func insertTaskBlock(data string, block []string, branch, category string) string {
	if strings.TrimSpace(data) == "" {
		return strings.Join(sectionLines(block, branch, category), "\n") + "\n"
	}
	lines := withFinalNewline(strings.Split(data, "\n"))
	if branch != "" {
		start, end, level := findBranchesSection(lines)
		if start < 0 {
			return insertBlockAt(lines, len(lines)-1, sectionLines(block, branch, ""))
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
		return insertBlockAt(lines, trimBlankEnd(lines, branchEnd, branchStart+1), block)
	}
	if category == "" {
		end := len(lines)
		for i, line := range lines {
			if _, _, ok := parseHeading(line); ok {
				end = i
				break
			}
		}
		return insertBlockAt(lines, trimBlankEnd(lines, end, 0), block)
	}
	if start, end := findCategorySection(lines, category); start >= 0 {
		return insertBlockAt(lines, trimBlankEnd(lines, end, start+1), block)
	}
	idx := len(lines)
	if branchesStart, _, _ := findBranchesSection(lines); branchesStart >= 0 {
		idx = branchesStart
	}
	return insertBlockAt(lines, trimBlankEnd(lines, idx, 0), sectionLines(block, "", category))
}

// sectionLines is block under the headings of a new section: a branch in a
// new Branches section, a category, or neither for the general list.
func sectionLines(block []string, branch, category string) []string {
	switch {
	case branch != "":
		return append([]string{"# Branches", "", "## " + branch, ""}, block...)
	case category != "":
		return append([]string{"# " + category, ""}, block...)
	}
	return block
}

// withFinalNewline ends the last line, in the file's line ending, so lines
// added at the end are separated from it.
func withFinalNewline(lines []string) []string {
	if last := len(lines) - 1; lines[last] != "" {
		lines[last] += lineEnding(lines)
		lines = append(lines, "")
	}
	return lines
}

// lineEnding is "\r" when the file's first line ends with "\r\n", so the
// lines the app adds match a Windows file. Lines already in the file keep
// their own endings.
func lineEnding(lines []string) string {
	if len(lines) > 1 && strings.HasSuffix(lines[0], "\r") {
		return "\r"
	}
	return ""
}

// withEnding copies lines, adding eol to those without a "\r" ending.
func withEnding(lines []string, eol string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		if !strings.HasSuffix(line, "\r") {
			line += eol
		}
		result[i] = line
	}
	return result
}

func parseHeading(line string) (int, string, bool) {
	parts := markdownHeading.FindStringSubmatch(line)
	if parts == nil {
		return 0, "", false
	}
	name := strings.TrimSpace(parts[2])
	closing := len(name)
	for closing > 0 && name[closing-1] == '#' {
		closing--
	}
	if closing > 0 && closing < len(name) && (name[closing-1] == ' ' || name[closing-1] == '\t') {
		name = strings.TrimSpace(name[:closing])
	}
	return len(parts[1]), name, true
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
		if !strings.EqualFold(NormalizeCategory(name), category) {
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

// insertBlockAt adds block before lines[idx], separated from its neighbours
// by blank lines. lines must end with a newline, as withFinalNewline leaves
// them, and idx must come before it.
func insertBlockAt(lines []string, idx int, block []string) string {
	eol := lineEnding(lines)
	addition := withEnding(block, eol)
	if idx > 0 && strings.TrimSpace(lines[idx-1]) != "" {
		addition = append([]string{eol}, addition...)
	}
	if strings.TrimSpace(lines[idx]) != "" {
		addition = append(addition, eol)
	}
	return strings.Join(insertLines(lines, idx, addition), "\n")
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

func normalizedTaskLine(selected Task, title string, done bool, p Priority) string {
	indent := taskIndent(selected.raw)
	mark := " "
	if done {
		mark = "x"
	}
	ending := ""
	if strings.HasSuffix(selected.raw, "\r") {
		ending = "\r"
	}
	return indent + "- [" + mark + "] " + title + priorityToken(p) + ending
}

// Toggle flips selected between open and done.
func Toggle(path string, selected Task) error {
	return rewriteTask(path, selected, func(lines []string) string {
		lines[selected.Line] = normalizedTaskLine(selected, selected.Text, !selected.Done, selected.Priority)
		return sortSection(strings.Join(lines, "\n"), selected.Category, selected.Branch)
	})
}

// SetPriority rewrites the !priority at the end of selected's task line.
func SetPriority(path string, selected Task, p Priority) error {
	return rewriteTask(path, selected, func(lines []string) string {
		lines[selected.Line] = normalizedTaskLine(selected, selected.Text, selected.Done, p)
		return sortSection(strings.Join(lines, "\n"), selected.Category, selected.Branch)
	})
}

// SetCategory moves selected under the heading for category, or to the
// general list when category is blank.
func SetCategory(path string, selected Task, category string) error {
	if selected.Branch != "" {
		return errors.New("branch tasks do not have categories")
	}
	category, _, err := validScope(category, "")
	if err != nil {
		return err
	}
	if strings.EqualFold(selected.Category, category) {
		return nil
	}
	return rewriteTask(path, selected, func(lines []string) string {
		return sortSection(movedTaskLines(lines, selected, taskBlock(selected, selected.Text, selected.Details), category, ""), category, "")
	})
}

func removeEmptyCategoryHeading(lines []string, selected Task) []string {
	if selected.Category == "" || selected.categoryLine < 0 || selected.categoryLine >= len(lines) {
		return lines
	}
	start := selected.categoryLine
	_, name, ok := parseHeading(lines[start])
	if !ok || !strings.EqualFold(NormalizeCategory(name), selected.Category) {
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

func removeEmptyBranchHeading(lines []string, branch string) []string {
	start, end, level := findBranchesSection(lines)
	if start < 0 {
		return lines
	}
	branchStart, branchEnd := findBranchSection(lines, start, end, level, branch)
	if branchStart < 0 {
		return lines
	}
	for _, line := range lines[branchStart+1 : branchEnd] {
		if strings.TrimSpace(line) != "" {
			return lines
		}
	}
	return append(append([]string{}, lines[:branchStart]...), lines[branchEnd:]...)
}

func replaceFile(path string, data []byte, mode os.FileMode) error {
	path, err := linkTarget(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".todo-*")
	if err != nil {
		return err
	}
	// After a successful rename the temp name is gone, so this only cleans up failures.
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := writeSynced(tmp, data, mode); err != nil {
		_ = tmp.Close() // The write error is the one worth reporting.
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// linkTarget follows symlinks from path, so a write replaces the file a
// link points to rather than the link. A link to a file not yet created
// names that file; any other missing path is used as it is.
func linkTarget(path string) (string, error) {
	target, err := filepath.EvalSymlinks(path)
	if err == nil {
		return target, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	link, err := os.Readlink(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	return link, nil
}

func writeSynced(f *os.File, data []byte, mode os.FileMode) error {
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
