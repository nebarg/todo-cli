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
var markdownHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.+?)\s*$`)

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
	// Details is the text under the task, without its subtasks.
	Details string
	// Subtask is true for a checkbox indented under another task, whose
	// section it shares. Its task is the last one before it that isn't a
	// subtask, and the lines indented under it are its details.
	Subtask bool

	raw          string
	categoryLine int
	bodyStart    int
	bodyEnd      int
	bodyRaw      []string
}

// Holds reports whether other, from the same read of the file, is one of t's
// subtasks, or anything else under t, which goes wherever t does.
func (t Task) Holds(other Task) bool {
	return t.Line < other.Line && other.Line < t.bodyEnd
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
	lines, _ := decode(data)
	return parseTasks(lines), nil
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

// fileFormat is what a file has besides its lines: the byte order mark it
// starts with, and whether its lines end in "\r\n". decode sets both aside,
// so the rest of the store only sees lines, and encode puts them back.
type fileFormat struct {
	bom  string
	crlf bool
}

// decode reads a file's bytes as lines, with every "\r\n" ending read as
// "\n", and the format to write them back in. A file that ends with a newline
// ends with an empty line. A file is CRLF when its first line ends with
// "\r\n" and it has another line, so a Windows file stays one. A file whose
// endings are mixed is written back with its first line's.
func decode(data []byte) ([]string, fileFormat) {
	bom, text := splitBOM(data)
	first, _, more := strings.Cut(text, "\n")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return lines, fileFormat{bom: bom, crlf: more && strings.HasSuffix(first, "\r")}
}

// encode is lines, as decode returned them and an edit changed them, in the
// form the file is written in.
func (f fileFormat) encode(lines []string) string {
	text := strings.Join(lines, "\n")
	if f.crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return f.bom + text
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
		t.Text, t.Priority = SplitPriority(parts[3])
		indent := taskIndent(raw)
		j := i + 1
		for ; j < len(lines); j++ {
			line := lines[j]
			if _, _, heading := parseHeading(line); heading {
				break
			}
			if parts := taskLine.FindStringSubmatch(line); parts != nil && strings.TrimSpace(parts[3]) != "" && !strings.HasPrefix(line, indent+"  ") {
				break
			}
		}
		t.bodyStart, t.bodyEnd = i+1, j
		t.bodyRaw = slices.Clone(lines[t.bodyStart:t.bodyEnd])
		spans := subtaskSpans(lines, t.bodyStart, t.bodyEnd)
		t.Details = detailText(outsideSpans(lines, t.bodyStart, t.bodyEnd, spans), indent)
		tasks = append(tasks, t)
		for _, span := range spans {
			tasks = append(tasks, subtask(lines, span, t))
		}
		i = j - 1
	}
	return tasks
}

// subtaskSpans finds the subtasks in lines[start:end], a task's body: each
// checkbox item in it, from its line up to the next line indented no deeper,
// or to the body's last line that isn't blank.
func subtaskSpans(lines []string, start, end int) [][2]int {
	var spans [][2]int
	for i := start; i < end; i++ {
		parts := taskLine.FindStringSubmatch(lines[i])
		if parts == nil || parts[2] == "" || strings.TrimSpace(parts[3]) == "" {
			continue
		}
		indent := len(taskIndent(lines[i]))
		spanEnd := trimBlankEnd(lines, end, i+1)
		for j := i + 1; j < spanEnd; j++ {
			if strings.TrimSpace(lines[j]) != "" && len(taskIndent(lines[j])) <= indent {
				spanEnd = j
				break
			}
		}
		spans = append(spans, [2]int{i, spanEnd})
		i = spanEnd - 1
	}
	return spans
}

// outsideSpans is lines[start:end] without the lines of spans.
func outsideSpans(lines []string, start, end int, spans [][2]int) []string {
	var outside []string
	for _, span := range spans {
		outside = append(outside, lines[start:span[0]]...)
		start = span[1]
	}
	return append(outside, lines[start:end]...)
}

// subtask is the subtask of parent on the lines span covers.
func subtask(lines []string, span [2]int, parent Task) Task {
	raw := lines[span[0]]
	parts := taskLine.FindStringSubmatch(raw)
	s := Task{
		Line: span[0], raw: raw, Done: parts[2] == "x" || parts[2] == "X", Subtask: true,
		Section: parent.Section, categoryLine: parent.categoryLine,
		bodyStart: span[0] + 1, bodyEnd: span[1],
	}
	s.Text, s.Priority = SplitPriority(parts[3])
	s.bodyRaw = slices.Clone(lines[s.bodyStart:s.bodyEnd])
	s.Details = detailText(s.bodyRaw, taskIndent(raw))
	return s
}

// detailText is a task's details from the lines under it: without the blank
// lines around them, or the indent of the task's text.
func detailText(lines []string, indent string) string {
	lines = trimBlankLines(lines)
	body := make([]string, len(lines))
	for i, line := range lines {
		body[i] = strings.TrimPrefix(line, indent+"  ")
	}
	return strings.Join(body, "\n")
}

// subtaskLines is the lines of the subtasks in a task's body, as written.
func subtaskLines(body []string) []string {
	var lines []string
	for _, span := range subtaskSpans(body, 0, len(body)) {
		lines = append(lines, body[span[0]:span[1]]...)
	}
	return lines
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
	lines, format := decode(data)
	updated := sortSection(insertTaskBlock(lines, block, to), to)
	return replaceFile(path, []byte(format.encode(updated)), mode)
}

// AddSubtask writes a new open subtask under parent, after its last subtask,
// or after its details with a blank line between. A trailing !priority in
// title sets its priority.
func AddSubtask(path string, parent Task, title string) error {
	if parent.Subtask {
		return errors.New("subtasks don't have subtasks; add it to their task")
	}
	title, p := SplitPriority(strings.TrimSpace(title))
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return errors.New("enter a single-line subtask")
	}
	return rewriteTask(path, parent, func(lines []string) []string {
		at, block := parent.Line+1, []string{taskIndent(parent.raw) + "  - [ ] " + title + priorityToken(p)}
		if spans := subtaskSpans(lines, parent.bodyStart, parent.bodyEnd); len(spans) > 0 {
			at = spans[len(spans)-1][1]
		} else if parent.Details != "" {
			at = trimBlankEnd(lines, parent.bodyEnd, parent.bodyStart)
			block = append([]string{""}, block...)
		}
		return insertLines(lines, at, block)
	})
}

func formattedDetails(details string) []string {
	details = strings.ReplaceAll(details, "\r\n", "\n")
	rows := trimBlankLines(strings.Split(details, "\n"))
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
// keeps its section is edited in place, and its subtasks go wherever it does.
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
	if selected.Subtask && !to.Same(selected.Section) {
		return errSubtaskSection
	}
	return rewriteTask(path, selected, func(lines []string) []string {
		if to.Same(selected.Section) {
			return sortSection(editedTaskLines(lines, selected, title, details), to)
		}
		return sortSection(movedTaskLines(lines, selected, taskBlock(selected, title, details), to), to)
	})
}

// errSubtaskSection refuses to move a subtask out from under its task.
var errSubtaskSection = errors.New("a subtask stays with its task; move the task instead")

// taskBlock is selected's lines with a new title and details, keeping the
// original details text when it has not changed.
func taskBlock(selected Task, title, details string) []string {
	block := []string{normalizedTaskLine(selected, title, selected.Done, selected.Priority)}
	if sameDetails(selected, details) {
		body := selected.bodyRaw
		return append(block, body[:trimBlankEnd(body, len(body), 0)]...)
	}
	return append(block, newBody(selected, details)...)
}

// sameDetails reports whether details, as typed into the form, are
// selected's own, once both are formatted as the app writes them.
func sameDetails(selected Task, details string) bool {
	return slices.Equal(formattedDetails(details), formattedDetails(selected.Details))
}

// newBody is the lines under selected with details in place of its own: a
// blank line and the details, as the app writes them, then its subtasks as
// they were.
func newBody(selected Task, details string) []string {
	var body []string
	if formatted := formattedDetails(details); len(formatted) > 0 {
		body = append(append(body, ""), formatted...)
	}
	if subtasks := subtaskLines(selected.bodyRaw); len(subtasks) > 0 {
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, subtasks...)
	}
	return body
}

// movedTaskLines removes selected from lines, drops a heading it leaves empty,
// and files block under to.
func movedTaskLines(lines []string, selected Task, block []string, to Section) []string {
	remaining := slices.Concat(lines[:selected.Line], lines[selected.bodyEnd:])
	remaining = removeEmptyCategoryHeading(remaining, selected)
	if selected.Branch != "" && selected.Branch != to.Branch {
		remaining = removeEmptyBranchHeading(remaining, selected.Branch)
	}
	return insertTaskBlock(remaining, block, to)
}

func editedTaskLines(lines []string, selected Task, title, details string) []string {
	updated := slices.Clone(lines[:selected.Line])
	updated = append(updated, normalizedTaskLine(selected, title, selected.Done, selected.Priority))
	if sameDetails(selected, details) {
		updated = append(updated, selected.bodyRaw...)
	} else if body := newBody(selected, details); len(body) > 0 {
		updated = append(updated, body...)
		updated = append(updated, "")
	} else if selected.bodyEnd < len(lines) {
		updated = append(updated, "")
	}
	return append(updated, lines[selected.bodyEnd:]...)
}

// rewriteTask replaces the file with change(lines), refusing when selected no
// longer matches the file so edits never land on the wrong task.
func rewriteTask(path string, selected Task, change func(lines []string) []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	lines, format := decode(data)
	if !taskUnchanged(lines, selected) {
		return ErrTaskChanged
	}
	return replaceFile(path, []byte(format.encode(change(lines))), info.Mode().Perm())
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

func insertTaskBlock(lines, block []string, to Section) []string {
	if allBlank(lines) {
		return slices.Concat(sectionLines(block, to), []string{""})
	}
	lines = withFinalNewline(lines)
	if to.Branch != "" {
		start, end, level := findBranchesSection(lines)
		if start < 0 {
			return insertBlockAt(lines, len(lines)-1, sectionLines(block, to))
		}
		branchStart, branchEnd := findBranchSection(lines, start, end, level, to.Branch)
		if branchStart < 0 {
			return insertBlockAt(lines, trimBlankEnd(lines, end, start+1), append([]string{strings.Repeat("#", level+1) + " " + to.Branch, ""}, block...))
		}
		// New branch tasks stay outside old nested headings.
		branchEnd = nextHeading(lines[:branchEnd], branchStart+1, anyLevel)
		return insertBlockAt(lines, trimBlankEnd(lines, branchEnd, branchStart+1), block)
	}
	if to.Category == "" {
		return insertBlockAt(lines, trimBlankEnd(lines, nextHeading(lines, 0, anyLevel), 0), block)
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

// withFinalNewline ends the last line, so lines added at the end are
// separated from it.
func withFinalNewline(lines []string) []string {
	if lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return lines
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

// anyLevel is the deepest heading level, so as nextHeading's maxLevel it
// stops at a heading of any level.
const anyLevel = 6

// nextHeading is where a section that starts before index from ends: the
// index of the first line from there on that is a heading of level maxLevel
// or shallower, or len(lines) if none is.
func nextHeading(lines []string, from, maxLevel int) int {
	for i := from; i < len(lines); i++ {
		if level, _, ok := parseHeading(lines[i]); ok && level <= maxLevel {
			return i
		}
	}
	return len(lines)
}

func findBranchesSection(lines []string) (int, int, int) {
	for i, line := range lines {
		level, name, ok := parseHeading(line)
		if !ok || !strings.EqualFold(name, "Branches") || (level != 1 && level != 2) {
			continue
		}
		return i, nextHeading(lines, i+1, level), level
	}
	return -1, -1, 0
}

func findBranchSection(lines []string, start, end, level int, branch string) (int, int) {
	for i := start + 1; i < end; i++ {
		currentLevel, name, ok := parseHeading(lines[i])
		if !ok || currentLevel != level+1 || name != branch {
			continue
		}
		return i, nextHeading(lines[:end], i+1, currentLevel)
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
		return i, nextHeading(lines, i+1, anyLevel)
	}
	return -1, -1
}

// insertBlockAt adds block before lines[idx], separated from its neighbours
// by blank lines. lines must end with a newline, as withFinalNewline leaves
// them, and idx must come before it.
func insertBlockAt(lines []string, idx int, block []string) []string {
	var addition []string
	if idx > 0 && strings.TrimSpace(lines[idx-1]) != "" {
		addition = append(addition, "")
	}
	addition = append(addition, block...)
	if strings.TrimSpace(lines[idx]) != "" {
		addition = append(addition, "")
	}
	return insertLines(lines, idx, addition)
}

func trimBlankEnd(lines []string, end, minimum int) int {
	for end > minimum && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

// trimBlankLines is lines without the blank lines at either end.
func trimBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return lines[start:trimBlankEnd(lines, len(lines), start)]
}

// allBlank reports whether every one of lines is blank, as when there are none.
func allBlank(lines []string) bool {
	return !slices.ContainsFunc(lines, func(line string) bool { return strings.TrimSpace(line) != "" })
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
	return indent + "- [" + mark + "] " + title + priorityToken(p)
}

// Toggle flips selected between open and done.
func Toggle(path string, selected Task) error {
	return rewriteTask(path, selected, func(lines []string) []string {
		lines[selected.Line] = normalizedTaskLine(selected, selected.Text, !selected.Done, selected.Priority)
		return sortSection(lines, selected.Section)
	})
}

// SetPriority rewrites the !priority at the end of selected's task line.
func SetPriority(path string, selected Task, p Priority) error {
	return rewriteTask(path, selected, func(lines []string) []string {
		lines[selected.Line] = normalizedTaskLine(selected, selected.Text, selected.Done, p)
		return sortSection(lines, selected.Section)
	})
}

// SetCategory moves selected under the heading for category, or to the
// general list when category is blank.
func SetCategory(path string, selected Task, category string) error {
	if selected.Subtask {
		return errSubtaskSection
	}
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
	return rewriteTask(path, selected, func(lines []string) []string {
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
	end := nextHeading(lines, start+1, anyLevel)
	if !allBlank(lines[start+1 : end]) {
		return lines
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
	if !allBlank(lines[branchStart+1 : branchEnd]) {
		return lines
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
