// Package store reads and edits tasks in a Markdown task file, preserving
// everything it does not change.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var taskLine = regexp.MustCompile(`^([ \t]*- )(?:\[([ xX])\] +)?(.*)$`)
var markdownHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.+?)\s*\r?$`)

// ErrTaskChanged means the file no longer matches the task that was read,
// so the edit was refused rather than risk changing the wrong lines.
var ErrTaskChanged = errors.New("task changed on disk")

// Task is one Markdown checklist item. The unexported fields record exactly
// what was read, so writes can refuse to touch a task that changed on disk.
type Task struct {
	// Line is the index of the task's line in the file, so the first line is 0.
	Line int
	Text string
	Done bool
	Section
	Priority Priority
	Details  string

	raw          string
	categoryLine int
	bodyStart    int
	bodyEnd      int
	bodyRaw      []string
}

// Section is where a task is filed: under a category, under a branch, or
// with both blank, in the general list. A branch task has no category.
type Section struct {
	Category string
	Branch   string
}

// Same reports whether s and other are the same section. Category names
// ignore case, as their headings do; branch names don't.
func (s Section) Same(other Section) bool {
	return s.Branch == other.Branch && strings.EqualFold(s.Category, other.Category)
}

// normalized checks a destination section, trimming the branch and
// normalizing the category. A blank category is the general list.
func (s Section) normalized() (Section, error) {
	branch := strings.TrimSpace(s.Branch)
	if strings.ContainsAny(branch, "\r\n") {
		return Section{}, errors.New("branch name must be one line")
	}
	if strings.TrimSpace(s.Category) == "" {
		return Section{Branch: branch}, nil
	}
	if strings.ContainsAny(s.Category, "\r\n") {
		return Section{}, errors.New("category must be one line")
	}
	category := NormalizeCategory(s.Category)
	if err := validateCategory(category); err != nil {
		return Section{}, err
	}
	if branch != "" {
		return Section{}, errors.New("branch tasks cannot have a category")
	}
	return Section{Category: category}, nil
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
	_, text := splitBOM(data)
	return parseTasks(strings.Split(text, "\n")), nil
}

// byteOrderMark starts some UTF-8 files, such as those Windows editors may
// save. It is set aside while a file is read, so it doesn't hide the first
// line, and written back with the file.
const byteOrderMark = "\ufeff"

// splitBOM separates a file's byte order mark, if it has one, from its text.
func splitBOM(data []byte) (bom, text string) {
	if text, ok := strings.CutPrefix(string(data), byteOrderMark); ok {
		return byteOrderMark, text
	}
	return "", string(data)
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
		t.bodyRaw = slices.Clone(lines[t.bodyStart:t.bodyEnd])
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

// Add writes a new open task, filed under to, and creates the file or section
// if needed. A trailing !priority in title is used when p is PriorityNone.
func Add(path, title, details string, p Priority, to Section) error {
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
	to, err := to.normalized()
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
	bom, text := splitBOM(data)
	updated := sortSection(insertTaskBlock(text, block, to), to)
	return replaceFile(path, []byte(bom+updated), mode)
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
// unless title ends with a new !priority, and files it under to. A task that
// keeps its section is edited in place.
func Edit(path string, selected Task, title, details string, to Section) error {
	title, p := SplitPriority(strings.TrimSpace(title))
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return errors.New("enter a single-line task title")
	}
	if p != PriorityNone {
		selected.Priority = p
	}
	to, err := to.normalized()
	if err != nil {
		return err
	}
	return rewriteTask(path, selected, func(lines []string) string {
		if to.Same(selected.Section) {
			return sortSection(strings.Join(editedTaskLines(lines, selected, title, details), "\n"), to)
		}
		return sortSection(movedTaskLines(lines, selected, taskBlock(selected, title, details), to), to)
	})
}

// taskBlock is selected's lines with a new title and details, keeping the
// original details text when it has not changed.
func taskBlock(selected Task, title, details string) []string {
	block := []string{normalizedTaskLine(selected, title, selected.Done, selected.Priority)}
	if strings.Join(formattedDetails(details), "\n") == strings.Join(formattedDetails(selected.Details), "\n") {
		body := slices.Clone(selected.bodyRaw)
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
// and files block under to.
func movedTaskLines(lines []string, selected Task, block []string, to Section) string {
	remaining := slices.Concat(lines[:selected.Line], lines[selected.bodyEnd:])
	remaining = removeEmptyCategoryHeading(remaining, selected)
	if selected.Branch != "" && selected.Branch != to.Branch {
		remaining = removeEmptyBranchHeading(remaining, selected.Branch)
	}
	return insertTaskBlock(strings.Join(remaining, "\n"), block, to)
}

func editedTaskLines(lines []string, selected Task, title, details string) []string {
	updated := slices.Clone(lines[:selected.Line])
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
	bom, text := splitBOM(data)
	lines := strings.Split(text, "\n")
	if !taskUnchanged(lines, selected) {
		return ErrTaskChanged
	}
	return replaceFile(path, []byte(bom+change(lines)), info.Mode().Perm())
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

// validateCategory checks a normalized category can be written as a heading
// that reads back as the same category.
func validateCategory(category string) error {
	if category == "" {
		return errors.New("enter a category name")
	}
	if strings.EqualFold(category, "Branches") {
		return errors.New(`"Branches" is reserved for the branch section`)
	}
	// A heading drops a closing " #", and a second leading @ or #.
	if _, name, _ := parseHeading("# " + category); NormalizeCategory(name) != category {
		return fmt.Errorf("%q can't be a category: its heading would read as %q", category, NormalizeCategory(name))
	}
	return nil
}

// NormalizeCategory drops one leading @ or # from a category name, and
// writes each run of whitespace in it as one space, so names that read the
// same in a heading are the same category.
func NormalizeCategory(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 1 && (raw[0] == '@' || raw[0] == '#') {
		raw = raw[1:]
	}
	return strings.Join(strings.Fields(raw), " ")
}

func insertTaskBlock(data string, block []string, to Section) string {
	if strings.TrimSpace(data) == "" {
		return strings.Join(sectionLines(block, to), "\n") + "\n"
	}
	lines := withFinalNewline(strings.Split(data, "\n"))
	if to.Branch != "" {
		start, end, level := findBranchesSection(lines)
		if start < 0 {
			return insertBlockAt(lines, len(lines)-1, sectionLines(block, to))
		}
		branchStart, branchEnd := findBranchSection(lines, start, end, level, to.Branch)
		if branchStart < 0 {
			return insertBlockAt(lines, trimBlankEnd(lines, end, start+1), append([]string{strings.Repeat("#", level+1) + " " + to.Branch, ""}, block...))
		}
		for i := branchStart + 1; i < branchEnd; i++ {
			if _, _, ok := parseHeading(lines[i]); ok {
				branchEnd = i // New branch tasks stay outside old nested headings.
				break
			}
		}
		return insertBlockAt(lines, trimBlankEnd(lines, branchEnd, branchStart+1), block)
	}
	if to.Category == "" {
		end := len(lines)
		for i, line := range lines {
			if _, _, ok := parseHeading(line); ok {
				end = i
				break
			}
		}
		return insertBlockAt(lines, trimBlankEnd(lines, end, 0), block)
	}
	if start, end := findCategorySection(lines, to.Category); start >= 0 {
		return insertBlockAt(lines, trimBlankEnd(lines, end, start+1), block)
	}
	idx := len(lines)
	if branchesStart, _, _ := findBranchesSection(lines); branchesStart >= 0 {
		idx = branchesStart
	}
	return insertBlockAt(lines, trimBlankEnd(lines, idx, 0), sectionLines(block, to))
}

// sectionLines is block under the headings of a new section: a branch in a
// new Branches section, a category, or neither for the general list.
func sectionLines(block []string, s Section) []string {
	switch {
	case s.Branch != "":
		return append([]string{"# Branches", "", "## " + s.Branch, ""}, block...)
	case s.Category != "":
		return append([]string{"# " + s.Category, ""}, block...)
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
		return sortSection(strings.Join(lines, "\n"), selected.Section)
	})
}

// SetPriority rewrites the !priority at the end of selected's task line.
func SetPriority(path string, selected Task, p Priority) error {
	return rewriteTask(path, selected, func(lines []string) string {
		lines[selected.Line] = normalizedTaskLine(selected, selected.Text, selected.Done, p)
		return sortSection(strings.Join(lines, "\n"), selected.Section)
	})
}

// SetCategory moves selected under the heading for category, or to the
// general list when category is blank.
func SetCategory(path string, selected Task, category string) error {
	if selected.Branch != "" {
		return errors.New("branch tasks do not have categories")
	}
	to, err := Section{Category: category}.normalized()
	if err != nil {
		return err
	}
	if to.Same(selected.Section) {
		return nil
	}
	return rewriteTask(path, selected, func(lines []string) string {
		return sortSection(movedTaskLines(lines, selected, taskBlock(selected, selected.Text, selected.Details), to), to)
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
	return slices.Concat(lines[:start], lines[end:])
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
	return slices.Concat(lines[:branchStart], lines[branchEnd:])
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
