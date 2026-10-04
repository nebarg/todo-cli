package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// readmeSample follows todo-system's samples/README.md, with extra cases.
const readmeSample = "# Project\n\n" +
	"- not a todo\n\n" +
	"## TODOs:\n\n" +
	"- abc\n" +
	"    - todo0 def\n" +
	"    - todo00: ghi\n" +
	"- [ ] bar\n" +
	"    - [x] baz\n" +
	"- todo12 stays generic\n" +
	"- Ship todo1 soon\n" +
	"-no space\n" +
	"* star item\n" +
	"  ```\n" +
	"  # indented comment\n" +
	"  ```\n" +
	"- after indented code\n" +
	"```\n" +
	"- in a code block\n" +
	"# not a heading\n" +
	"```\n" +
	"- after code\n" +
	"#Not a heading\n" +
	"- still todos\n\n" +
	"## Another section\n\n" +
	"- abc\n\n" +
	"### todo\n\n" +
	"- [X] lower heading\r\n"

func TestLoadReadmeFindsListItemsUnderTodoHeadings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(readmeSample), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		line  int
		text  string
		level string
		done  bool
	}
	wants := []want{
		{6, "abc", "", false},
		{7, "def", "0", false},
		{8, "ghi", "00", false},
		{9, "bar", "", false},
		{10, "baz", "", true},
		{11, "todo12 stays generic", "", false},
		{12, "Ship soon", "1", false},
		{18, "after indented code", "", false},
		{23, "after code", "", false},
		{25, "still todos", "", false},
		{33, "lower heading", "", true},
	}
	if len(tasks) != len(wants) {
		t.Fatalf("got %d tasks, want %d: %+v", len(tasks), len(wants), tasks)
	}
	for i, w := range wants {
		got := tasks[i]
		if got.Line != w.line || got.Text != w.text || got.Level != w.level || got.Done != w.done {
			t.Errorf("task %d = line %d %q level %q done %v, want %+v", i, got.Line, got.Text, got.Level, got.Done, w)
		}
	}
}

func TestLoadReadmeKeepsHeadingsInsideATodoSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	readme := "# Todo\n\n" +
		"- loose\n\n" +
		"## Backend\n\n" +
		"- [ ] api\n" +
		"### Database ##\n\n" +
		"- [x] migrate\n" +
		"## Frontend\n" +
		"- todo0 styles\n" +
		"# Install\n\n" +
		"- not a task\n" +
		"## TODOs\n" +
		"- second section\n" +
		"### Todo\n" +
		"- under a nested todo heading\n" +
		"## Usage\n" +
		"- not a task either\n"
	if err := os.WriteFile(path, []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		line    int
		text    string
		heading string
		level   string
		done    bool
	}
	wants := []want{
		{2, "loose", "", "", false},
		{6, "api", "Backend", "", false},
		{9, "migrate", "Database", "", true},
		{11, "styles", "Frontend", "0", false},
		{16, "second section", "", "", false},
		{18, "under a nested todo heading", "Todo", "", false},
	}
	if len(tasks) != len(wants) {
		t.Fatalf("got %d tasks, want %d: %+v", len(tasks), len(wants), tasks)
	}
	for i, w := range wants {
		got := tasks[i]
		if got.Line != w.line || got.Text != w.text || got.Heading != w.heading || got.Level != w.level || got.Done != w.done {
			t.Errorf("task %d = line %d %q heading %q level %q done %v, want %+v", i, got.Line, got.Text, got.Heading, got.Level, got.Done, w)
		}
	}
}

func TestLoadReadmeReadsPlainItemsUnderACheckboxTaskAsDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	readme := "## TODOs\n\n" +
		"- [ ] **Slug stability.** Renaming a workspace regenerates its slug.\n" +
		"    Decide between:\n" +
		"    - a slug fixed at creation, or\n" +
		"      wrapped onto a second line\n" +
		"    - editable slugs with a history table\n" +
		"        - [ ] Draft the migration\n" +
		"    - consider a rename limit\n\n" +
		"- Plain parent\n" +
		"    - plain child\n" +
		"        - grandchild\n" +
		"- [x] Done with notes\r\n" +
		"  - a note\r\n" +
		"Unindented text\n" +
		"  - after the text\n"
	if err := os.WriteFile(path, []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		line    int
		text    string
		details string
		subtask bool
	}
	wants := []want{
		{2, "**Slug stability.** Renaming a workspace regenerates its slug.", "Decide between:\n- a slug fixed at creation, or\n  wrapped onto a second line\n- editable slugs with a history table\n- consider a rename limit", false},
		{7, "Draft the migration", "", true},
		// todo-system reads items nested under a plain item as to-dos.
		{10, "Plain parent", "", false},
		{11, "plain child", "", true},
		{12, "grandchild", "", true},
		{13, "Done with notes", "- a note", false},
		{16, "after the text", "", false},
	}
	if len(tasks) != len(wants) {
		t.Fatalf("got %d tasks, want %d: %+v", len(tasks), len(wants), tasks)
	}
	for i, w := range wants {
		if got := tasks[i]; got.Line != w.line || got.Text != w.text || got.Details != w.details || got.Subtask != w.subtask {
			t.Errorf("task %d = line %d %q details %q subtask %v, want %+v", i, got.Line, got.Text, got.Details, got.Subtask, w)
		}
	}

	if err := ToggleReadme(path, tasks[0]); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), strings.Replace(readme, "- [ ] **Slug", "- [x] **Slug", 1); got != want {
		t.Fatalf("toggle changed more than the checkbox:\n%q\nwant\n%q", got, want)
	}
}

func TestLoadReadmeWithoutFileOrTodoSection(t *testing.T) {
	dir := t.TempDir()
	if tasks, err := LoadReadme(filepath.Join(dir, "README.md")); err != nil || tasks != nil {
		t.Fatalf("missing README = %v, %v", tasks, err)
	}
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("# Project\n\n- feature\n\n## Todo list\n\n- not a todo section\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if tasks, err := LoadReadme(path); err != nil || len(tasks) != 0 {
		t.Fatalf("README without a TODO heading = %+v, %v", tasks, err)
	}
}

func TestToggleReadmeOnlyChangesTheCheckbox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	original := "# Todo\n\n- todo0 plain item !high\n  - [ ] nested\n- [x] done item\r\n\n## Next\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		task int
		want string
	}{
		{0, "# Todo\n\n- [x] todo0 plain item !high\n  - [ ] nested\n- [x] done item\r\n\n## Next\n"},
		{0, "# Todo\n\n- [ ] todo0 plain item !high\n  - [ ] nested\n- [x] done item\r\n\n## Next\n"},
		{1, "# Todo\n\n- [ ] todo0 plain item !high\n  - [x] nested\n- [x] done item\r\n\n## Next\n"},
		{2, "# Todo\n\n- [ ] todo0 plain item !high\n  - [x] nested\n- [ ] done item\r\n\n## Next\n"},
	}
	for _, step := range steps {
		tasks, err := LoadReadme(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := ToggleReadme(path, tasks[step.task]); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != step.want {
			t.Fatalf("after toggling task %d:\n%q\nwant\n%q", step.task, got, step.want)
		}
	}
}

func TestReadmeByteOrderMarkDoesNotHideTheHeading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(byteOrderMark+"## TODO\n\n- Write docs\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := LoadReadme(path)
	if err != nil || len(tasks) != 1 || tasks[0].Text != "Write docs" {
		t.Fatalf("tasks = %+v, %v", tasks, err)
	}
	if err := ToggleReadme(path, tasks[0]); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), byteOrderMark+"## TODO\n\n- [x] Write docs\n"; got != want {
		t.Fatalf("README = %q, want %q", got, want)
	}
}

func TestToggleReadmeRefusesAChangedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte("## TODO\n\n- first\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("## TODO\n\n- first, reworded\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ToggleReadme(path, tasks[0]); !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("toggle of a changed line = %v, want ErrTaskChanged", err)
	}
}

func TestReadmeTaskRoundTripsThroughToggle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	original := "# Project\n\n## TODO\n\n- todo0 Fix it\n- [x] Shipped\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	read, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []ReadmeTask{
		{Line: 4, Text: "Fix it", Level: "0", raw: "- todo0 Fix it"},
		{Line: 5, Text: "Shipped", Done: true, raw: "- [x] Shipped"},
	}
	if !reflect.DeepEqual(read, want) {
		t.Fatalf("read %+v, want %+v", read, want)
	}

	if err := ToggleReadme(path, read[0]); err != nil {
		t.Fatal(err)
	}
	done, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	want[0].Done, want[0].raw = true, "- [x] todo0 Fix it"
	if !reflect.DeepEqual(done, want) {
		t.Fatalf("after done %+v, want %+v", done, want)
	}
	if err := ToggleReadme(path, read[0]); !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("toggle of a task read before its line changed = %v, want ErrTaskChanged", err)
	}

	if err := ToggleReadme(path, done[0]); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadReadme(path)
	if err != nil {
		t.Fatal(err)
	}
	// Reopening leaves the checkbox that marking it done added.
	want[0].Done, want[0].raw = false, "- [ ] todo0 Fix it"
	if !reflect.DeepEqual(reopened, want) {
		t.Fatalf("after reopening %+v, want %+v", reopened, want)
	}
	if got := readFile(t, path); got != "# Project\n\n## TODO\n\n- [ ] todo0 Fix it\n- [x] Shipped\n" {
		t.Fatalf("README = %q", got)
	}
}
