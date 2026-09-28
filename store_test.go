package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
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
	if err := addTask(path, "third"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Test TODOs\n\n- [x] first\nSome context.\n- [x] second\n\n## General\n\n- [ ] third\n"
	if string(got) != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestBranchesAndMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := addTask(path, "general task"); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "branch task", "high", []string{"tests"}, "feature/login"); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "another branch task", "", nil, "feature/login"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "### feature/login") != 1 {
		t.Fatalf("duplicate branch heading: %s", data)
	}
	if !strings.Contains(string(data), "## General\n") || !strings.Contains(string(data), "## Branches\n\n### feature/login\n") {
		t.Fatalf("missing headings: %s", data)
	}
	tasks, err := loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 || tasks[2].branch != "feature/login" || tasks[2].priority != "high" || taskLabel(tasks[2]) != "tests" || !tasks[2].headingLabel {
		t.Fatalf("parsed tasks: %+v", tasks)
	}
	if !strings.Contains(string(data), "#### @tests") || strings.Contains(string(data), "- Labels:") {
		t.Fatalf("label was not stored as a heading: %s", data)
	}
	if err := setTaskPriority(path, tasks[2], "low"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := setTaskLabel(path, tasks[2], "smoke"); err != nil {
		t.Fatal(err)
	}
	tasks, err = loadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if tasks[2].priority != "low" || taskLabel(tasks[2]) != "smoke" || tasks[1].text != "another branch task" {
		t.Fatalf("updated tasks: %+v", tasks)
	}
}

func TestLabelPrefixesAreNormalized(t *testing.T) {
	got := parseLabels("@auth, #tests, backend, @AUTH")
	if strings.Join(got, ",") != "auth,tests,backend" {
		t.Fatalf("labels = %v", got)
	}
}

func TestLabelsRequireOneAlphanumericWord(t *testing.T) {
	for _, label := range []string{"two words", "bug-fix", "under_score", "a?", "@", "@@auth", "emoji🙂"} {
		path := filepath.Join(t.TempDir(), "TODO.md")
		if err := addTaskWithOptions(path, "Task", "", []string{label}, ""); err == nil || !strings.Contains(err.Error(), "one word") {
			t.Errorf("label %q was accepted or gave an unclear error: %v", label, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("invalid label %q changed the file: %v", label, err)
		}
	}
	for _, label := range []string{"auth", "A1", "@tests2", "#café3"} {
		path := filepath.Join(t.TempDir(), "TODO.md")
		if err := addTaskWithOptions(path, "Task", "", []string{label}, ""); err != nil {
			t.Fatalf("valid label %q rejected: %v", label, err)
		}
		tasks, err := loadTasks(path)
		if err != nil || len(tasks) != 1 || taskLabel(tasks[0]) != normalizeLabelInput(label) {
			t.Fatalf("label %q was not saved correctly: %v, %+v", label, err, tasks)
		}
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
	path := filepath.Join(t.TempDir(), "TODO.md")
	if err := addTaskWithDetails(path, "Auth task", "Keep this detail.", "high", []string{"auth"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "General task", "", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := addTaskWithOptions(path, "Branch auth", "", []string{"auth"}, "feature/login"); err != nil {
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
	for _, want := range []string{"### @auth", "#### @auth", "  - Priority: High", "  Keep this detail."} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
	if strings.Index(s, "General task") > strings.Index(s, "### @auth") || strings.Index(s, "Branch plain") > strings.Index(s, "#### @auth") {
		t.Fatalf("unlabeled tasks were inserted under label headings: %s", s)
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
	if strings.Contains(string(data), "\n### @auth\n") || !strings.Contains(string(data), "\n### @docs\n") {
		t.Fatalf("relabel left an empty heading or missed the new heading: %s", data)
	}
	if err := addTaskWithOptions(path, "Too many", "", []string{"one", "two"}, ""); err == nil {
		t.Fatal("multiple labels were accepted")
	}
}

func TestLegacyLabelMovesToHeading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	old := "## General\n\n- [ ] Legacy\n  - Priority: High\n  - Labels: auth, tests\n\n  Keep this detail.\n"
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
	if !strings.Contains(string(data), "### @backend") || strings.Contains(string(data), "- Labels:") {
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
	if err != nil || len(tasks) != 3 || tasks[0].details != wantDetails {
		t.Fatalf("reloaded details: %v, %+v", err, tasks)
	}
	if err := toggleTask(path, tasks[0]); err != nil {
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
	if err := addTask(path, "general task"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(data), "## General") > strings.Index(string(data), "## Branches") {
		t.Fatalf("general section follows branches: %s", data)
	}
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
