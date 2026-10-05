package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	original := "# Test TODOs\n\n- [ ] first\nSome context.\n- [x] second\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("loadTasks: %v, %d tasks", err, len(tasks))
	}
	if err := Toggle(path, tasks[0]); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "third", "", PriorityNone, Section{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "- [ ] third\n\n# Test TODOs\n\n- [x] first\nSome context.\n- [x] second\n"
	if string(got) != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestBranchesAndMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := Add(path, "general task", "", PriorityNone, Section{}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "branch task", "", "high", Section{Branch: "feature/login"}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "another branch task", "", "", Section{Branch: "feature/login"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "## feature/login") != 1 {
		t.Fatalf("duplicate branch heading: %s", data)
	}
	if !strings.Contains(string(data), "# Branches\n\n## feature/login\n") || strings.Contains(string(data), "General") {
		t.Fatalf("missing headings: %s", data)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 || tasks[1].Branch != "feature/login" || tasks[1].Priority != "high" || tasks[1].Category != "" {
		t.Fatalf("parsed tasks: %+v", tasks)
	}
	if err := Add(path, "invalid", "", "", Section{Category: "tests", Branch: "feature/login"}); err == nil {
		t.Fatal("branch category was accepted")
	}
	if err := SetPriority(path, tasks[1], "low"); err != nil {
		t.Fatal(err)
	}
	tasks, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range tasks {
		if item.Text == "branch task" && item.Priority != "low" {
			t.Fatalf("priority edit failed: %+v", tasks)
		}
		if item.Text == "branch task" && SetCategory(path, item, "smoke") == nil {
			t.Fatal("branch category edit was accepted")
		}
	}
	if tasks[1].Text != "branch task" || tasks[2].Text != "another branch task" {
		t.Fatalf("updated tasks: %+v", tasks)
	}
}

func TestCategoriesAllowSpacesAndSymbols(t *testing.T) {
	for _, c := range []struct{ category, err string }{
		{"line\nbreak", "category must be one line"},
		{"line\r\nbreak", "category must be one line"},
		{"Branches", `"Branches" is reserved`},
		{" branches ", `"Branches" is reserved`},
		{"C #", `"C #" can't be a category: its heading would read as "C"`},
		{"##x", `"#x" can't be a category: its heading would read as "x"`},
	} {
		t.Run(c.category, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "TODO.md")
			if err := Add(path, "Task", "", "", Section{Category: c.category}); err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("category %q was accepted or gave an unclear error: %v", c.category, err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("invalid category %q changed the file: %v", c.category, err)
			}
		})
	}
	for category, want := range map[string]string{
		"two words": "two words", " leading": "leading", "trailing ": "trailing", "tab\tname": "tab name",
		"@release  notes": "release notes", "# docs and FAQ": "docs and FAQ", "C# notes": "C# notes",
		"auth": "auth", "A1": "A1", "@tests2": "tests2", "#café3": "café3", "+v1": "+v1", "bug-fix": "bug-fix",
		"under_score": "under_score", "a?": "a?", "emoji🙂": "emoji🙂", "foo#": "foo#", "@": "@", "#": "#",
	} {
		t.Run(category, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "TODO.md")
			if err := Add(path, "Task", "", "", Section{Category: category}); err != nil {
				t.Fatalf("valid category %q rejected: %v", category, err)
			}
			tasks, err := Load(path)
			if err != nil || len(tasks) != 1 || tasks[0].Category != want {
				t.Fatalf("category %q was not saved as %q: %v, %+v", category, want, err, tasks)
			}
			if got := readFile(t, path); !strings.HasPrefix(got, "# "+want+"\n") {
				t.Fatalf("category %q has the heading %q", category, got)
			}
		})
	}
	if _, name, ok := parseHeading("# auth ###"); !ok || name != "auth" {
		t.Fatalf("closing Markdown hashes were not parsed correctly: %q", name)
	}
}

func TestSpacedCategoryMatchesItsHeadingWhateverItsCaseAndSpacing(t *testing.T) {
	path, tasks := writeAndLoad(t, "# Release  Notes\n\n- [ ] Existing\n")
	if len(tasks) != 1 || tasks[0].Category != "Release Notes" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if err := Add(path, "New", "", PriorityNone, Section{Category: "release notes"}); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), "# Release  Notes\n\n- [ ] Existing\n\n- [ ] New\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestHashHeadingIsTheSameCategory(t *testing.T) {
	path, tasks := writeAndLoad(t, "# #auth\n\n- [ ] Existing\n")
	if len(tasks) != 1 || tasks[0].Category != "auth" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if err := Add(path, "New", "", PriorityNone, Section{Category: "auth"}); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), "# #auth\n\n- [ ] Existing\n\n- [ ] New\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestWritesFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "notes", "todo.md")
	if err := os.Mkdir(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "todo.md")
	if err := os.Symlink(filepath.Join("notes", "todo.md"), link); err != nil {
		t.Fatal(err)
	}
	// The target doesn't exist yet, so the first task creates it.
	if err := Add(link, "First", "", PriorityNone, Section{}); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(link)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %+v, %v", tasks, err)
	}
	if err := Toggle(link, tasks[0]); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v, %v", info, err)
	}
	if got := readFile(t, target); got != "- [x] First\n" {
		t.Fatalf("target = %q", got)
	}
}

func TestChangingCategoryToSymbolName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := Add(path, "Task", "", "", Section{Category: "auth"}); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("initial category: %v, %+v", err, tasks)
	}
	if err := SetCategory(path, tasks[0], "+v1"); err != nil {
		t.Fatal(err)
	}
	tasks, err = Load(path)
	if err != nil || len(tasks) != 1 || tasks[0].Category != "+v1" {
		t.Fatalf("symbol category was not saved: %v, %+v", err, tasks)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "# +v1\n") || strings.Contains(string(data), "# auth\n") {
		t.Fatalf("category heading was not updated: %v, %q", err, data)
	}
}

func TestSpacedCategoryTasksCanBeEditedAndMoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("## General\n\n### @a category?\n\n- [ ] Existing task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 1 || tasks[0].Category != "a category?" {
		t.Fatalf("spaced category was not readable: %v, %+v", err, tasks)
	}
	if err := Edit(path, tasks[0], "Edited task", "With details.", Section{Category: tasks[0].Category}); err != nil {
		t.Fatalf("task in a spaced category could not be edited: %v", err)
	}
	if got, want := readFile(t, path), "## General\n\n### @a category?\n\n- [ ] Edited task\n\n  With details.\n"; got != want {
		t.Fatalf("edit in place = %q, want %q", got, want)
	}
	if tasks, err = Load(path); err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %+v, %v", tasks, err)
	}
	if err := SetCategory(path, tasks[0], "another  category"); err != nil {
		t.Fatal(err)
	}
	tasks, err = Load(path)
	if err != nil || len(tasks) != 1 || tasks[0].Category != "another category" {
		t.Fatalf("spaced category could not be renamed: %v, %+v", err, tasks)
	}
	if got := readFile(t, path); strings.Contains(got, "a category?") || !strings.Contains(got, "# another category\n") {
		t.Fatalf("file after the move = %q", got)
	}
}

func TestSingleCategoryHeadingsAndMoves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := Add(path, "Auth task", "Keep this detail.", "high", Section{Category: "auth"}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "General task", "", "", Section{}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "Branch plain", "", "", Section{Branch: "feature/login"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"# auth", "# Branches", "## feature/login", "- [ ] Auth task !high\n", "  Keep this detail."} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
	if strings.Index(s, "General task") > strings.Index(s, "# auth") || strings.Index(s, "Branch plain") < strings.Index(s, "## feature/login") {
		t.Fatalf("tasks were inserted under incorrect headings: %s", s)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	var selected Task
	for _, item := range tasks {
		if item.Text == "Auth task" {
			selected = item
		}
	}
	if err := SetCategory(path, selected, "docs"); err != nil {
		t.Fatal(err)
	}
	tasks, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range tasks {
		if item.Text == "Auth task" {
			found = item.Category == "docs" && item.Priority == "high" && item.Details == "Keep this detail."
		}
	}
	if !found {
		t.Fatalf("recategorising changed task content: %+v", tasks)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\n# auth\n") || !strings.Contains(string(data), "\n# docs\n") {
		t.Fatalf("recategorising left an empty heading or missed the new heading: %s", data)
	}
}

func TestTaskDetailsAreParsedAndPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	original := "## General\n\n- [ ] Fix login redirect !high\n\n  When a session expires, return to the previous page.\n\n  - [ ] Add a regression test\n\n- [ ] Another task\n\n## Branches\n\n### feature/login\n\n- [ ] Branch task\n\n  Branch-specific context.\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 4 {
		t.Fatalf("got %d tasks, want 4: %+v", len(tasks), tasks)
	}
	// The indented checkbox is a subtask, not part of the details.
	wantDetails := "When a session expires, return to the previous page."
	if tasks[0].Details != wantDetails || !tasks[1].Subtask || tasks[1].Text != "Add a regression test" || tasks[3].Details != "Branch-specific context." {
		t.Fatalf("parsed details: %+v", tasks)
	}
	if err := SetPriority(path, tasks[0], "low"); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "- [ ] Fix login redirect !low\n\n  When a session expires, return to the previous page.\n\n  - [ ] Add a regression test") {
		t.Fatalf("details changed while editing priority: %s", updated)
	}
	tasks, err = Load(path)
	if err != nil || len(tasks) != 4 {
		t.Fatalf("reloaded details: %v, %+v", err, tasks)
	}
	var selected Task
	for _, item := range tasks {
		if item.Text == "Fix login redirect" {
			selected = item
		}
	}
	if selected.Details != wantDetails {
		t.Fatalf("reloaded details: %+v", tasks)
	}
	if err := Toggle(path, selected); err != nil {
		t.Fatal(err)
	}
	updated, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "- [x] Fix login redirect") || !strings.Contains(string(updated), "  - [ ] Add a regression test") {
		t.Fatalf("details changed while completing task: %s", updated)
	}
}

func TestEditTaskContentPreservesMetadataAndOtherTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	original := "## General\n\n- [ ] First task !high\n\n  Old details.\n\n- [ ] Second task\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Edit(path, tasks[0], "Renamed task", "New context.\n\n- [ ] Nested step", tasks[0].Section); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- [ ] Renamed task !high", "  New context.\n\n  - [ ] Nested step", "- [ ] Second task"} {
		if !strings.Contains(string(updated), want) {
			t.Fatalf("missing %q after edit: %s", want, updated)
		}
	}
	// A checkbox written into the details reads back as a subtask, which
	// clearing the details leaves alone.
	tasks, err = Load(path)
	if err != nil || len(tasks) != 3 || tasks[0].Priority != "high" || tasks[0].Details != "New context." || !tasks[1].Subtask || tasks[1].Text != "Nested step" {
		t.Fatalf("reloaded tasks: %v, %+v", err, tasks)
	}
	if err := Edit(path, tasks[0], "Renamed task", "", tasks[0].Section); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), "## General\n\n- [ ] Renamed task !high\n  - [ ] Nested step\n\n- [ ] Second task\n"; got != want {
		t.Fatalf("after clearing the details:\n%q\nwant\n%q", got, want)
	}
}

func TestEditTaskContentRejectsChangedBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] First\n\n  Original details.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [ ] First\n\n  Edited elsewhere.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Edit(path, tasks[0], "New title", "My edit", tasks[0].Section); !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("edit error = %v, want errTaskChanged", err)
	}
}

func TestGeneralInsertedBeforeExistingBranches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := Add(path, "branch task", "", "", Section{Branch: "feature/login"}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "general task", "", PriorityNone, Section{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(data), "general task") > strings.Index(string(data), "# Branches") || strings.Contains(string(data), "General") {
		t.Fatalf("general section follows branches: %s", data)
	}
}

func TestExternalMarkdownIsReadWithoutChangingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	original := "- bare generic\n\n# Work\n\n- [ ] malformed priority !urgent\n\n  Keep this note.\n\n# Branches\n\n## feature/login\n\n- [ ] direct branch task\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 3 {
		t.Fatalf("tasks = %+v, error = %v", tasks, err)
	}
	if tasks[0].Text != "bare generic" || tasks[0].Done || tasks[0].Category != "" || tasks[1].Text != "malformed priority !urgent" || tasks[1].Priority != "" || tasks[1].Category != "Work" || tasks[1].Details != "Keep this note." || tasks[2].Branch != "feature/login" {
		t.Fatalf("misread external file: %+v", tasks)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("reading changed file: %q", got)
	}
	if err := Edit(path, tasks[0], "renamed generic", "", tasks[0].Section); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !strings.HasPrefix(string(got), "- [ ] renamed generic\n") || !strings.Contains(string(got), "- [ ] malformed priority !urgent\n") {
		t.Fatalf("edit failed to normalize bare item or changed unrelated content: %q", got)
	}
}

func TestAnyNonBranchHeadingDefinesCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- generic\n\n## General\n\n- under heading\n\n### deeper\n\n- nested heading\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 3 || tasks[0].Category != "" || tasks[1].Category != "General" || tasks[2].Category != "deeper" {
		t.Fatalf("heading categories = %+v, error = %v", tasks, err)
	}
}

func TestBranchNamedBranchesRemainsARegularBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := Add(path, "nested name", "", "", Section{Branch: "Branches"}); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 1 || tasks[0].Branch != "Branches" {
		t.Fatalf("branch name parsed incorrectly: %v, %+v", err, tasks)
	}
}

func TestBareItemsBecomeCheckboxesOnlyWhenEdited(t *testing.T) {
	for _, edit := range []struct {
		name string
		run  func(string, Task) error
	}{
		{"toggle", Toggle},
		{"priority", func(path string, item Task) error { return SetPriority(path, item, "high") }},
		{"category", func(path string, item Task) error { return SetCategory(path, item, "Work") }},
	} {
		t.Run(edit.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte("- bare\n\n- untouched\n"), 0644); err != nil {
				t.Fatal(err)
			}
			tasks, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); !strings.HasPrefix(string(got), "- bare\n") {
				t.Fatalf("read normalized a bare item: %s", got)
			}
			if err := edit.run(path, tasks[0]); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), "- untouched") || !strings.Contains(string(got), "- [") || strings.Contains(string(got), "\n- bare\n") {
				t.Fatalf("bare item edit damaged file: %s", got)
			}
		})
	}
}

func TestTasksStaySortedByPriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	for _, section := range []Section{{}, {Category: "Work"}, {Branch: "feature/login"}} {
		for _, item := range []struct {
			title    string
			priority Priority
		}{{"none", ""}, {"low", "low"}, {"high", "high"}, {"medium", "medium"}} {
			if err := Add(path, item.title+section.Category+section.Branch, "", item.priority, section); err != nil {
				t.Fatal(err)
			}
		}
	}
	tasks, err := Load(path)
	if err != nil || len(tasks) != 12 {
		t.Fatalf("tasks = %+v, error = %v", tasks, err)
	}
	for i := 0; i < len(tasks); i += 4 {
		for j, want := range []Priority{PriorityHigh, PriorityMedium, PriorityLow, PriorityNone} {
			if tasks[i+j].Priority != want {
				t.Fatalf("scope %d priority %d = %q, want %q: %+v", i/4, j, tasks[i+j].Priority, want, tasks)
			}
		}
	}
	if err := SetPriority(path, tasks[3], "high"); err != nil {
		t.Fatal(err)
	}
	tasks, err = Load(path)
	if err != nil || indexOrder(tasks[:4]) != "high,none,medium,low" || tasks[1].Priority != "high" {
		t.Fatalf("raised task did not move up behind the other high task: %v, %+v", err, tasks)
	}
	if indexOrder(tasks[4:]) != "highWork,mediumWork,lowWork,noneWork,highfeature/login,mediumfeature/login,lowfeature/login,nonefeature/login" {
		t.Fatalf("a priority change reordered other sections: %+v", tasks)
	}
}

func TestChangedTaskIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] first\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [ ] edited elsewhere\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Toggle(path, tasks[0]); !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("toggleTask error = %v, want errTaskChanged", err)
	}
}

func TestEditMovesTaskBetweenSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	original := "- [ ] Stay\n\n# auth\n\n- [ ] Move me !high\n\n  Old details.\n\n# Branches\n\n## feature/a\n\n- [ ] Branch move\n\n## feature/b\n\n- [ ] Other branch\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	find := func(text string) Task {
		t.Helper()
		tasks, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			if task.Text == text {
				return task
			}
		}
		t.Fatalf("task %q not found", text)
		return Task{}
	}
	if err := Edit(path, find("Move me"), "Moved", "New details.", Section{Category: "@docs"}); err != nil {
		t.Fatal(err)
	}
	if moved := find("Moved"); moved.Category != "docs" || moved.Priority != PriorityHigh || moved.Details != "New details." {
		t.Fatalf("category move lost content: %+v", moved)
	}
	if err := Edit(path, find("Moved"), "Moved", "New details.", Section{}); err != nil {
		t.Fatal(err)
	}
	if moved := find("Moved"); moved.Category != "" || moved.Branch != "" {
		t.Fatalf("blank category did not move the task to General: %+v", moved)
	}
	if err := Edit(path, find("Branch move"), "Branch move", "", Section{Branch: "feature/b"}); err != nil {
		t.Fatal(err)
	}
	if moved := find("Branch move"); moved.Branch != "feature/b" {
		t.Fatalf("branch move = %+v", moved)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"# auth", "# docs", "## feature/a"} {
		if strings.Contains(string(data), gone+"\n") {
			t.Errorf("moving left an empty %q heading: %s", gone, data)
		}
	}
	if !strings.Contains(string(data), "- [ ] Stay") || !strings.Contains(string(data), "- [ ] Other branch") {
		t.Fatalf("moving changed other tasks: %s", data)
	}
	if err := Edit(path, find("Stay"), "Stay", "", Section{Category: "docs", Branch: "feature/b"}); err == nil {
		t.Fatal("a task was allowed both a category and a branch")
	}
}

func TestPriorityIsATrailingToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := Add(path, "Typed priority !high", "", PriorityNone, Section{}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "Flag priority", "", PriorityLow, Section{}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "Both agree !low", "", PriorityLow, Section{}); err != nil {
		t.Fatal(err)
	}
	if err := Add(path, "Conflict !high", "", PriorityLow, Section{}); err == nil {
		t.Fatal("conflicting priorities were accepted")
	}
	want := "- [ ] Typed priority !high\n\n- [ ] Flag priority !low\n\n- [ ] Both agree !low\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
	find := func(text string) Task {
		t.Helper()
		tasks, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			if task.Text == text {
				return task
			}
		}
		t.Fatalf("task %q not found", text)
		return Task{}
	}
	if err := Edit(path, find("Typed priority"), "Typed priority !medium", "", Section{}); err != nil {
		t.Fatal(err)
	}
	if task := find("Typed priority"); task.Priority != PriorityMedium {
		t.Fatalf("typing !medium in an edit = %+v", task)
	}
	if err := SetPriority(path, find("Flag priority"), PriorityNone); err != nil {
		t.Fatal(err)
	}
	if err := Toggle(path, find("Both agree")); err != nil {
		t.Fatal(err)
	}
	want = "- [ ] Typed priority !medium\n\n- [x] Both agree !low\n\n- [ ] Flag priority\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestPriorityTokenKeepsWindowsLineEndings(t *testing.T) {
	path, tasks := writeAndLoad(t, "- [ ] Task !high\r\n- [ ] Other\r\n")
	if tasks[0].Text != "Task" || tasks[0].Priority != PriorityHigh {
		t.Fatalf("task = %+v", tasks[0])
	}
	if err := SetPriority(path, tasks[0], PriorityLow); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "- [ ] Task !low\r\n- [ ] Other\r\n" {
		t.Fatalf("file = %q", got)
	}
}

// TestMixedLineEndingsAreWrittenAsTheFirstLinesEnding checks that a file whose
// lines end differently is read without the "\r" of those that end in "\r\n",
// and written with every line ending as its first does.
func TestMixedLineEndingsAreWrittenAsTheFirstLinesEnding(t *testing.T) {
	for _, item := range []struct{ name, content, want string }{
		{"Windows first", "- [ ] One\r\n- [ ] Two\n- [ ] Three\r\n", "- [x] One\r\n- [ ] Two\r\n- [ ] Three\r\n"},
		{"Unix first", "- [ ] One\n- [ ] Two\r\n- [ ] Three\n", "- [x] One\n- [ ] Two\n- [ ] Three\n"},
	} {
		t.Run(item.name, func(t *testing.T) {
			path, tasks := writeAndLoad(t, item.content)
			if got := indexOrder(tasks); got != "One,Two,Three" {
				t.Fatalf("tasks = %q", got)
			}
			if err := Toggle(path, tasks[0]); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, path); got != item.want {
				t.Fatalf("file = %q, want %q", got, item.want)
			}
		})
	}
}

// TestByteOrderMarkIsSetAside makes each write to a file with a byte order
// mark and to the same file without one. The mark mustn't change what is
// read, and stays at the start of the file.
func TestByteOrderMarkIsSetAside(t *testing.T) {
	writes := []struct {
		name  string
		write func(path string, tasks []Task) error
	}{
		{"add to the first heading", func(path string, _ []Task) error {
			return Add(path, "New", "", PriorityNone, Section{Category: "work"})
		}},
		{"toggle", func(path string, tasks []Task) error { return Toggle(path, tasks[0]) }},
		{"edit", func(path string, tasks []Task) error {
			return Edit(path, tasks[0], "Renamed !low", "Details", Section{Category: "work"})
		}},
		{"set priority", func(path string, tasks []Task) error { return SetPriority(path, tasks[1], PriorityHigh) }},
		{"set category", func(path string, tasks []Task) error { return SetCategory(path, tasks[0], "other") }},
		{"clear done", func(path string, tasks []Task) error {
			r, err := PlanRemove(path, doneTasks(tasks))
			if err != nil {
				return err
			}
			return r.Apply()
		}},
		{"clear done and undo", func(path string, tasks []Task) error {
			r, err := PlanRemove(path, doneTasks(tasks))
			if err != nil {
				return err
			}
			if err := r.Apply(); err != nil {
				return err
			}
			return r.Undo()
		}},
	}
	for _, content := range []string{"# work\n\n- [ ] First\n- [x] Done\n", "- [ ] First\n- [x] Done\n", "# work\r\n\r\n- [ ] First\r\n- [x] Done\r\n"} {
		for _, w := range writes {
			t.Run(w.name, func(t *testing.T) {
				plainPath, plainTasks := writeAndLoad(t, content)
				bomPath, bomTasks := writeAndLoad(t, byteOrderMark+content)
				if !reflect.DeepEqual(bomTasks, plainTasks) {
					t.Fatalf("with a byte order mark, read\n%+v\nwant\n%+v", bomTasks, plainTasks)
				}
				if err := w.write(plainPath, plainTasks); err != nil {
					t.Fatal(err)
				}
				if err := w.write(bomPath, bomTasks); err != nil {
					t.Fatal(err)
				}
				if got, want := readFile(t, bomPath), byteOrderMark+readFile(t, plainPath); got != want {
					t.Fatalf("file = %q, want %q", got, want)
				}
			})
		}
	}
}

// TestParseMatchesLoad checks that parsing a file's bytes gives the tasks
// loading the file does, for the task file and a README, with a byte order
// mark and Windows line endings to set aside.
func TestParseMatchesLoad(t *testing.T) {
	content := byteOrderMark + "## TODO\r\n\r\n- [ ] First !high\r\n  - [x] Second\r\n"
	path, loaded := writeAndLoad(t, content)
	if len(loaded) != 2 {
		t.Fatalf("loaded %+v", loaded)
	}
	if parsed := Parse([]byte(content)); !reflect.DeepEqual(parsed, loaded) {
		t.Errorf("Parse = %+v, want %+v", parsed, loaded)
	}
	loadedReadme, err := LoadReadme(path)
	if err != nil || len(loadedReadme) != 2 {
		t.Fatalf("loaded README %+v, %v", loadedReadme, err)
	}
	if parsed := ParseReadme([]byte(content)); !reflect.DeepEqual(parsed, loadedReadme) {
		t.Errorf("ParseReadme = %+v, want %+v", parsed, loadedReadme)
	}
	if Parse(nil) != nil || ParseReadme(nil) != nil {
		t.Errorf("no contents have tasks, got %+v and %+v", Parse(nil), ParseReadme(nil))
	}
}

func indexOrder(tasks []Task) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = t.Text
	}
	return strings.Join(names, ",")
}
