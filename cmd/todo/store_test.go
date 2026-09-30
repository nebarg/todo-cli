package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	original := "# Test TODOs\n\n- [ ] first\nSome context.\n- [x] second\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("loadTasks: %v, %d tasks", err, len(tasks))
	}
	if err := toggleTask(path, tasks[0]); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "third", priorityNone, nil, ""); err != nil {
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
	if err := addTaskWithOptions(path, "general task", priorityNone, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "branch task", "high", nil, "feature/login"); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "another branch task", "", nil, "feature/login"); err != nil {
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
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 || tasks[1].branch != "feature/login" || tasks[1].priority != "high" || taskLabel(tasks[1]) != "" {
		t.Fatalf("parsed tasks: %+v", tasks)
	}
	if err := addTaskWithOptions(path, "invalid", "", []string{"tests"}, "feature/login"); err == nil {
		t.Fatal("branch category was accepted")
	}
	if err := setTaskPriority(path, tasks[1], "low"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range tasks {
		if item.text == "branch task" && item.priority != "low" {
			t.Fatalf("priority edit failed: %+v", tasks)
		}
		if item.text == "branch task" && setTaskLabel(path, item, "smoke") == nil {
			t.Fatal("branch category edit was accepted")
		}
	}
	if tasks[1].text != "branch task" || tasks[2].text != "another branch task" {
		t.Fatalf("updated tasks: %+v", tasks)
	}
}

func TestLabelPrefixesAreNormalized(t *testing.T) {
	got := parseLabels("@auth, #tests, backend, @AUTH")
	if strings.Join(got, ",") != "auth,tests,backend" {
		t.Fatalf("labels = %v", got)
	}
}

func TestCategoriesAllowSymbolsButNotWhitespace(t *testing.T) {
	for _, label := range []string{"two words", " leading", "trailing ", "tab\tname", "line\nbreak", ""} {
		path := filepath.Join(t.TempDir(), "TODO.md")
		if err := addTaskWithOptions(path, "Task", "", []string{label}, ""); err == nil || !strings.Contains(err.Error(), "category must be a single word without whitespace") {
			t.Errorf("category %q was accepted or gave an unclear error: %v", label, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("invalid category %q changed the file: %v", label, err)
		}
	}
	for _, label := range []string{"auth", "A1", "@tests2", "#café3", "+v1", "bug-fix", "under_score", "a?", "emoji🙂", "foo#", "@", "#"} {
		path := filepath.Join(t.TempDir(), "TODO.md")
		if err := addTaskWithOptions(path, "Task", "", []string{label}, ""); err != nil {
			t.Fatalf("valid category %q rejected: %v", label, err)
		}
		tasks, err := loadTasks(path)
		if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != normalizeLabelInput(label) {
			t.Fatalf("category %q was not saved correctly: %v, %+v", label, err, tasks)
		}
	}
	if _, name, ok := parseHeading("# auth ###"); !ok || name != "auth" {
		t.Fatalf("closing Markdown hashes were not parsed correctly: %q", name)
	}
	if err := validateLabel("Branches"); err == nil {
		t.Fatal("reserved branch heading was accepted as a category")
	}
}

func TestChangingCategoryToSymbolName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := addTaskWithOptions(path, "Task", "", []string{"auth"}, ""); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("initial category: %v, %+v", err, tasks)
	}
	if err := setTaskLabel(path, tasks[0], "+v1"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != "+v1" {
		t.Fatalf("symbol category was not saved: %v, %+v", err, tasks)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "# +v1\n") || strings.Contains(string(data), "# auth\n") {
		t.Fatalf("category heading was not updated: %v, %q", err, data)
	}
}

func TestLegacySpacedLabelCanBeRenamed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	old := "## General\n\n### @a label?\n\n- [ ] Existing task\n"
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != "a label?" {
		t.Fatalf("legacy label was not readable: %v, %+v", err, tasks)
	}
	if err := setTaskLabel(path, tasks[0], "bad label"); err == nil {
		t.Fatal("invalid edit was accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != old {
		t.Fatalf("invalid edit changed the file: %v, %q", err, data)
	}
	if err := setTaskLabel(path, tasks[0], "clean2"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != "clean2" {
		t.Fatalf("legacy label could not be renamed: %v, %+v", err, tasks)
	}
}

func TestSingleLabelHeadingsAndMoves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := addTaskWithDetails(path, "Auth task", "Keep this detail.", "high", []string{"auth"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "General task", "", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "Branch plain", "", nil, "feature/login"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{"# auth", "# Branches", "## feature/login", "  - Priority: High", "  Keep this detail."} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
	if strings.Index(s, "General task") > strings.Index(s, "# auth") || strings.Index(s, "Branch plain") < strings.Index(s, "## feature/login") {
		t.Fatalf("tasks were inserted under incorrect headings: %s", s)
	}
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	var selected task
	for _, item := range tasks {
		if item.text == "Auth task" {
			selected = item
		}
	}
	if err := setTaskLabel(path, selected, "docs"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range tasks {
		if item.text == "Auth task" {
			found = taskLabel(item) == "docs" && item.priority == "high" && item.details == "Keep this detail."
		}
	}
	if !found {
		t.Fatalf("relabel changed task content: %+v", tasks)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\n# auth\n") || !strings.Contains(string(data), "\n# docs\n") {
		t.Fatalf("relabel left an empty heading or missed the new heading: %s", data)
	}
	if err := addTaskWithOptions(path, "Too many", "", []string{"one", "two"}, ""); err == nil || err.Error() != "a task can have only one category" {
		t.Fatalf("multiple categories returned %v", err)
	}
}

func TestLegacyLabelMovesToHeading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	old := "- [ ] Legacy\n  - Priority: High\n  - Labels: auth, tests\n\n  Keep this detail.\n"
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != "auth" {
		t.Fatalf("legacy label missing: %v, %+v", err, tasks)
	}
	if err := setTaskLabel(path, tasks[0], "backend"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# backend") || strings.Contains(string(data), "- Labels:") {
		t.Fatalf("legacy metadata was not migrated: %s", data)
	}
	tasks, err = loadTasks(path)
	if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != "backend" || tasks[0].priority != "high" || tasks[0].details != "Keep this detail." {
		t.Fatalf("migrated task changed: %v, %+v", err, tasks)
	}
}

func TestTaskDetailsAreParsedAndPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	original := "## General\n\n- [ ] Fix login redirect\n  - Priority: High\n  - Labels: auth, bug\n\n  When a session expires, return to the previous page.\n\n  - [ ] Add a regression test\n\n- [ ] Another task\n\n## Branches\n\n### feature/login\n\n- [ ] Branch task\n\n  Branch-specific context.\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3: %+v", len(tasks), tasks)
	}
	wantDetails := "When a session expires, return to the previous page.\n\n- [ ] Add a regression test"
	if tasks[0].details != wantDetails || tasks[2].details != "Branch-specific context." {
		t.Fatalf("parsed details: %+v", tasks)
	}
	if err := setTaskPriority(path, tasks[0], "low"); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "  - Labels: auth, bug\n\n  When a session expires, return to the previous page.\n\n  - [ ] Add a regression test") {
		t.Fatalf("details changed while editing metadata: %s", updated)
	}
	tasks, err = loadTasks(path)
	if err != nil || len(tasks) != 3 {
		t.Fatalf("reloaded details: %v, %+v", err, tasks)
	}
	var selected task
	for _, item := range tasks {
		if item.text == "Fix login redirect" {
			selected = item
		}
	}
	if selected.details != wantDetails {
		t.Fatalf("reloaded details: %+v", tasks)
	}
	if err := toggleTask(path, selected); err != nil {
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
	original := "## General\n\n- [ ] First task\n  - Priority: High\n  - Labels: auth\n\n  Old details.\n\n- [ ] Second task\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := editTaskContent(path, tasks[0], "Renamed task", "New context.\n\n- [ ] Nested step"); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- [ ] Renamed task\n  - Priority: High\n  - Labels: auth", "  New context.\n\n  - [ ] Nested step", "- [ ] Second task"} {
		if !strings.Contains(string(updated), want) {
			t.Fatalf("missing %q after edit: %s", want, updated)
		}
	}
	tasks, err = loadTasks(path)
	if err != nil || len(tasks) != 2 || tasks[0].priority != "high" || tasks[0].details != "New context.\n\n- [ ] Nested step" {
		t.Fatalf("reloaded tasks: %v, %+v", err, tasks)
	}
	if err := editTaskContent(path, tasks[0], "Renamed task", ""); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil || len(tasks) != 2 || tasks[0].details != "" {
		t.Fatalf("details were not removed: %v, %+v", err, tasks)
	}
}

func TestEditTaskContentRejectsChangedBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] First\n\n  Original details.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [ ] First\n\n  Edited elsewhere.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := editTaskContent(path, tasks[0], "New title", "My edit"); !errors.Is(err, errTaskChanged) {
		t.Fatalf("edit error = %v, want errTaskChanged", err)
	}
}

func TestGeneralInsertedBeforeExistingBranches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := addTaskWithOptions(path, "branch task", "", nil, "feature/login"); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "general task", priorityNone, nil, ""); err != nil {
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

func TestDefaultFileUsesLowercaseName(t *testing.T) {
	if got := defaultFile(projectContext{}); got != "todo.md" {
		t.Fatalf("default file = %q", got)
	}
	if got := defaultFile(projectContext{root: "/project"}); got != filepath.Join("/project", "todo.md") {
		t.Fatalf("project file = %q", got)
	}
}

func TestExternalMarkdownIsReadWithoutChangingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	original := "- bare generic\n\n# Work\n\n- [ ] malformed priority\n  - Priority: urgent\n\n  Keep this note.\n\n# Branches\n\n## feature/login\n\n- [ ] direct branch task\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 3 {
		t.Fatalf("tasks = %+v, error = %v", tasks, err)
	}
	if tasks[0].text != "bare generic" || tasks[0].done || taskLabel(tasks[0]) != "" || tasks[1].priority != "" || taskLabel(tasks[1]) != "Work" || tasks[1].details != "Keep this note." || tasks[2].branch != "feature/login" {
		t.Fatalf("misread external file: %+v", tasks)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("reading changed file: %q", got)
	}
	if err := editTaskContent(path, tasks[0], "renamed generic", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !strings.HasPrefix(string(got), "- [ ] renamed generic\n") || !strings.Contains(string(got), "  - Priority: urgent") {
		t.Fatalf("edit failed to normalize bare item or changed unrelated content: %q", got)
	}
}

func TestAnyNonBranchHeadingDefinesCategory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	content := "- generic\n\n## General\n\n- under heading\n\n### deeper\n\n- nested heading\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 3 || taskLabel(tasks[0]) != "" || taskLabel(tasks[1]) != "General" || taskLabel(tasks[2]) != "deeper" {
		t.Fatalf("heading categories = %+v, error = %v", tasks, err)
	}
}

func TestBranchNamedBranchesRemainsARegularBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	if err := addTaskWithOptions(path, "nested name", "", nil, "Branches"); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 1 || tasks[0].branch != "Branches" {
		t.Fatalf("branch name parsed incorrectly: %v, %+v", err, tasks)
	}
}

func TestBareItemsBecomeCheckboxesOnlyWhenEdited(t *testing.T) {
	for _, edit := range []struct {
		name string
		run  func(string, task) error
	}{
		{"toggle", toggleTask},
		{"priority", func(path string, item task) error { return setTaskPriority(path, item, "high") }},
		{"category", func(path string, item task) error { return setTaskLabel(path, item, "Work") }},
	} {
		t.Run(edit.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "todo.md")
			if err := os.WriteFile(path, []byte("- bare\n\n- untouched\n"), 0644); err != nil {
				t.Fatal(err)
			}
			tasks, err := loadTasks(path)
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

func TestNewTasksSortedPriorityAndEditsStayInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	for _, scope := range []struct{ label, branch string }{{}, {label: "Work"}, {branch: "feature/login"}} {
		for _, item := range []struct {
			title    string
			priority priority
		}{{"none", ""}, {"low", "low"}, {"high", "high"}, {"medium", "medium"}} {
			if err := addTaskWithOptions(path, item.title+scope.label+scope.branch, item.priority, labelSlice(scope.label), scope.branch); err != nil {
				t.Fatal(err)
			}
		}
	}
	tasks, err := loadTasks(path)
	if err != nil || len(tasks) != 12 {
		t.Fatalf("tasks = %+v, error = %v", tasks, err)
	}
	for i := 0; i < len(tasks); i += 4 {
		for j, want := range []priority{priorityHigh, priorityMedium, priorityLow, priorityNone} {
			if tasks[i+j].priority != want {
				t.Fatalf("scope %d priority %d = %q, want %q: %+v", i/4, j, tasks[i+j].priority, want, tasks)
			}
		}
	}
	if err := setTaskPriority(path, tasks[3], "high"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil || tasks[0].text != "high" || tasks[1].text != "medium" || tasks[2].text != "low" || tasks[3].text != "none" || tasks[3].priority != "high" {
		t.Fatalf("priority edit moved a task: %v, %+v", err, tasks)
	}
}

func labelSlice(label string) []string {
	if label == "" {
		return nil
	}
	return []string{label}
}

func TestChangedTaskIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := os.WriteFile(path, []byte("- [ ] first\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("- [ ] edited elsewhere\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := toggleTask(path, tasks[0]); !errors.Is(err, errTaskChanged) {
		t.Fatalf("toggleTask error = %v, want errTaskChanged", err)
	}
}

func TestScanSource(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // Force the built-in scanner.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("// @todo fix this\nconst name = \"todo scan\"\n// TODO: test it\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TODO.md"), []byte("- [ ] TODO: stored task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "dep.js"), []byte("// TODO: dependency\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.go"), []byte("// TODO: hidden\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "binary.dat"), []byte{'T', 'O', 'D', 'O', 0, 'x'}, 0644); err != nil {
		t.Fatal(err)
	}
	matches, err := scanSource(dir, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].path != "code.go" || matches[0].line != 1 || matches[1].line != 3 {
		t.Fatalf("matches: %+v", matches)
	}
	all, err := scanSource(dir, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("all-file matches: %+v", all)
	}
}

func TestParsePriority(t *testing.T) {
	for _, item := range []struct {
		input string
		want  priority
		err   bool
	}{
		{"", priorityNone, false},
		{" High ", priorityHigh, false},
		{"MEDIUM", priorityMedium, false},
		{"low", priorityLow, false},
		{"urgent", priorityNone, true},
	} {
		t.Run(item.input, func(t *testing.T) {
			got, err := parsePriority(item.input)
			if got != item.want || (err != nil) != item.err {
				t.Fatalf("parsePriority(%q) = %q, %v", item.input, got, err)
			}
		})
	}
}

func TestPriorityCyclesAndTitles(t *testing.T) {
	p := priorityNone
	var seen []string
	for range 4 {
		p = p.next()
		seen = append(seen, p.title())
	}
	if got := strings.Join(seen, ","); got != "High,Medium,Low," {
		t.Fatalf("priority cycle = %q", got)
	}
	if priority("urgent").next() != priorityNone || priority("urgent").rank() != priorityNone.rank() {
		t.Fatal("unknown priority should behave like no priority")
	}
}

func TestChangedTaskErrorLeavesUIHintsToCaller(t *testing.T) {
	if strings.Contains(errTaskChanged.Error(), "press") {
		t.Fatalf("store error mentions a UI key: %q", errTaskChanged)
	}
	if got := errorStatus(fmt.Errorf("save: %w", errTaskChanged)); !strings.Contains(got, "press r to reload") {
		t.Fatalf("TUI status lost the reload hint: %q", got)
	}
}

func TestSortedMatchesOrderBeforeLimit(t *testing.T) {
	matches := []sourceTodo{{path: "b.go", line: 1}, {path: "a.go", line: 9}, {path: "a.go", line: 2}, {path: "c.go", line: 1}}
	got := sortedMatches(matches, 3)
	if len(got) != 3 || got[0] != (sourceTodo{path: "a.go", line: 2}) || got[1].line != 9 || got[2].path != "b.go" {
		t.Fatalf("matches were limited before sorting: %+v", got)
	}
	if got := sortedMatches(nil, 3); len(got) != 0 {
		t.Fatalf("empty scan = %+v", got)
	}
}

func TestBuiltInScanLimitIsDeterministic(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // Force the built-in scanner.
	dir := t.TempDir()
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("// TODO: item\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		matches, err := scanSource(dir, 3, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 3 || matches[0].path != "f00.go" || matches[2].path != "f02.go" {
			t.Fatalf("limited scan picked arbitrary files: %+v", matches)
		}
	}
}

func TestGitOutputReportsGitError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	_, err := gitOutput(t.TempDir(), "rev-parse", "--show-toplevel")
	if err == nil || !strings.Contains(err.Error(), "git rev-parse:") || !strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
		t.Fatalf("git error lost its message: %v", err)
	}
}
