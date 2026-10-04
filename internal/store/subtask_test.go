package store

import (
	"errors"
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
	want := strings.Replace(subtaskSample, "  - [ ] Write the form !low", "  - [x] Write the form !low", 1)
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

func TestRemovingSubtasks(t *testing.T) {
	path, tasks := writeAndLoad(t, subtaskSample)
	removal, err := PlanRemove(path, []Task{tasks[2]})
	if err != nil {
		t.Fatal(err)
	}
	if err := removal.Apply(); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), strings.Replace(subtaskSample, "  - [x] Add the route\n\n", "", 1); got != want {
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
	want := strings.Replace(strings.Replace(subtaskSample, "  - [x] Add the route\n\n", "", 1), "- [x] Old task\n  - [ ] Left open\n\n", "", 1)
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
