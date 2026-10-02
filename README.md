# todo-cli

A TODO app that keeps tasks in a Markdown file, with a terminal dashboard.

It also lists TODO comments in source files and the TODO list in a README, including [todo-system](https://github.com/archtechx/todo-system)'s categories (`todo@boundary`) and levels (`todo0`, `todo1`). [`todo-scan`](#todo-scan) lists TODO comments on their own, without a task file.

Use a terminal with a [Nerd Font](https://www.nerdfonts.com/) for the Git and branch icons; without one they show as empty boxes.

## Install

```sh
go install github.com/nebarg/todo-cli/cmd/todo@latest
go install github.com/nebarg/todo-cli/cmd/todo-scan@latest
```

Install either or both. From a checkout, `go build ./cmd/todo ./cmd/todo-scan`.

## Command line

```
todo [flags]                   open the dashboard
todo [flags] [@category] task  add a task
todo [flags] --clear-done      remove done tasks
todo [flags] --clear-missing   remove tasks of branches no longer in Git
```

Flags come first; everything after them is the task. To start a task with a dash, put `--` before it: `todo -- -v flag is broken`.

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
| `-e`, `--exclude dir` | Skip a directory in the dashboard's Files tab. See [Skipping directories](#skipping-directories) |
| `--clear-done` | Remove every done task, and list what went |
| `--clear-missing` | Remove every task, open or done, of branches whose local Git branch no longer exists, with their headings. Git branches aren't changed. Needs Git |
| `-h`, `--help` | Show usage |

A task goes in the general list, a category or a branch. Branch tasks can't have a category. Categories are one word, and `Branches` is reserved. A branch must exist locally.

The task file is `todo.md` at the repository root, or in the current directory outside Git. It's created when you add the first task.

## Dashboard

### Tabs

- **1 General**: tasks outside branch sections. `▸ category` rows open to show that category's tasks, and `▸ README.md` opens [your README's TODOs](#todos-in-readmemd); uncategorised tasks follow.
- **2 Branches**: every branch with tasks. At startup it opens on the current branch's tasks, if it has any.
- **3 Files**: TODO comments in source files under the working directory, with [todo-system](#todo-system-syntax) categories and levels. Scanning runs in the background, and these are read only.

Each tab returns to where you left it. Press `1` or `3` again to leave an opened category, and `2` again to switch between the branch list and the current branch. `←` or `esc` goes back one level.

A branch whose local Git branch has been deleted shows as `⚠ branch-name  missing` in red. Its tasks stay visible but read only. Opening a branch re-checks it; `r` re-checks them all. If Git has switched branch since the Branches tab opened the current one, `r` opens the new current branch instead, or the branch list if it has no tasks.

### Reading the list

- `●` in red, yellow or cyan: high, medium or low priority. `○`: no priority.
- `✓`: done. Done tasks sit at the end of each group.
- `⋯` at the end of a row: the task has details. Press `→` to read them.
- Counts such as `1/2` are done/total, for tabs, categories and branches. Files shows its number of matches, or `…` while scanning.
- The status bar under the list describes the highlighted row, and the footer shows the main keys for it.

Rows keep their place after edits. The list re-sorts when it opens and when you press `r`.

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
- **Priority** is a trailing `!high`, `!medium` or `!low`. Only a last word that names a priority counts, so `Ship it!` and `Fix !important CSS` stay as they are.
- **Plain list items** such as `- Buy milk` are read as tasks, and become `- [ ] Buy milk` when edited.

When the app writes a task, it re-sorts that task's section: open tasks by priority (high, medium, low, none), then done tasks.

- Tasks move with their details, and tasks of equal priority keep your order.
- Notes above the first task and nested headings stay put, as do other sections.
- A compact list without blank lines stays compact.
- All other Markdown is preserved.

## TODOs in README.md

The dashboard reads the TODO list in the `README.md` next to the task file, which is the repository root by default, following todo-system's rules for READMEs. The tasks are listed under `▸ README.md` in General and aren't copied into `todo.md`.

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

The dashboard's Files tab and [`todo-scan`](#todo-scan) list case-insensitive `TODO` and `@todo` comments with their file and line. A marker counts when it starts the comment, as in `// TODO fix`, or is followed by `:` or `(` anywhere in it, so `* @return todo` doesn't match. In commented-out code, a comment after the code counts as starting there, as in `// x = 1; // TODO drop x`. `@ todo` is read as `@todo`. [todo-system markers](#todo-system-syntax) count anywhere in a comment.

Both show the comment text first and a shortened path beside it. The status bar shows the full path of the highlighted TODO. `→` opens a detail page with the TODO's text and as much of the surrounding code as fits, and `e` opens the file in your editor.

- The dashboard scans the working directory and below, and `todo-scan` the directory you give it. Both use ripgrep (`rg`) if it's installed.
- It skips gitignored, binary and Markdown files, and directories starting with `.`. Hidden files, such as `.eslintrc.js`, are read.
- Only comments count. Each file is read with the comment syntax for its extension, so code and strings such as `class Todo {`, `"TODO"`, CSS's `#todo` and C's `#define TODO` don't match. Comments spanning several lines are followed to their end.
- A file with PHP in it is read as PHP whatever its extension, and HTML and template files also count `//` and `/* */` comments in their scripts and styles. Files of an unknown type are read a line at a time, guessing where each line's comment starts.
- `node_modules` and `vendor` are skipped by default.

### Skipping directories

To skip other directories, use `-e` / `--exclude`, once per directory. It works for the dashboard and `todo-scan`:

```sh
todo -e node_modules -e vendor -e dist
todo-scan -e ./web/generated
```

- A bare name, such as `dist`, skips every directory with that name.
- Anything with a slash, such as `./web/generated`, is a path from the working directory.
- Giving `-e` replaces the defaults, so list `node_modules` and `vendor` again if you still want them skipped.

### todo-scan

`todo-scan` is the Files tab on its own, full screen. It doesn't read or create a task file.

```sh
todo-scan
todo-scan ~/Code/other-project
todo-scan -e dist web
```

The directory defaults to the current one. `-e` / `--exclude` works as above, with paths taken from where you run it. The keys are the Files tab's, plus `r` to rescan and `q` to quit.

`--list` and `--check` print instead of opening the browser:

| Command | Prints | Exit status |
| --- | --- | --- |
| `todo-scan --list` | Every TODO, as `path:line: text`, most urgent first | 0 |
| `todo-scan --check` | How many there are, such as `3 TODOs` | 1 if there are any, 0 if not |
| `todo-scan --list --check` | The list on stdout, the count on stderr | 1 if there are any, 0 if not |

`--levels` limits the browser, `--list` and `--check` to levelled TODOs, `todo0` to `todo9`. `--level` limits them to the levels given: `--level 0` matches `todo0` only, `--level 00` matches `todo00`, and `--level 0+` matches any number of zeros. Repeat it, or separate levels with commas:

```sh
todo-scan --level 0+
todo-scan --list --level 0 --level 1
todo-scan --check --level 0+,1
```

Errors, such as a missing directory or an unknown flag, exit with 2.

A CI step that fails while any levelled TODOs remain:

```yaml
- name: No urgent TODOs
  run: todo-scan --list --check --levels
```

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
- Four or more zeros are labelled `0x4` to `0x9`, then `0x9+`. They sort by the real count.
- As in todo-system, a TODO has a category or a level, not both: `todo1@boundary` is just in `boundary`.
- Levels todo-system doesn't accept, such as `todo11`, have no level. They count as generic TODOs where a plain `TODO` would: at the start of a comment, or before `:` or `(`.

## Development

```sh
go run ./cmd/todo
go test ./...
go tool golangci-lint run ./...
```

The linter version is pinned in `go.mod` and configured in `.golangci.yml`.
