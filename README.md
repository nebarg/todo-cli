# todo-cli

A small TODO app that keeps your tasks in a Markdown file, with a terminal dashboard for working through them.

Use a terminal with a [Nerd Font](https://www.nerdfonts.com/) for the Git and branch icons; without one they show as empty boxes.

## Install

```sh
go install github.com/nebarg/todo-cli/cmd/todo@latest
```

Or build from a checkout with `go build ./cmd/todo`.

## Command line

With no arguments, `todo` opens the dashboard. With text, it adds a task and exits:

```sh
todo Test login failures
todo -p high @tests Fix flaky login test
todo -b Fix the bug on this branch
```

| Option | Effect |
| --- | --- |
| `-p` / `-priority high\|medium\|low` | Set the priority. A trailing `'!high'` in the text does the same; quote it, as shells treat `!` as history expansion |
| `@category` as the first word | File the task under a category. `-c` / `-category` does the same; use one or the other |
| `-b` / `-branch` | Add to the current Git branch |
| `-branch-name feature/login` | Add to another local Git branch |
| `-file path/to/tasks.md` | Use another task file, for the dashboard too |

Flags go before the task text. `todo add scan` adds a task literally named “scan”.

Other commands:

- `todo clear-done` removes every done task and lists what went. Inside a Git repository it also deletes branch sections whose local branch no longer exists, open tasks included.
- `todo scan [directory]` lists TODO comments in source files. See [TODOs in source files](#todos-in-source-files).

The task file is `todo.md` at the repository root, or in the current directory outside Git. It's created when you add the first task.

## Dashboard

### Tabs

- **1 General**: tasks outside branch sections. `▸ category` rows open to show that category's tasks; uncategorised tasks follow.
- **2 Branches**: every branch with tasks. At startup it opens on the current branch's tasks, if it has any.
- **3 Files**: TODO comments found in source files. Scanning runs in the background, and these are read only.

Each tab returns to where you left it. Press `1` again to leave an opened category, and `2` again to switch between the branch list and the current branch. `←` or `esc` goes back one level.

A branch whose local Git branch has been deleted shows as `⚠ branch-name  missing` in red. Its tasks stay visible but read only. Opening a branch re-checks it; `r` re-checks them all.

### Reading the list

- `●` in red, yellow or cyan: high, medium or low priority. `○`: no priority.
- `✓`: done. Done tasks sit at the end of each group.
- `⋯` at the end of a row: the task has details. Press `→` to read them.
- Counts such as `1/2` are done/total, for tabs, categories and branches. Files shows its number of matches, or `…` while scanning.
- The status bar under the list describes the highlighted row, and the footer shows the main keys for it.

The list keeps rows still while you work, so cycling `p` doesn't make a row jump. It re-sorts when it opens and when you press `r`.

### All tasks

Press `i` for a full-screen list of every Markdown task, with its category (`@auth`) or branch in the last column. `s` cycles between priority, branch and category order, and `i`, `esc` or `←` returns to the dashboard. The task keys work here too, with `enter` opening the edit form.

### Adding and editing

`a` adds a task where you are:

- in General, a general task, or one in the opened category
- in Branches, to the opened branch, or the current Git branch at the top level
- in Files or All tasks, a general task

`b` adds to the current Git branch from anywhere, or to the opened branch.

In the form:

- `tab` / `shift+tab` move between Task, Category or Branch, and Details.
- `enter` adds a new line in Task or Details, `ctrl+enter` saves, and `esc` cancels.
- A trailing `!high` in the title sets the priority.
- The Branch field suggests local Git branches as you type. You can only pick a branch that exists.
- When editing, changing the category or branch moves the task. A heading left empty is removed.

`c` changes just the category of a general task, without the form.

### Clearing done tasks

`X` clears done tasks from where you are: the opened category or branch, the whole tab, or everything in the All tasks view. Missing branches go too, open tasks included. Inside a missing branch, `X` deletes that branch.

A dialog shows what will be removed, including headings left empty. Only `y` goes ahead. Afterwards `u` undoes it, until the file next changes.

### Keys

| Key | Action |
| --- | --- |
| `1` `2` `3`, `tab` / `shift+tab` | Switch tab |
| `↑` `↓` / `j` `k` | Move |
| `→` | Open a category or branch, or a task's details |
| `←` / `esc` | Back |
| `i` | All tasks (`s` to change the sort) |
| `a` / `b` | Add a task / add a branch task |
| `e` / `enter` | Edit a task, or open a file TODO in your editor. `enter` also opens a category or branch |
| `d` / `space` | Mark done or reopen |
| `p` / `c` | Cycle priority / change category |
| `X` / `u` | Clear done / undo the clear |
| `r` | Reload the file and Git branches, and rescan files |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

File TODOs open in `$VISUAL`, then `$EDITOR`, falling back to `vi`. Vim, Neovim, VS Code, Codium and Cursor open at the TODO's line. When the editor exits, the Files tab rescans.

## Markdown format

```md
- [ ] Generic task

# tests

- [ ] Test login failures !high

  When a session expires, signing in again returns to the wrong page.
  Expected: return to the page the user was viewing.

# Branches

## feature/login

- [ ] Fix the flaky login test
```

- **Sections:** tasks before the first heading are general. `## branch-name` headings under `# Branches` are branch sections, and headings nested inside a branch stay part of it. Any other heading is a category.
- **Categories** can't contain spaces, and `Branches` is reserved. `auth`, `@auth` and `#auth` all mean the same category.
- **Details** are everything under a task until the next task or heading: paragraphs, lists, code, even indented checkboxes. The app writes them indented by two spaces.
- **Priority** is a trailing `!high`, `!medium` or `!low`. Only a last word that names a priority counts, so `Ship it!` and `Fix !important CSS` stay as they are. Old `- Priority:` and `- Labels:` lines now read as details.
- **Plain list items** such as `- Buy milk` are read as tasks, and become `- [ ] Buy milk` when edited.

The file stays readable without the app. When the app writes a task, it re-sorts that task's section: open tasks high, medium, low, then no priority, with done tasks last.

- Tasks move with their details, and tasks of equal priority keep your order.
- Notes above the first task and nested headings stay put, as do other sections.
- A compact list without blank lines stays compact.
- All other Markdown is preserved.

## TODOs in source files

`todo scan [directory]` lists case-insensitive `TODO` and `@todo` comments with their file and line. The dashboard's Files tab shows the same results, up to 1,000 matches.

- Scanning starts at the repository root and uses ripgrep (`rg`) if it's installed.
- It skips hidden, ignored, binary and Markdown files.
- `todo -all-files scan` includes Markdown and ignored or hidden text files. `.git` is always skipped.

## Development

```sh
go run ./cmd/todo
go test ./...
go tool golangci-lint run ./...
```

The linter version is pinned in `go.mod` and configured in `.golangci.yml`.
