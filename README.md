# todo-cli

A small Markdown-backed TODO app with a full-width terminal task list.

## Build and run

```sh
go build -o todo .
./todo
```

Put the built `todo` binary in a directory on your `PATH` to run it as `todo` from anywhere. `go run .` also works during development.

With no arguments, `todo` opens the dashboard. With task text, it adds a task and exits:

```sh
todo Test login failures
todo -p high -c tests Fix flaky login test
todo -b Fix the bug on this branch
```

Flags go before the task text. `-p` accepts `high`, `medium`, or `low`; `-c` or `-category` assigns one category. Categories are a single word of letters and numbers, with an optional `@` or `#` prefix when entering them. `-b` puts the task under the current Git branch. Use `-branch-name feature/login` to name a branch explicitly. `todo add scan` adds a task literally named “scan”.

Inside a Git repository, the default file is `todo.md` at the repository root. Outside Git, it is `todo.md` in the current directory. Use `-file path/to/tasks.md` to choose another file. The file is created when you add the first task.

## Dashboard

The dashboard shows one full-width list at a time. Press `1`, `2`, or `3` to switch between **General**, **Git Branches**, and **Files**; `tab` and `shift+tab` cycle between them. General shows only tasks outside branch sections: its `@category` rows open to show matching category tasks, followed by generic tasks. Git Branches lists every branch with TODOs, with the current branch selected when it has tasks. Open a branch to see its tasks directly. Press `←` or `esc` to go back one level. Tasks are not indented beneath categories. The header reads **To Do**, and checklist counts show completed/total, including category and branch groups. Open tasks have an `○`; completed tasks have a dim `✓`, so titles stay aligned. A `⋯` after a task title means it has more details.

Press `i` for a full-screen list of every task in the Markdown file. Each row shows priority, category, branch, and task with its details on one line. The list uses the priority order established when the dashboard opened or was reloaded. Press `p`, `Shift+B`, or `c` there to sort by priority, branch, or category. Press `b` to add a branch task. Tasks without a category or branch come after named groups when sorting by category or branch. A single `!` marks high (red), medium (orange), and low (yellow) priority; `-` means none in the full-screen list. In the General and Branches lists, priority colours the task text without a marker. File TODOs from source scanning are not included in this Markdown task list.

Press `→` on a task to open a full-width detail page, with its title and description first, then status, scope, priority, and category. Press `←` or `esc` to return to the same list position. For a file TODO, the detail page shows nearby source lines. File scanning starts in the background when the dashboard opens, so task navigation remains available while the scan runs. Source matches are read only.

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | Cycle between lists |
| `1` / `2` / `3` | Show General / Git Branches / Files |
| `i` | Open the full-screen Markdown task list; press `i`, `esc`, or `←` to return |
| `p` / `Shift+B` / `c` in the full-screen list | Sort by priority / branch / category |
| `↑` / `↓`, `d` / `space`, `e` / `enter` in the full-screen list | Move, complete or reopen, edit a task |
| `enter` / `→` on a category or branch | Open its matching tasks |
| `←` inside a category or branch | Return to its list |
| `→` on a task, `←` or `esc` in details | Open the detail page / return to the task list |
| `↑` / `↓` or `k` / `j` | Move in a list or scroll the detail page |
| `a` | Add to the current place: General, an opened category, or the selected/open branch |
| `b` | Add a branch task from any pane |
| `d` or `space` | Complete or reopen a selected Markdown task |
| `e` or `enter` on a task | Open the edit form for a Markdown task; open a file TODO in your editor |
| `p` / `c` | Cycle priority / edit the category on a Markdown task |
| `v` | Jump to the list of branches |
| `enter` on a file TODO | Open that file at the line in `$VISUAL` or `$EDITOR` |
| `r` | Reload the Markdown file and rescan source files |
| `q` or `ctrl+c` | Quit |

The add form starts with a full-width, two-line task input, followed by a shorter **Category** or **Branch** field and **Details**. Category or Branch and Details have captions above their inputs; the focused caption turns yellow. Use `tab` or `↓` to move through fields, `shift+tab` or `↑` to move back, `ctrl+enter` to save, and `esc` to cancel. `Enter` adds a line in Task or Details; Task lines are joined into one Markdown checklist title when saved. Category is optional; type an existing category or a new one to create its Markdown heading. Branch is required for a branch task; type an existing branch name or a new name to create a Markdown branch section. This does not create or check out a Git branch. The edit form has Task and Details. The app adds the Markdown indentation for details. Pressing `a` at the General root adds a generic task, even if a category row is selected. Open a category first to prefill its category. In Git Branches, `a` prefills the highlighted or opened branch; `b` explicitly opens the branch add form from any view. Category editing uses a one-line prompt with `enter` to save; clearing it makes the task generic. Spaces are ignored in the category input, including pasted spaces. File TODOs open at the selected line in Vim, Neovim, VS Code, Codium, or Cursor; other editors open the file normally. VS Code-style editors use `--wait` so the dashboard reloads when editing finishes.

## Markdown format

```md
- [ ] Generic task

# tests

- [ ] Test login failures
  - Priority: High

  When a session expires, signing in again returns to the wrong page.
  Expected: return to the page the user was viewing.

# Branches

## feature/login

- [ ] Fix the flaky login test
```

Tasks before the first heading are generic. The app writes categories as `# Category`; when reading an external file, any non-branch heading defines a category. `# Branches` contains `## branch-name` headings, with tasks directly below each branch. The full-screen list includes all of them. A non-branch task has at most one category; categories can be entered as `auth`, `@auth`, or `#auth`, and the dashboard shows them as `@auth`. New categories may contain only letters and numbers, without spaces or punctuation; `Branches` is reserved. Existing categories in older files remain readable and can be renamed. Indent description paragraphs, lists, or code blocks by two spaces beneath a task; they appear on the detail page. Keep priority directly below the checkbox, before the description. Indented checkboxes are treated as part of the description, not separate tasks. Bare list items such as `- Buy milk` are readable and become `- [ ] Buy milk` when edited. Changing priority edits the task in place; the dashboard sorts tasks by priority when it opens and when you press `r`, keeping rows still between reloads. Older `- Labels:` metadata remains readable; changing a task's category moves it to a heading. Other Markdown is preserved when tasks are changed.

## TODOs in source files

Run `todo scan [directory]` for a plain list with filename and line number. The dashboard shows the same results automatically, limited to the first 1,000 matches. Scanning starts at the repository root by default. It uses ripgrep (`rg`) when available and falls back to a built-in parallel scanner otherwise.

The search finds case-insensitive `TODO` and `@todo` markers in common code comment forms. It skips hidden, ignored, and binary files. Markdown files are excluded so the task file and documentation do not appear as code TODOs. The built-in scanner uses Git's tracked and unignored file list when inside a repository; elsewhere it skips hidden files and common dependency/build directories.

Use `todo -all-files scan [directory]` to include Markdown and normally ignored or hidden text files. Git's internal `.git` directory stays excluded.
