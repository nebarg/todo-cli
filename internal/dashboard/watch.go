package dashboard

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
)

// pollInterval is how often the dashboard checks the files it shows tasks
// from, so edits made outside it, such as by an agent, show up.
const pollInterval = time.Second

// filesChangedMsg reports that the files the dashboard reads changed on disk.
// stats are the files as the check found them.
type filesChangedMsg struct{ stats []os.FileInfo }

// watchFiles checks paths every interval in the background, and reports once
// any of them is created, removed or replaced, or changes size or
// modification time, since stats. Checking in the background rather than
// sending a message each interval means the dashboard only updates when
// something changed. With no stats, the first check reports a change.
func watchFiles(paths []string, stats []os.FileInfo, interval time.Duration) tea.Cmd {
	return func() tea.Msg {
		for {
			time.Sleep(interval)
			now := statFiles(paths)
			if !slices.EqualFunc(stats, now, sameVersion) {
				return filesChangedMsg{stats: now}
			}
		}
	}
}

// statFiles stats each path, with nil for one that can't be statted, which
// reading it will then report.
func statFiles(paths []string) []os.FileInfo {
	stats := make([]os.FileInfo, len(paths))
	for i, path := range paths {
		if info, err := os.Stat(path); err == nil {
			stats[i] = info
		}
	}
	return stats
}

// sameVersion reports whether a and b, stats of one path, are the same file
// with the same size and modification time. nil is a missing file.
func sameVersion(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// readFiles reads each path, with nil for a missing one.
func readFiles(paths []string) ([][]byte, error) {
	contents := make([][]byte, len(paths))
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		contents[i] = data
	}
	return contents, nil
}

// watchedFiles are the files the dashboard shows tasks from: the task file,
// then the README.
func (m *model) watchedFiles() []string { return []string{m.file, m.readmeFile()} }

// watch starts checking the watched files for changes since stats.
func (m *model) watch(stats []os.FileInfo) tea.Cmd {
	return watchFiles(m.watchedFiles(), stats, pollInterval)
}

// filesChanged reloads the tasks if the files no longer hold what the
// dashboard last read, keeping each row in its place, and watches again.
// The dashboard's own writes change the files too, but it reads them straight
// after, so they don't reload and drop the undo a clear or delete left.
func (m *model) filesChanged(msg filesChangedMsg) tea.Cmd {
	contents, err := readFiles(m.watchedFiles())
	if err != nil || !slices.EqualFunc(contents, m.contents, bytes.Equal) {
		if err := m.readTasks(false, nil); err != nil {
			m.status = err.Error()
		}
	}
	return m.watch(msg.stats)
}
