package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nebarg/todo-cli/internal/project"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestSameVersionSeesEveryKindOfChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	writeFile(t, path, "- [ ] Task\n")
	before := stat(t, path)
	if !sameVersion(before, stat(t, path)) {
		t.Fatal("an untouched file changed")
	}
	if !sameVersion(nil, nil) || sameVersion(before, nil) || sameVersion(nil, before) {
		t.Fatal("a missing file compared wrongly")
	}

	if err := os.Chtimes(path, time.Time{}, before.ModTime().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if sameVersion(before, stat(t, path)) {
		t.Fatal("a new modification time went unseen")
	}

	writeFile(t, path, "- [ ] Longer task\n")
	if err := os.Chtimes(path, time.Time{}, before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if sameVersion(before, stat(t, path)) {
		t.Fatal("a new size went unseen")
	}

	// A file renamed over the original, as the store and many editors
	// write, differs only in being another file.
	writeFile(t, path, "- [ ] Task\n")
	if err := os.Chtimes(path, time.Time{}, before.ModTime()); err != nil {
		t.Fatal(err)
	}
	before = stat(t, path)
	replacement := filepath.Join(dir, "replacement.md")
	writeFile(t, replacement, "- [ ] Task\n")
	if err := os.Chtimes(replacement, time.Time{}, before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if sameVersion(before, stat(t, path)) {
		t.Fatal("a replaced file went unseen")
	}
}

func TestWatchFilesReportsOnlyOnceSomethingChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	missing := filepath.Join(dir, "README.md")
	writeFile(t, path, "- [ ] Task\n")
	paths := []string{path, missing}
	msgs := make(chan tea.Msg, 1)
	go func() { msgs <- watchFiles(paths, statFiles(paths), time.Millisecond)() }()

	select {
	case msg := <-msgs:
		t.Fatalf("reported %v with nothing changed", msg)
	case <-time.After(50 * time.Millisecond):
	}

	// Renamed into place, so no check can find it half written.
	written := filepath.Join(dir, "written.md")
	writeFile(t, written, "## TODOs\n\n- Write docs\n")
	if err := os.Rename(written, missing); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-msgs:
		changed, ok := msg.(filesChangedMsg)
		if !ok || len(changed.stats) != 2 || changed.stats[1] == nil {
			t.Fatalf("msg = %#v", msg)
		}
		if !sameVersion(changed.stats[1], stat(t, missing)) {
			t.Fatal("the stats are not the files as found")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a new README.md went unreported")
	}
}

func TestWatchFilesWithoutStatsReportsAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.md")
	writeFile(t, path, "- [ ] Task\n")
	if _, ok := watchFiles([]string{path}, nil, time.Millisecond)().(filesChangedMsg); !ok {
		t.Fatal("the first check without stats reported nothing")
	}
}

func TestOutsideEditsReloadKeepingRowsInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.md")
	writeFile(t, path, "- [ ] Low !low\n\n- [ ] High !high\n")
	m, err := newModel(path, project.Context{}, testFiles())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "- [ ] Low !low\n\n- [x] High !high\n\n- [ ] Added by an agent !high\n")
	writeFile(t, filepath.Join(dir, "README.md"), "## TODOs\n\n- Write docs\n")
	updated, cmd := m.Update(filesChangedMsg{})
	m = updated.(*model)
	if cmd == nil {
		t.Fatal("watching stopped after a change")
	}
	rows := m.rows(generalPane)
	if len(rows) != 4 || rows[0].kind != rowReadme || rows[1].todo.Text != "High" || !rows[1].todo.Done ||
		rows[2].todo.Text != "Low" || rows[3].todo.Text != "Added by an agent" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestOwnWritesDontReloadOrLoseUndo(t *testing.T) {
	m, path := clearModel(t)
	selectTask(t, m, "Loose open")
	m = press(press(m, "backspace"), "y")
	updated, cmd := m.Update(filesChangedMsg{})
	m = updated.(*model)
	if cmd == nil || m.lastRemoval == nil {
		t.Fatalf("own delete: watching %v, undo %v", cmd != nil, m.lastRemoval != nil)
	}
	if m = press(m, "u"); fileContent(t, path) != clearContent {
		t.Fatalf("undo failed: %q", m.status)
	}
}

func TestUnreadableFilesShowAnError(t *testing.T) {
	m, path := clearModel(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	updated, cmd := m.Update(filesChangedMsg{})
	m = updated.(*model)
	if footer := ansi.Strip(m.renderFooter(200)); cmd == nil || m.status == "" || !strings.Contains(footer, m.status) {
		t.Fatalf("watching %v, status %q, footer %q", cmd != nil, m.status, footer)
	}
}
