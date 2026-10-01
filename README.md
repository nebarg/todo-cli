# todo-cli

A small TODO app that keeps your tasks in a Markdown file, with a terminal dashboard for working through them.

It also finds TODO comments in your code and the TODO list in your README, and understands [todo-system](https://github.com/archtechx/todo-system)'s categories (`todo@boundary`) and priorities (`todo0`, `todo1`).

Use a terminal with a [Nerd Font](https://www.nerdfonts.com/) for the Git and branch icons; without one they show as empty boxes.

## Install

```sh
go install github.com/nebarg/todo-cli/cmd/todo@latest
```

Or build from a checkout with `go build ./cmd/todo`.

## Command line

```
todo [flags]                     open the dashboard
todo [flags] [@category] task    add a task
todo [flags] --scan [directory]  list TODO comments in source files
todo [flags] --clear-done        remove done tasks
todo [flags] --clear-missing     remove tasks of branches no longer in Git
```

Flags come first, and everything after them is the task, so a task can start with any word. To start a task with a dash, put `--` before it: `todo -- -v flag is broken`.

```sh
todo Update the changelog
todo @tests Fix the flaky login test
todo -b . -p h Fix the bug on this branch
```

| Flag | Effect |
| --- | --- |
| `-p`, `--priority h\|high\|m\|medium\|l\|low` | Priority of the new task |
| `-c`, `--category name` | Category of the new task. A leading `@name` word does the same; use one or the other |
| `-b`, `--branch name` | Local Git branch of the new task. `.` means the current branch |
| `-f`, `--file path` | Use another task file instead of `todo.md` |
| `-e`, `--exclude dir` | Skip a directory in `--scan` and the dashboard's Files tab. See [Skipping directories](#skipping-directories) |
| `--all-files` | With `--scan`, include Markdown, hidden and ignored files |
| `--scan [directory]` | List TODO comments under the directory, or here. See [TODOs in source files](#todos-in-source-files) |
| `--clear-done` | Remove every done task, and list what went |
| `--clear-missing` | Remove every task of branches whose local Git branch no longer exists, open ones included, and their headings. Your Git branches aren't touched. Needs Git. Give both `--clear-` flags to do both |
| `-h`, `--help` | Show usage |

A task goes in one place: the general list, a category, or a branch. Branch tasks can't have a category. Categories are one word, and `Branches` is reserved; a branch must already exist locally.

The task file is `todo.md` at the repository root, or in the current directory outside Git. It's created when you add the first task.

## Dashboard

### Tabs

- **1 General**: tasks outside branch sections. `▸ category` rows open to show that category's tasks, and `▸ README.md` opens [your README's TODOs](#todos-in-readmemd); uncategorised tasks follow.
- **2 Branches**: every branch with tasks. At startup it opens on the current branch's tasks, if it has any.
- **3 Files**: TODO comments in source files under the working directory, with [todo-system](#todo-system-syntax) categories and levels. Scanning runs in the background, and these are read only.

Each tab returns to where you left it. Press `1` or `3` again to leave an opened category, and `2` again to switch between the branch list and the current branch. `←` or `esc` goes back one level.

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
- The Branch field suggests local Git branches as you type. You can only pick a branch that exists.
- When editing, changing the category or branch moves the task. A heading left empty is removed.

`c` changes just the category of a general task, without the form.

### Clearing done tasks

`X` clears done tasks from where you are: the opened category or branch, the whole tab, or everything in the All tasks view. The tasks of missing branches go too, open ones included. Inside a missing branch, `X` removes all of its tasks.

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
| `e` / `enter` | Edit a task, or open a file or README TODO in your editor. `enter` also opens a category or branch |
| `d` / `space` | Mark done or reopen |
| `p` / `c` | Cycle priority / change category |
| `X` / `u` | Clear done / undo the clear |
| `r` | Reload the file and Git branches, and rescan files |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

File and README TODOs open in `$VISUAL`, then `$EDITOR`, falling back to `vi`. Vim, Neovim, VS Code, Codium and Cursor open at the TODO's line. When the editor exits, the tasks reload and the Files tab rescans.

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

## TODOs in README.md

As in todo-system, the dashboard reads the TODO list in the `README.md` beside the task file, so at the repository root by default. They're listed under `▸ README.md` in General, and never copied into `todo.md`.

```md
## TODOs

- Write the install guide
- [ ] todo0 Fix the broken example
```

- Tasks are the list items (`- foo` or `- [ ] foo`) directly under a heading reading `TODO` or `TODOs`, with or without a `:`, in any case. The next heading ends the list.
- Nested list items are tasks too, and anything in a ` ``` ` code block is skipped.
- `d` marks a task done or reopens it. It changes only the checkbox, adding one to a plain `- foo`, and leaves the README's order alone.
- Levels such as `todo0` show and sort as in the Files tab. `p`, `c`, `X` and the edit form don't apply; `e` opens the README in your editor at the task.

## TODOs in source files

`todo --scan [directory]` lists case-insensitive `TODO` and `@todo` comments with their file and line. A marker counts when it starts the comment, as in `// TODO fix`, or is followed by `:` or `(` anywhere in it, so `* @return todo` doesn't match. [todo-system markers](#todo-system-syntax) count anywhere in a comment.

The dashboard's Files tab shows the same results, up to 1,000 matches, with the comment text first and a shortened path beside it. The status bar shows the full path of the highlighted TODO. `→` opens a detail page with the TODO's text and as much of the surrounding code as fits, and `e` opens the file in your editor.

- Scanning covers the working directory and below, and uses ripgrep (`rg`) if it's installed.
- It skips gitignored, hidden, binary and Markdown files. `todo --all-files --scan` includes them, apart from binaries.
- Directories starting with `.` are always skipped, as are `node_modules` and `vendor` by default.

### Skipping directories

To skip other directories, use `-e` / `--exclude`, once per directory. It works for `--scan` and the dashboard:

```sh
todo -e node_modules -e vendor -e dist --scan
todo -e ./web/generated
```

- A bare name, such as `dist`, skips every directory with that name.
- Anything with a slash, such as `./web/generated`, is a path from the working directory.
- Giving `-e` replaces the defaults, so list `node_modules` and `vendor` again if you still want them skipped.

### todo-system syntax

The scanner supports [todo-system](https://github.com/archtechx/todo-system)'s markers, anywhere in a comment:

| Marker | Meaning |
| --- | --- |
| `todo@boundary Split this` | Category: listed under a `▸ boundary` row in the Files tab |
| `todo000`, `todo00`, `todo0` | Priority levels: the more zeros, the more urgent |
| `todo1` … `todo9` | Lower priority levels, in order |
| `TODO: fix`, `todo refactor` | Generic |

- Category rows open like categories in General, and `3` again goes back.
- Levels are listed first, most urgent at the top, with the level beside the text. Zero levels are red, and other levels yellow.
- Four or more zeros are labelled `0x4` to `0x9`, then `0x9+`, so the column stays narrow. They still sort by the real count.
- As in todo-system, a TODO has a category or a level, not both: `todo1@boundary` is just in `boundary`.
- Numbers todo-system doesn't accept, such as `todo12`, are treated as generic rather than hidden.
- `todo --scan` lists levels first too, then the rest by file.

## Development

```sh
go run ./cmd/todo
go test ./...
go tool golangci-lint run ./...
```

The linter version is pinned in `go.mod` and configured in `.golangci.yml`.
