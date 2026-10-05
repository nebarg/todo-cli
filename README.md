# Simple task management in the terminal

[todo](#todo) is a terminal dashboard for your tasks. They're kept in a plain Markdown file and shown alongside the TODOs in your README and the TODO comments in your code. You can also add tasks and clear done ones from the command line without opening the dashboard, so `todo` works in scripts too.

[todo-scan](#todo-scan) is a separate file scanner that lists the TODO comments found in your code on their own. It can be used in CI/CD pipelines or with the user interface. This is essentially a standalone "Files" tab from the todo app.

The two don't depend on each other, so you can delete whichever one you don't need.

Both read [todo-system](https://github.com/archtechx/todo-system)'s categories (`todo@auth`) and levels (`todo0`, `todo1`). You can [read why this is a pretty cool system](https://stancl.substack.com/p/organizing-todos-in-code) on Samuel's blog.

![The General tab of todo's dashboard](docs/todo.png)

> [!WARNING]
> The Git and branch icons need a [Nerd Font](https://www.nerdfonts.com/) in your terminal; without one they show as empty boxes.

## Index

- [Install](#install)
  - [Download](#download)
  - [Add them to your PATH](#add-them-to-your-path)
  - [Using Go](#using-go)
  - [zsh completion](#zsh-completion)
- [Try it](#try-it)
- [todo](#todo)
  - [Dashboard](#dashboard)
    - [Reading the list](#reading-the-list)
    - [All tasks](#all-tasks)
    - [Adding and editing](#adding-and-editing)
    - [Subtasks](#subtasks)
    - [Deleting and clearing](#deleting-and-clearing)
    - [Keys](#keys)
  - [Markdown format](#markdown-format)
  - [TODOs in README.md](#todos-in-readmemd)
- [todo-scan](#todo-scan)
- [TODO comments](#todo-comments)
  - [Skipping directories](#skipping-directories)
  - [todo-system syntax](#todo-system-syntax)
- [Development](#development)

## Install

### Download

Each [release](https://github.com/nebarg/todo-cli/releases) has an archive of both commands for macOS and Linux, on `arm64` or `amd64`. For the latest on an Apple silicon Mac:

```sh
curl -sL https://github.com/nebarg/todo-cli/releases/latest/download/todo-cli_darwin_arm64.tar.gz | tar -xz todo todo-scan
```

For an Intel Mac, use `darwin_amd64`, and for Linux, `linux_arm64` or `linux_amd64`.

### Add them to your PATH

To install them for your user only, without `sudo`, move them to `~/.local/bin`:

```sh
mkdir -p ~/.local/bin
mv todo todo-scan ~/.local/bin/
```

If `todo --version` then reports `command not found`, add that directory to your `PATH` in your shell's startup file and open a new terminal:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc   # zsh, the macOS default
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc  # bash
```

To install them for every user, move them to `/usr/local/bin`, which is on the `PATH` by default on macOS and most Linux distributions:

```sh
sudo mkdir -p /usr/local/bin
sudo mv todo todo-scan /usr/local/bin/
```

### Using Go

```sh
go install github.com/nebarg/todo-cli/cmd/todo@latest
go install github.com/nebarg/todo-cli/cmd/todo-scan@latest
```

`go install` puts them in `$(go env GOPATH)/bin`, which is `~/go/bin` by default, or in `$GOBIN` if it's set. Add that directory to your `PATH` as above. From a checkout, `go build ./cmd/todo ./cmd/todo-scan` builds both in the current directory.

### zsh completion

zsh ships a completion for DevTodo, a different program also called `todo`. Pressing Tab after `todo` runs this with DevTodo's flags, which prints `unknown flag: --format`. To turn that completion off, add this to `~/.zshrc` after `compinit`:

```zsh
compdef -d todo
```

## Try it

[`example`](example) has a task file with a category, priorities and two branches, a README with TODOs, and code with TODO comments:

```sh
cd example
todo -f todo.md
todo-scan code
```

## todo

```
todo [flags]                   open the dashboard
todo [flags] [@category] task  add a task
todo [flags] --clear-done      remove done tasks
todo [flags] --clear-missing   remove tasks of branches not in Git
```

```sh
todo                                   # open the dashboard
todo Update the changelog              # add a general task
todo -p high Fix the login redirect    # with a priority
todo @docs Write the install guide     # in the docs category
todo -c "release notes" Draft v2 notes # in a category with spaces
todo -c docs -p l Proofread the FAQ    # in the docs category at low priority
todo -b . Fix the flaky test           # on the current Git branch
todo -b feature/login Add 2FA          # on another branch
todo -f ~/notes/todo.md Renew passport # in another task file
todo -- -v flag is broken              # task text starting with a dash
todo --clear-done                      # remove done tasks
todo --clear-missing                   # remove the tasks of branches not in Git
todo -e dist -e node_modules           # open the dashboard, skipping these in Files
```

Everything after the flags is the task.

| Flag | Effect |
| --- | --- |
| `-p`, `--priority h\|high\|m\|medium\|l\|low` | Priority of the new task |
| `-c`, `--category name` | Category of the new task. For categories without spaces, you can use the shortcut `@category-name`. Use quotes for categories with spaces |
| `-b`, `--branch name` | Git branch of the new task. `.` means the current branch. A branch Git doesn't have locally still adds the task, the output will say it went to an unknown branch |
| `-f`, `--file path` | Use another task file instead of `todo.md` |
| `-e`, `--exclude dir` | Skip a directory in the dashboard's Files tab. See [Skipping directories](#skipping-directories) |
| `--clear-done` | Remove every done task, with its subtasks, and every done subtask, and list what went |
| `--clear-missing` | Remove every task, open or done, of branches Git doesn't have locally, with their headings. That includes branches not created yet. Git branches aren't changed. Needs Git |
| `--version` | Print the version |
| `-h`, `--help` | Show usage |

- The task file is `todo.md` at the Git repository root, or in the current directory outside Git. If there's no `todo.md` but there is a `TODO.md`, or the name in any other case, that file is used instead. Otherwise `todo.md` is created when you add the first task.
- A task goes in the general list, a category or a branch. Branch tasks can't have a category. `Branches` is reserved as a category name.

### Dashboard

![The README.md group, a task's details, the branch list, a branch's tasks, the Files tab and a TODO's details](docs/todo.gif)

The dashboard has three tabs:

- **1 General**: your tasks outside branch sections. Each category is a `▸ category` row you can open, and `▸ README.md` opens [your README's TODOs](#todos-in-readmemd). Tasks without a category are listed after them.
- **2 Branches**: every branch with tasks. At startup it opens on the current branch, if that branch has any tasks.
- **3 Files**: the [TODO comments](#todo-comments) in files under the working directory. Scanning runs in the background, and these are read only.

Use `↑` and `↓` to move. Up from the first row goes to the last, and down from the last goes back to the first. `→` opens a category, branch or task, and `←` or `esc` goes back one level.

Each tab remembers where you left it. Pressing `1` again goes back to the top of General, and `3` again goes back out of a TODO or category. Pressing `2` again switches between the branch list and the current branch.

A branch Git doesn't have locally, because it was deleted or isn't created yet, shows as `⚠ branch-name not in Git` in red, and the status bar inside it says so too. Its tasks work like any others. The dashboard checks with Git at startup and when you press `r`. If you've switched branch since the Branches tab opened the current one, `r` opens the new current branch, or the branch list if that branch has no tasks.

#### Reading the list

- `●` in red, yellow or cyan is a high, medium or low priority task. `○` has no priority.
- `✓` is a done task.
- `⋯` at the end of a row means the task has details. Press `→` to read them.
- Indented rows are [subtasks](#subtasks).
- Counts such as `1/2` are done/total, for tabs, categories and branches. They don't include subtasks. The Files tab shows how many TODOs it found, or `…` while it scans.
- The status bar under the list describes the highlighted row, and the footer shows the main keys for it.

Rows stay where they are while you work, so marking a task done doesn't make the list jump. The list is sorted when the dashboard starts and when you press `r`: open tasks first, by priority, then done tasks. A task you add goes after the open tasks of its group until then.

#### All tasks

`i` opens a full-screen list of every task in your task file, with its category (`@auth`) or branch in the last column. `s` switches between priority, branch and category order, and `i`, `esc` or `←` takes you back to the dashboard. The task keys work here too, and `enter` opens the edit form.

#### Adding and editing

`a` adds a task where you are:

- in General, a general task, or one in the category you've opened
- in Branches, a task on the branch you've opened, or on the current Git branch from the branch list
- in Files or All tasks, a general task
- in README.md, nothing, as your README is read only

`b` adds a task to the current Git branch from anywhere, or to the branch you've opened.

To edit a task, press `e` or `enter`. The form has three fields: Task, Category (or Branch for a branch task) and Details.

- `tab` and `shift+tab` move between the fields.
- `enter` starts a new line in Task or Details. `ctrl+enter` saves, and `esc` cancels.
- Category suggests an existing category in grey as you type. `tab` fills it in, and `ctrl+n` / `ctrl+p` switch between the categories that match.
- Branch suggests your local Git branches as you type. A name Git doesn't have is offered last, marked `not in Git`, for a branch you haven't created yet.
- Changing the category or branch moves the task there, with its subtasks. A heading left empty is removed.

`c` changes just the category of a general task, without opening the form.

#### Subtasks

A subtask is a checkbox indented under a task in your task file. The dashboard shows subtasks indented under their task, in General, Branches and All tasks.

- To add a subtask, open a task with `→` and press `a`. The new subtask goes after the task's other subtasks. If you've opened a subtask instead, `a` adds another subtask to the same task.
- A subtask's form only has the Task field, because a subtask always stays in its task's category or branch. End the text with `!high`, `!medium` or `!low` to give it a priority.
- `d`, `p`, `e` and `backspace` work on a subtask just as on a task. `c` doesn't.
- When you mark a task done, its subtasks are greyed out, but they aren't ticked. When the list is sorted, subtasks move with their task, open ones first.
- The status bar of a task with subtasks shows how many of them are done.

Deleting a task deletes its subtasks too. Clearing done tasks removes a done task with all of its subtasks, including open ones. A done subtask of an open task is cleared on its own.

#### Deleting and clearing

`backspace` deletes the selected task, with its details and subtasks. On a category or branch row, it deletes every task in it, done or not, and removes the heading once nothing else is under it.

`X` (`shift+x`) clears done tasks from where you are: the category or branch you've opened, the whole tab, or everything in the All tasks view.

Both show a dialog saying what will go, including any subtasks and headings left empty, and only `y` goes ahead. Afterwards `u` undoes it, as long as the file hasn't changed since.

Your README is read only, so neither works on its TODOs.

#### Keys

| Key | Action |
| --- | --- |
| `1` `2` `3`, `tab` / `shift+tab` | Switch tab |
| `↑` `↓` / `k` `j` | Move up or down, wrapping round at either end |
| `→` | Open a category, branch or task |
| `←` / `esc` | Back |
| `i` | All tasks (`s` to change the order) |
| `a` | Add a task. On an opened task, add a subtask |
| `b` | Add a task to a branch |
| `e` / `enter` | Edit a task, or open a TODO in your editor. `enter` also opens a category or branch |
| `d` / `space` | Mark done or reopen |
| `p` | Cycle the priority |
| `c` | Change the category |
| `backspace` | Delete a task, or a category or branch with all of its tasks |
| `X` (`shift+x`) | Clear done tasks |
| `u` | Undo the last clear or delete |
| `r` | Re-sort the tasks, check Git branches again, and rescan files |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

File and README TODOs open in `$VISUAL`, then `$EDITOR`, falling back to `vi`. Vim, Neovim, VS Code, Codium and Cursor open at the TODO's line. When the editor closes, the tasks reload and the Files tab rescans.

The dashboard also watches the task file and `README.md`, so changes made outside it, by an editor, a script or an agent, show up within a second, with each task keeping its place.

### Markdown format

```md
- [ ] Generic task

# tests

- [ ] Test login failures !high

  When a session expires, signing in again returns to the wrong page.
  Expected: return to the page the user was viewing.

  - [ ] Reproduce it in a test
  - [x] Find the redirect handler

# Branches

## feature/login

- [ ] Fix the flaky login test
```

- **Sections:** tasks before the first heading are general. `## branch-name` headings under `# Branches` are branch sections, and headings nested inside a branch stay part of it. Any other heading is a category.
- **Categories** can contain spaces, and `Branches` is reserved. `auth`, `@auth` and `#auth` all mean the same category, and so do names that only differ in case or spacing, such as `Release  Notes` and `release notes`.
- **Details** are everything under a task, up to the next task or heading, apart from its subtasks: paragraphs, lists and code. The app writes them indented by two spaces.
- **Subtasks** are checkboxes indented under a task, such as `Reproduce it in a test`. They only go one level deep: anything indented under a subtask, even a checkbox, is that subtask's details. A checkbox you type into the form's Details becomes a subtask.
- **Priority** is a trailing `!high`, `!medium` or `!low`. Only a last word that names a priority counts, so `Ship it!` and `Fix !important CSS` stay as they are.
- **Plain list items** such as `- Buy milk` are tasks too, and become `- [ ] Buy milk` when you edit them.
- **Formatting:** `**bold**`, `*italic*`, `~~strikethrough~~` and `` `code` `` show formatted in the dashboard, as do `__bold__`, `_italic_` and `~strikethrough~`. Struck-through text is greyed out as well, for terminals that don't draw strikethrough, such as macOS Terminal. Underscores inside words, as in `user_id`, stay as written, and `\*` writes a literal `*`. Code blocks in details show as written, and the edit form shows the Markdown itself.

When the app writes a task, it sorts that task's section by priority: high, medium, low, then none. Everything else is left as you wrote it:

- Tasks of the same priority keep your order.
- Marking a task done or reopening it doesn't move it, so a task you reopen is back where it was.
- Tasks move with their details and subtasks, and subtasks keep the order you wrote them in.
- Notes above the first task, nested headings and other sections stay where they are.
- A compact list without blank lines stays compact.
- All other Markdown is kept as it is.

### TODOs in README.md

The dashboard also reads the TODO list in the `README.md` next to your task file, following todo-system's rules for READMEs. You'll find its tasks under `▸ README.md` in General. They aren't copied into `todo.md`.

```md
## TODOs

- Write the install guide
- [ ] todo0 Fix the broken example
  - the link to the API docs is a 404
  - [ ] Check the other links

### Dashboard

- [ ] Show the README's headings
```

Which tasks are read:

- The list items (`- foo` or `- [ ] foo`) under a heading called `TODO` or `TODOs`, in any case, with or without a `:`. The list ends at the next heading of the same level or higher, such as `## Install` after `## TODOs`.
- A deeper heading inside the list, such as `### Dashboard`, groups the tasks under it. `▸ README.md` shows a row for each heading, in the README's order, then the tasks that aren't under one. A heading inside another, such as a `####` under a `###`, is a group of its own. todo-system itself stops at any heading, so it won't see these tasks.
- Nested list items are [subtasks](#subtasks), such as `Check the other links`. Anything nested deeper shows as a subtask of the same task.
- Plain items under a checkbox task aren't subtasks. Like the 404 note above, they're the task's details, along with any other text indented under it.
- Anything in a ` ``` ` code block is skipped.

What you can do with them:

- `d` marks a task done or reopens it. Only the checkbox changes, and one is added to a plain `- foo`. The README's order is left alone.
- `e` opens the README in your editor at the task.
- Nothing else changes your README: `a`, `p`, `c`, `backspace`, `X` and the edit form only work on your task file.

The tasks show their [formatting](#markdown-format), and levels such as `todo0` show and sort as they do in the Files tab.

## todo-scan

![todo-scan browsing the example's TODO comments](docs/todo-scan.png)

`todo-scan` is the dashboard's Files tab on its own, for any directory. It doesn't read or create a task file.

```sh
todo-scan                       # browse the TODOs under the current directory
todo-scan ~/Code/api            # under another directory
todo-scan -e dist -e vendor     # skipping these directories
todo-scan --list                # print them as path:line: text, most urgent first
todo-scan --check               # print how many there are, and exit 1 if any
todo-scan --list --check        # the list on stdout, the count on stderr
todo-scan --levels --list       # only levelled TODOs, todo0 to todo9
todo-scan --level 0+ --check    # only todo0, todo00 and so on, and exit 1 if any
todo-scan --level 0,1 --list    # only todo0 and todo1
```

`--check` exits with 1 if any TODOs are found, allowing you to fail your CI/CD pipeline.

| Flag | Effect |
| --- | --- |
| `--list` | Print every TODO as `path:line: text`, most urgent first |
| `--check` | Print how many TODOs there are, such as `3 TODOs`, and exit 1 if there are any. With `--list`, the count goes to stderr |
| `--levels` | Only levelled TODOs, `todo0` to `todo9` |
| `--level level` | Only TODOs at this level: `0` matches `todo0`, `00` matches `todo00`, and `0+` any number of zeros. Repeat it, or separate levels with commas |
| `-e`, `--exclude dir` | Skip a directory. See [Skipping directories](#skipping-directories) |
| `--version` | Print the version |
| `-h`, `--help` | Show usage |

- The directory defaults to the current one. Without `--list` or `--check`, it opens the browser, with the Files tab's keys plus `r` to rescan and `q` to quit.
- `--levels` and `--level` apply to the browser too.
- Given a directory, `--list` starts each path with it in its shortest form: `todo-scan --list ../api` prints paths such as `../api/main.go`, and `todo-scan --list ./api/` paths such as `api/main.go`.
- Exit status: 1 when `--check` finds TODOs, 2 for errors such as a missing directory or an unknown flag, and 0 otherwise.

A CI step that fails while any levelled TODOs remain:

```yaml
- name: No levelled TODOs
  run: todo-scan --list --check --levels
```

## TODO comments

The Files tab and `todo-scan` list case-insensitive `TODO` and `@todo` comments with their file and line. A marker counts when it starts the comment, as in `// TODO fix`, or is followed by `:` or `(` anywhere in it, so `* @return todo` doesn't match. In commented-out code, a comment after the code counts as starting there, as in `// x = 1; // TODO drop x`. `@ todo` is read as `@todo`. [todo-system markers](#todo-system-syntax) count anywhere in a comment.

Both show the comment text first and a shortened path beside it. The status bar shows the full path of the highlighted TODO. `→` opens a detail page with the TODO's text and as much of the surrounding code as fits, and `e` opens the file in your editor.

- Only comments count. Each file is read with the comment syntax for its extension, so code and strings such as `class Todo {`, `"TODO"`, CSS's `#todo` and C's `#define TODO` don't match. Comments spanning several lines are followed to their end.
- A file with PHP in it is read as PHP whatever its extension, and HTML and template files also count `//` and `/* */` comments in their scripts and styles. Files of an unknown type are read a line at a time, guessing where each line's comment starts.
- Gitignored, binary and Markdown files, files over 1 MB, and directories starting with `.` are skipped. Hidden files, such as `.eslintrc.js`, are read.

### Skipping directories

`node_modules` and `vendor` are skipped by default. To skip others, give `-e` / `--exclude` once per directory, to `todo` or `todo-scan`:

```sh
todo -e node_modules -e vendor -e dist
todo-scan -e ./web/generated
```

- A bare name, such as `dist`, skips every directory with that name.
- Anything with a slash, such as `./web/generated`, is a path from the working directory.
- Using `-e` replaces the defaults, so you will need to exclude `node_modules` and `vendor` again if you still want them skipped.

### todo-system syntax

[todo-system](https://github.com/archtechx/todo-system)'s markers count anywhere in a comment:

| Marker | Meaning |
| --- | --- |
| `todo@boundary Split this` | Category: listed under a `▸ boundary` row |
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
go tool -modfile=tools/go.mod golangci-lint run ./...
go tool -modfile=tools/go.mod govulncheck ./...
```

The linter and govulncheck versions are pinned in their own module, `tools/go.mod`, and the linter is configured in `.golangci.yml`.
