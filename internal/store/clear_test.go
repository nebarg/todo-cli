package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAndLoad(t *testing.T, content string) (string, []Task) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, tasks
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func doneTasks(tasks []Task) []Task {
	var done []Task
	for _, task := range tasks {
		if task.Done {
			done = append(done, task)
		}
	}
	return done
}

func TestRemoveDoneDropsTasksAndEmptyHeadings(t *testing.T) {
	for _, item := range []struct {
		name, content, want  string
		categories, branches []string
	}{
		{
			name:    "general tasks with details",
			content: "- [x] Done\n  - Priority: High\n\n  Some details.\n\n- [ ] Open\n\n- [X] Also done\n",
			want:    "- [ ] Open\n",
		},
		{
			name:       "category left empty",
			content:    "- [ ] Open\n\n# docs\n\n- [x] Done\n\n- [x] Done too\n\n# auth\n\n- [ ] Login\n",
			want:       "- [ ] Open\n\n# auth\n\n- [ ] Login\n",
			categories: []string{"docs"},
		},
		{
			name:    "category with notes stays",
			content: "# docs\n\nKeep this note.\n\n- [x] Done\n",
			want:    "# docs\n\nKeep this note.\n",
		},
		{
			name:     "branch left empty keeps other branches",
			content:  "- [ ] Open\n\n# Branches\n\n## feature/a\n\n- [x] Done\n\n## feature/b\n\n- [ ] Branch open\n",
			want:     "- [ ] Open\n\n# Branches\n\n## feature/b\n\n- [ ] Branch open\n",
			branches: []string{"feature/a"},
		},
		{
			name:       "last branch removes the Branches heading",
			content:    "# docs\n\n- [x] Doc\n\n# Branches\n\n## feature/a\n\n- [x] Done\n\n## feature/b\n\n- [x] Also done\n",
			want:       "",
			categories: []string{"docs"},
			branches:   []string{"feature/a", "feature/b"},
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			path, tasks := writeAndLoad(t, item.content)
			r, err := PlanRemove(path, doneTasks(tasks))
			if err != nil {
				t.Fatal(err)
			}
			if readFile(t, path) != item.content {
				t.Fatal("planning wrote the file")
			}
			if strings.Join(r.Categories, ",") != strings.Join(item.categories, ",") || strings.Join(r.Branches, ",") != strings.Join(item.branches, ",") {
				t.Errorf("empty headings = %q %q, want %q %q", r.Categories, r.Branches, item.categories, item.branches)
			}
			if err := r.Apply(); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, path); got != item.want {
				t.Errorf("file = %q, want %q", got, item.want)
			}
			if err := r.Undo(); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, path); got != item.content {
				t.Errorf("undo = %q, want %q", got, item.content)
			}
		})
	}
}

func TestRemoveDoneOnlyTouchesGivenTasks(t *testing.T) {
	path, tasks := writeAndLoad(t, "- [x] General done\n\n# docs\n\n- [x] Doc done\n\n- [ ] Doc open\n")
	var docs []Task
	for _, task := range tasks {
		if task.Category == "docs" {
			docs = append(docs, task)
		}
	}
	r, err := PlanRemove(path, doneTasks(docs))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tasks) != 1 || r.Tasks[0].Text != "Doc done" {
		t.Fatalf("planned tasks = %+v", r.Tasks)
	}
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "- [x] General done\n\n# docs\n\n- [ ] Doc open\n" {
		t.Fatalf("file = %q", got)
	}
}

func TestRemoveDeletesAWholeBranchWithOpenTasks(t *testing.T) {
	path, tasks := writeAndLoad(t, "- [ ] Keep\n\n# Branches\n\n## gone\n\n- [ ] Open\n  - Priority: High\n\n  Details.\n\n- [x] Done\n\n## kept\n\n- [ ] Stays\n")
	var gone []Task
	for _, task := range tasks {
		if task.Branch == "gone" {
			gone = append(gone, task)
		}
	}
	r, err := PlanRemove(path, gone)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "- [ ] Keep\n\n# Branches\n\n## kept\n\n- [ ] Stays\n" || strings.Join(r.Branches, ",") != "gone" {
		t.Fatalf("file = %q, branches = %q", got, r.Branches)
	}
}

func TestRemoveDoneRefusesChangedFiles(t *testing.T) {
	path, tasks := writeAndLoad(t, "- [x] Done\n- [ ] Open\n")
	if err := os.WriteFile(path, []byte("- [ ] Done\n- [ ] Open\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanRemove(path, doneTasks(tasks)); !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("planning over a changed task = %v", err)
	}

	path, tasks = writeAndLoad(t, "- [x] Done\n- [ ] Open\n")
	r, err := PlanRemove(path, doneTasks(tasks))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [x] Done\n- [ ] Open\n- [ ] New\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(); !errors.Is(err, ErrFileChanged) {
		t.Fatalf("applying over a changed file = %v", err)
	}

	if err := os.WriteFile(path, []byte("- [x] Done\n- [ ] Open\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [ ] Open\n- [ ] Added after\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := r.Undo(); !errors.Is(err, ErrFileChanged) {
		t.Fatalf("undoing over a changed file = %v", err)
	}
	if got := readFile(t, path); got != "- [ ] Open\n- [ ] Added after\n" {
		t.Fatalf("refused undo still wrote the file: %q", got)
	}
}
