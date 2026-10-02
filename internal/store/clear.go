package store

import (
	"cmp"
	"errors"
	"os"
	"slices"
	"strings"
)

// ErrFileChanged means the file is no longer the one a removal was planned
// from, or undone to, so it was left alone.
var ErrFileChanged = errors.New("file changed on disk")

// Removal is a planned deletion of tasks. Apply writes it, and Undo puts
// the file back as long as nothing else has changed it in between.
type Removal struct {
	Tasks      []Task
	Categories []string
	Branches   []string

	path, before, after string
	mode                os.FileMode
}

// PlanRemove works out the file without tasks, and which category and branch
// headings they leave empty, without writing it.
func PlanRemove(path string, tasks []Task) (Removal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Removal{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Removal{}, err
	}
	r := Removal{Tasks: slices.Clone(tasks), path: path, before: string(data), mode: info.Mode().Perm()}
	// Removing from the bottom up keeps the line numbers of the tasks and
	// headings still to come valid.
	slices.SortFunc(r.Tasks, func(a, b Task) int { return cmp.Compare(b.Line, a.Line) })
	bom, text := splitBOM(data)
	lines := strings.Split(text, "\n")
	for _, t := range r.Tasks {
		if !taskUnchanged(lines, t) {
			return Removal{}, ErrTaskChanged
		}
		lines = append(lines[:t.Line:t.Line], lines[t.bodyEnd:]...)
		if kept := removeEmptyCategoryHeading(lines, t); len(kept) < len(lines) {
			lines = kept
			r.Categories = append(r.Categories, t.Category)
		}
		if t.Branch == "" {
			continue
		}
		if kept := removeEmptyBranchHeading(lines, t.Branch); len(kept) < len(lines) {
			lines = removeEmptyBranchesHeading(kept)
			r.Branches = append(r.Branches, t.Branch)
		}
	}
	slices.Sort(r.Categories)
	slices.Sort(r.Branches)
	r.after = bom + strings.Join(lines, "\n")
	return r, nil
}

// Apply writes the planned file, refusing if it changed since it was planned.
func (r Removal) Apply() error {
	return r.replace(r.before, r.after)
}

// Undo restores the file from before Apply, refusing if it changed since.
func (r Removal) Undo() error {
	return r.replace(r.after, r.before)
}

func (r Removal) replace(expected, updated string) error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}
	if string(data) != expected {
		return ErrFileChanged
	}
	return replaceFile(r.path, []byte(updated), r.mode)
}

func removeEmptyBranchesHeading(lines []string) []string {
	start, end, _ := findBranchesSection(lines)
	if start < 0 {
		return lines
	}
	for _, line := range lines[start+1 : end] {
		if strings.TrimSpace(line) != "" {
			return lines
		}
	}
	return slices.Concat(lines[:start], lines[end:])
}
