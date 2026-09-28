# todo-cli

A small Markdown-backed TODO app with a three-panel terminal dashboard.

## Build and run

```sh
go build -o todo .
./todo
```

Put the built `todo` binary in a directory on your `PATH` to run it as `todo` from anywhere. `go run .` also works during development.

With no arguments, `todo` opens the dashboard. With task text, it adds a task and exits:

```sh
todo Test login failures
todo -p high -l tests Fix flaky login test
todo -b Fix the bug on this branch
```

Flags go before the task text. `-p` accepts `high`, `medium`, or `low`; `-l` or `-label` assigns one label. `-b` puts the task under the current Git branch. Use `-branch-name feature/login` to name a branch explicitly. `todo add scan` adds a task literally named “scan”.

Inside a Git repository, the default file is `TODO.md` at the repository root. Outside Git, it is `TODO.md` in the current directory. Use `-file path/to/tasks.md` to choose another file. The file is created when you add the first task.

## Dashboard

The dashboard shows **General**, **Branches**, and **File TODOs** together. General shows only tasks outside branch sections: its `@label` rows open to show matching general tasks, followed by unlabeled general tasks. The Branches panel lists every branch with TODOs, with the current branch selected when it has tasks. Open a branch to see its `@label` rows and unlabeled tasks; open a label to see its tasks. Press `←` to go back one level. Tasks are not indented beneath labels. Markdown checkbox markers are hidden in the dashboard; completed tasks use a checkmark.

Press `i` for a full-screen list of every task in the Markdown file. Each row shows priority, label, branch, and task with its details on one line. The list opens sorted by priority, highest first. Press `p`, `b`, or `l` there to sort by priority, branch, or label. Unlabeled and general tasks come after named groups when sorting by label or branch. Priority markers are `!!!` red for high, `!!` orange for medium, `!` yellow for low, and `-` for none; the same markers and colours appear in the dashboard. File TODOs from source scanning are not included in this Markdown task list.

The right pane puts the selected task's title and description first, with status, scope, priority, and label below. For a file TODO it shows nearby source lines. File scanning starts in the background when the dashboard opens, so task navigation remains available while the scan runs. Source matches are read only.

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Move between panels |
| `1` / `2` / `3` | Focus General / Branches / File TODOs |
| `i` | Open the full-screen Markdown task list; press `i`, `Esc`, or `←` to return |
| `p` / `b` / `l` in the full-screen list | Sort by priority / branch / label |
| `↑` / `↓`, `Space` / `Enter`, `e` in the full-screen list | Move, complete or reopen, edit a task |
| `Enter` / `→` on a label or branch | Open its matching tasks |
| `←` inside a label or branch | Return to its list |
| `→` / `←` on a task | Focus the detail pane / return to the task list |
| `↑` / `↓` or `k` / `j` | Move in a list or scroll the focused detail pane |
| `a` / `b` | Add a general / branch task; the active label or selected branch is used |
| `Space` or `Enter` | Complete or reopen a selected Markdown task |
| `e` | Open the edit form for a Markdown task; open a file TODO in your editor |
| `p` / `l` | Cycle priority / edit the single label on a Markdown task |
| `v` | Jump to the list of branches |
| `Enter` on a file TODO | Open that file at the line in `$VISUAL` or `$EDITOR` |
| `r` | Reload the Markdown file and rescan source files |
| `q` or `Ctrl+C` | Quit |

The add and edit form has a title and a multiline details field. Press `Tab`, `↓`, or `Enter` from the title to reach details. Press `Ctrl+S` to save or `Esc` to cancel. The app adds the Markdown indentation for details. Adding from a label group applies that label; adding from a selected branch puts the task under that branch. Label editing uses a one-line prompt with `Enter` to save; clearing it makes the task unlabeled. File TODOs open at the selected line in Vim, Neovim, VS Code, Codium, or Cursor; other editors open the file normally. VS Code-style editors use `--wait` so the dashboard reloads when editing finishes.

## Markdown format

```md
## General

- [ ] Unlabeled task

### @tests

- [ ] Test login failures
  - Priority: High

  When a session expires, signing in again returns to the wrong page.
  Expected: return to the page the user was viewing.

## Branches

### feature/login

#### @tests

- [ ] Fix the flaky login test
```

General tasks stay in the General panel; branch tasks stay under their branch. The full-screen list includes both. Each task has at most one label. A General label is a `### @label` heading; a branch label is a `#### @label` heading below its `### branch-name`. Labels can be entered as `auth`, `@auth`, or `#auth`; the dashboard shows them as `@auth`. Indent description paragraphs, lists, or code blocks by two spaces beneath a task; they appear in the dashboard's detail pane. Keep priority directly below the checkbox, before the description. Indented checkboxes are treated as part of the description, not separate tasks. Older `- Labels:` metadata remains readable; changing a task's label moves it to a heading. Other Markdown is preserved when tasks are changed.

## TODOs in source files

Run `todo scan [directory]` for a plain list with filename and line number. The dashboard shows the same results automatically, limited to the first 1,000 matches. Scanning starts at the repository root by default. It uses ripgrep (`rg`) when available and falls back to a built-in parallel scanner otherwise.

The search finds case-insensitive `TODO` and `@todo` markers in common code comment forms. It skips hidden, ignored, and binary files. Markdown files are excluded so the task file and documentation do not appear as code TODOs. The built-in scanner uses Git's tracked and unignored file list when inside a repository; elsewhere it skips hidden files and common dependency/build directories.

Use `todo -all-files scan [directory]` to include Markdown and normally ignored or hidden text files. Git's internal `.git` directory stays excluded.
