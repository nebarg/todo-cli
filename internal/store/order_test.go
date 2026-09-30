package store

import "testing"

func TestWritesSortOnlyTheirSection(t *testing.T) {
	for _, item := range []struct {
		name, content, want string
		change              func(path string, tasks []Task) error
	}{
		{
			name:    "done task moves to the end with its details",
			content: "- [ ] First !low\n\n  About first.\n\n- [ ] Second !high\n\n# other\n\n- [ ] Unsorted\n\n- [ ] Hand order !high\n",
			want:    "- [ ] Second !high\n\n- [x] First !low\n\n  About first.\n\n# other\n\n- [ ] Unsorted\n\n- [ ] Hand order !high\n",
			change:  func(path string, tasks []Task) error { return Toggle(path, tasks[0]) },
		},
		{
			name:    "equal priorities keep their order",
			content: "- [ ] B !high\n\n- [ ] A !high\n\n- [ ] C\n",
			want:    "- [ ] B !high\n\n- [ ] A !high\n\n- [ ] C !high\n",
			change:  func(path string, tasks []Task) error { return SetPriority(path, tasks[2], PriorityHigh) },
		},
		{
			name:    "compact lists stay compact",
			content: "# docs\n- [ ] One\n- [ ] Two\n- [ ] Three\n",
			want:    "# docs\n- [ ] Three !medium\n- [ ] One\n- [ ] Two\n",
			change:  func(path string, tasks []Task) error { return SetPriority(path, tasks[2], PriorityMedium) },
		},
		{
			name:    "notes and nested headings stay put",
			content: "# Branches\n\n## feature/x\n\nNotes about the branch.\n\n- [ ] Plain\n\n- [ ] Raise me\n\n### Later\n\n- [ ] Nested low !low\n\n- [ ] Nested\n",
			want:    "# Branches\n\n## feature/x\n\nNotes about the branch.\n\n- [ ] Raise me !high\n\n- [ ] Plain\n\n### Later\n\n- [ ] Nested low !low\n\n- [ ] Nested\n",
			change:  func(path string, tasks []Task) error { return SetPriority(path, tasks[1], PriorityHigh) },
		},
		{
			name:    "trailing blank lines before the next heading stay",
			content: "- [x] Done\n- [ ] Open\n\n\n# next\n",
			want:    "- [ ] Open\n- [x] Done\n\n\n# next\n",
			change:  func(path string, tasks []Task) error { return SetPriority(path, tasks[1], PriorityNone) },
		},
		{
			name:    "windows line endings",
			content: "- [ ] One\r\n\r\n- [ ] Two\r\n",
			want:    "- [ ] Two !low\r\n\r\n- [ ] One\r\n",
			change:  func(path string, tasks []Task) error { return SetPriority(path, tasks[1], PriorityLow) },
		},
		{
			name:    "a new task joins its section in order",
			content: "- [ ] High !high\n\n- [ ] Plain\n\n- [x] Done\n",
			want:    "- [ ] High !high\n\n- [ ] New !medium\n\n- [ ] Plain\n\n- [x] Done\n",
			change:  func(path string, _ []Task) error { return Add(path, "New", "", PriorityMedium, "", "") },
		},
		{
			name:    "a moved task is sorted into its new section",
			content: "- [ ] Move me !high\n\n# docs\n\n- [ ] Doc\n\n- [x] Doc done\n",
			want:    "# docs\n\n- [ ] Move me !high\n\n- [ ] Doc\n\n- [x] Doc done\n",
			change:  func(path string, tasks []Task) error { return SetCategory(path, tasks[0], "docs") },
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			path, tasks := writeAndLoad(t, item.content)
			if err := item.change(path, tasks); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, path); got != item.want {
				t.Fatalf("file = %q\nwant   %q", got, item.want)
			}
		})
	}
}
