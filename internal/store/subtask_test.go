package store

import (
	"errors"
	"os"
	"strings"
	"testing"
)

const subtaskSample = "# auth\n\n" +
	"- [ ] Ship login !high\n\n" +
	"  Session notes.\n\n" +
	"  - [ ] Write the form !low\n" +
	"    Use the shared input.\r\n" +
	"  - [x] Add the route\n\n" +
	"  - plain note\n\n" +
	"- [x] Old task\n" +
	"  - [ ] Left open\n\n" +
	"# Branches\n\n## main\n\n" +
	"- [ ] Branch task\n" +
	"  - [ ] Branch subtask\n"

// subtaskSampleWritten is subtaskSample as the store writes it back: the
// "\r\n" ending of one of its lines becomes "\n", the ending of its first.
var subtaskSampleWritten = strings.ReplaceAll(subtaskSample, "\r\n", "\n")

func TestSubtasksAreTheCheckboxesUnderATask(t *testing.T) {
	_, tasks := writeAndLoad(t, subtaskSample)
	type want struct {
		line     int
		text     string
		subtask  bool
		done     bool
		priority Priority
		details  string
		section  Section
	}
	auth, main := Section{Category: "auth"}, Section{Branch: "main"}
	wants := []want{
		{2, "Ship login", false, false, PriorityHigh, "Session notes.\n\n- plain note", auth},
		{6, "Write the form", true, false, PriorityLow, "Use the shared input.", auth},
		{8, "Add the route", true, true, PriorityNone, "", auth},
		{12, "Old task", false, true, PriorityNone, "", auth},
		{13, "Left open", true, false, PriorityNone, "", auth},
		{19, "Branch task", false, false, PriorityNone, "", main},
		{20, "Branch subtask", true, false, PriorityNone, "", main},
	}
	if len(tasks) != len(wants) {
		t.Fatalf("got %d tasks, want %d: %+v", len(tasks), len(wants), tasks)
	}
	for i, w := range wants {
		got := tasks[i]
		if got.Line != w.line || got.Text != w.text || got.Subtask != w.subtask || got.Done != w.done || got.Priority != w.priority || got.Details != w.details || got.Section != w.section {
			t.Errorf("task %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestSubtaskWritesChangeOnlyTheirLines(t *testing.T) {
	path, tasks := writeAndLoad(t, subtaskSample)
	if err := Toggle(path, tasks[1]); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(subtaskSampleWritten, "  - [ ] Write the form !low", "  - [x] Write the form !low", 1)
	if got := readFile(t, path); got != want {
		t.Fatalf("after toggling a subtask:\n%q\nwant\n%q", got, want)
	}
	tasks, _ = Load(path)
	if err := SetPriority(path, tasks[2], PriorityMedium); err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(want, "  - [x] Add the route", "  - [x] Add the route !medium", 1)
	if got := readFile(t, path); got != want {
		t.Fatalf("after a subtask's priority:\n%q\nwant\n%q", got, want)
	}
	tasks, _ = Load(path)
	if err := Edit(path, tasks[2], "Add the routes", tasks[2].Details, tasks[2].Section); err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(want, "  - [x] Add the route !medium", "  - [x] Add the routes !medium", 1)
	if got := readFile(t, path); got != want {
		t.Fatalf("after editing a subtask:\n%q\nwant\n%q", got, want)
	}
	tasks, _ = Load(path)
	for _, err := range []error{
		SetCategory(path, tasks[1], "docs"),
		Edit(path, tasks[1], "Write the form", "", Section{Category: "docs"}),
	} {
		if !errors.Is(err, errSubtaskSection) {
			t.Fatalf("moving a subtask = %v, want errSubtaskSection", err)
		}
	}
}

func TestTasksTakeTheirSubtasksAlong(t *testing.T) {
	path, tasks := writeAndLoad(t, "- [ ] First\n  - [ ] Its step\n\n- [ ] Second !high\n")
	// Sorting the section after a write moves the task's subtasks with it.
	if err := Toggle(path, tasks[0]); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), "- [ ] Second !high\n\n- [x] First\n  - [ ] Its step\n"; got != want {
		t.Fatalf("after completing the task:\n%q\nwant\n%q", got, want)
	}
	tasks, _ = Load(path)
	if err := SetCategory(path, tasks[1], "docs"); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), "- [ ] Second !high\n\n# docs\n\n- [x] First\n  - [ ] Its step\n"; got != want {
		t.Fatalf("after moving the task:\n%q\nwant\n%q", got, want)
	}
	tasks, _ = Load(path)
	if err := Edit(path, tasks[1], "First", "New notes.", Section{}); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), "- [ ] Second !high\n\n- [x] First\n\n  New notes.\n\n  - [ ] Its step\n"; got != want {
		t.Fatalf("after moving the task back with new details:\n%q\nwant\n%q", got, want)
	}
}

func TestReopeningATaskPutsItBackWhereItWas(t *testing.T) {
	const content = "- [ ] A\n- [ ] B\n  - [ ] B1\n  - [x] B2\n- [ ] C\n"
	path, tasks := writeAndLoad(t, content)
	if err := Toggle(path, tasks[1]); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), strings.Replace(content, "- [ ] B", "- [x] B", 1); got != want {
		t.Fatalf("after done:\n%q\nwant\n%q", got, want)
	}
	tasks, _ = Load(path)
	if err := Toggle(path, tasks[1]); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != content {
		t.Fatalf("after reopening:\n%q\nwant\n%q", got, content)
	}
}

func TestRemovingSubtasks(t *testing.T) {
	path, tasks := writeAndLoad(t, subtaskSample)
	removal, err := PlanRemove(path, []Task{tasks[2]})
	if err != nil {
		t.Fatal(err)
	}
	if err := removal.Apply(); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), strings.Replace(subtaskSampleWritten, "  - [x] Add the route\n\n", "", 1); got != want {
		t.Fatalf("after removing a subtask:\n%q\nwant\n%q", got, want)
	}

	// A task goes with its subtasks, which aren't removed a second time.
	path, tasks = writeAndLoad(t, subtaskSample)
	removal, err = PlanRemove(path, doneTasks(tasks))
	if err != nil {
		t.Fatal(err)
	}
	if len(removal.Tasks) != 2 || removal.Tasks[0].Text != "Old task" || removal.Tasks[1].Text != "Add the route" {
		t.Fatalf("removal tasks = %+v", removal.Tasks)
	}
	if err := removal.Apply(); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(strings.Replace(subtaskSampleWritten, "  - [x] Add the route\n\n", "", 1), "- [x] Old task\n  - [ ] Left open\n\n", "", 1)
	if got := readFile(t, path); got != want {
		t.Fatalf("after removing the done tasks:\n%q\nwant\n%q", got, want)
	}
}

func TestRemovingATaskWithItsSubtask(t *testing.T) {
	path, tasks := writeAndLoad(t, subtaskSample)
	removal, err := PlanRemove(path, tasks[:2])
	if err != nil {
		t.Fatal(err)
	}
	if len(removal.Tasks) != 1 || removal.Tasks[0].Text != "Ship login" {
		t.Fatalf("removal tasks = %+v", removal.Tasks)
	}
	if err := removal.Apply(); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), strings.Replace(subtaskSample, "- [ ] Ship login !high\n\n  Session notes.\n\n  - [ ] Write the form !low\n    Use the shared input.\r\n  - [x] Add the route\n\n  - plain note\n\n", "", 1); got != want {
		t.Fatalf("after removing the task and its subtask:\n%q\nwant\n%q", got, want)
	}
}

func TestAddSubtask(t *testing.T) {
	for _, item := range []struct{ name, content, title, want string }{
		{"after the last subtask", "- [ ] Task\n  - [ ] First\n\n- [ ] Next\n", "Second !high", "- [ ] Task\n  - [ ] First\n  - [ ] Second !high\n\n- [ ] Next\n"},
		{"after the details", "- [ ] Task\n\n  Notes.\n\n- [ ] Next\n", "First", "- [ ] Task\n\n  Notes.\n\n  - [ ] First\n\n- [ ] Next\n"},
		{"under a bare task", "# auth\n\n- Task", "First", "# auth\n\n- Task\n  - [ ] First"},
		{"in a Windows file", "- [ ] Task\r\n\r\n  Notes.\r\n", "First", "- [ ] Task\r\n\r\n  Notes.\r\n\r\n  - [ ] First\r\n"},
	} {
		t.Run(item.name, func(t *testing.T) {
			path, tasks := writeAndLoad(t, item.content)
			if err := AddSubtask(path, tasks[0], item.title); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, path); got != item.want {
				t.Fatalf("file = %q, want %q", got, item.want)
			}
		})
	}
	path, tasks := writeAndLoad(t, "- [ ] Task\n  - [ ] First\n")
	for _, err := range []error{AddSubtask(path, tasks[1], "Nested"), AddSubtask(path, tasks[0], " ")} {
		if err == nil {
			t.Fatal("a subtask of a subtask, or a blank one, was added")
		}
	}
	if err := os.WriteFile(path, []byte("- [ ] Task, changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AddSubtask(path, tasks[0], "Late"); !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("adding under a changed task = %v, want ErrTaskChanged", err)
	}
}

func TestClearTargetsTakeSubtasksWithTheirTask(t *testing.T) {
	_, tasks := writeAndLoad(t, "- [x] Old task\n  - [ ] Left open\n  - [x] Done under done\n- [ ] Ship login\n  - [x] Add the route\n  - [ ] Write the form\n\n"+
		"# Branches\n\n## feature/gone\n\n- [ ] Gone task\n  - [x] Gone sub\n")
	for _, item := range []struct {
		name                                       string
		missing                                    func(string) bool
		texts                                      string
		done, doneSubtasks, subtasks, openSubtasks int
		branches                                   string
	}{
		{"done", func(string) bool { return false }, "Old task,Add the route,Gone sub", 3, 2, 2, 1, ""},
		{"done and missing", func(branch string) bool { return branch == "feature/gone" }, "Old task,Add the route,Gone task", 2, 1, 3, 1, "feature/gone"},
	} {
		t.Run(item.name, func(t *testing.T) {
			got := PickClearTargets(tasks, item.missing)
			var texts []string
			for _, task := range got.Tasks {
				texts = append(texts, task.Text)
			}
			if strings.Join(texts, ",") != item.texts || got.Done != item.done || got.DoneSubtasks != item.doneSubtasks ||
				got.Subtasks != item.subtasks || got.OpenSubtasks != item.openSubtasks || strings.Join(got.Branches, ",") != item.branches {
				t.Fatalf("tasks %q, done %d (%d subtasks), %d subtasks (%d open), branches %q; want %q, %d (%d), %d (%d), %q",
					texts, got.Done, got.DoneSubtasks, got.Subtasks, got.OpenSubtasks, got.Branches,
					item.texts, item.done, item.doneSubtasks, item.subtasks, item.openSubtasks, item.branches)
			}
		})
	}
}
