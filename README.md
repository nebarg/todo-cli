# todo-cli

A small Markdown-backed TODO app with a full-width terminal task list.

It looks best in a terminal using a [Nerd Font](https://www.nerdfonts.com/): the Git and branch icons come from it, and without one they show as empty boxes.

## Build and run

```sh
go build ./cmd/todo
./todo
```

Or install it straight onto your `PATH` (in `$(go env GOPATH)/bin`):

```sh
go install github.com/nebarg/todo-cli/cmd/todo@latest
```

During development, run these from the repository root: `go run ./cmd/todo`, `go test ./...`, and `go tool golangci-lint run ./...` for linting (the linter is pinned in `go.mod`, configured in `.golangci.yml`).

With no arguments, `todo` opens the dashboard. With task text, it adds a task and exits:

```sh
todo Test login failures
todo -p high @tests Fix flaky login test
todo -b Fix the bug on this branch
```

Flags go before the task text. `-p` accepts `high`, `medium`, or `low`. Start the task text with `@category` to file it under a category (for example `todo @boundary Fix the thing`); only the first word counts, so an `@` later in the title stays part of it. `-c` or `-category` does the same as a flag; use one or the other, not both. Categories can contain any characters except whitespace, so names such as `+v1` and `bug-fix` work. An optional leading `@` or `#` is treated as a prefix. `-b` puts the task under the current Git branch. Use `-branch-name feature/login` to choose an existing local Git branch explicitly. `todo add scan` adds a task literally named “scan”.

Inside a Git repository, the default file is `todo.md` at the repository root. Outside Git, it is `todo.md` in the current directory. Use `-file path/to/tasks.md` to choose another file. The file is created when you add the first task.

## Dashboard

The dashboard shows one full-width list at a time. Press `1`, `2`, or `3` to switch between **General**, **Git Branches**, and **Files**; `tab` and `shift+tab` cycle between them. General shows only tasks outside branch sections: its `▸ category` rows open to show matching category tasks, followed by a blank line and the generic tasks. Git Branches lists every branch with TODOs, with the current branch selected when it has tasks and marked *current* in green. Category and branch rows show their completed/total count right-aligned. A saved TODO group whose local Git branch no longer exists shows `⚠ branch-name  missing`, in red with *missing* in italics; its tasks remain visible but read only, and inside the branch, the status bar explains that the branch no longer exists. Opening a branch re-checks that its Git branch still exists, so the warning and read-only lock are current while you view its tasks; press `r` to refresh every branch's status in the list. Open a branch to see its tasks directly; a breadcrumb such as `General › @Docs  1/2` shows where you are and that group's completed/total count. Press `←` or `esc` to go back one level. Tasks are not indented beneath categories. The top bar has the tabs on the left and the repository and current Git branch on the right, marked with a Git icon (needs a Nerd Font); on narrow terminals the repository name, then the branch, give way to the tabs. Each tab shows its completed/total count (Files shows the number of matches, or `…` while scanning). Open tasks with a priority have a filled `●` coloured red (high), yellow (medium), or cyan (low); an empty grey `○` means no priority. The `?` help lists the same legend. Completed tasks have a dim `✓`, so titles stay aligned. A `⋯` at the right edge of a row means the task has more details. The footer lists the most useful keys for the selected row, dropping the least important ones on narrow terminals; press `?` for every key.

Press `i` for a full-screen list of every task in the Markdown file. Each row starts with the task and its details; the last column shows its category or branch. A branch has a `` (Nerd Font branch) icon, while a category has an `@` prefix. Priority colours the task's `○` as in the dashboard; completed tasks are dim. The list starts in priority order when the dashboard opens or reloads. Press `s` to cycle through priority, branch, and category sorting. Press `p` to change a selected task's priority, `c` to edit its category, `a` to add a general task, or `b` to add a branch task; after saving, the dashboard opens with the new task selected. Completed tasks sort after open tasks: at the end of the whole list in priority order, or at the end of their branch, category, or general group in the other orders. The General and Branches panes also keep completed tasks at the end of each task group. Tasks without a category or branch come after named groups when sorting by category or branch. File TODOs from source scanning are not included in this Markdown task list.

A status bar along the bottom of the General and Branches panels describes the highlighted row: a task's status and priority (for example `Open  ·  ● High priority`), or how many of a category's or branch's tasks are done. Press `→` on a task to open a full-width detail page with its title and description; the same status bar sits at the bottom. Press `←` or `esc` to return to the same list position. For a file TODO, the detail page shows nearby source lines. File scanning starts in the background when the dashboard opens, so task navigation remains available while the scan runs. Source matches are read only.

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | Cycle between lists |
| `1` / `2` / `3` | Show General / Git Branches / Files |
| `i` | Open the full-screen Markdown task list; press `i`, `esc`, or `←` to return |
| `s` in the full-screen list | Cycle priority / branch / category sorting |
| `↑` / `↓`, `d` / `space`, `e` / `enter` in the full-screen list | Move, complete or reopen, edit a task |
| `enter` / `→` on a category or branch | Open its matching tasks |
| `←` inside a category or branch | Return to its list |
| `→` on a task, `←` or `esc` in details | Open the detail page / return to the task list |
| `a` | Add to General (including from the full-screen list), an opened category, the current Git branch from the branch list, or an opened branch |
| `b` | Add a branch task from any pane |
| `d` or `space` | Complete or reopen a selected Markdown task |
| `e` or `enter` on a task | Open the edit form for a Markdown task; open a file TODO in your editor |
| `p` / `c` | Cycle priority / edit the category on a Markdown task |
| `v` | Jump to the list of branches |
| `enter` on a file TODO | Open that file at the line in `$VISUAL` or `$EDITOR` |
| `r` | Reload Markdown and Git branches, and rescan source files |
| `?` | Show all keys; any key closes it |
| `q` or `ctrl+c` | Quit |

The add form starts with a full-width, two-line task input, followed by a shorter **Category** or **Branch** field and **Details**. The heading is a breadcrumb such as `General › New task`. Category or Branch and Details have captions above their inputs; the focused field has a yellow bar beside it and its caption turns yellow. Save errors replace the key hints at the bottom of the form. Use `tab` or `↓` to move through fields, `shift+tab` or `↑` to move back, `ctrl+enter` to save, and `esc` to cancel. `Enter` adds a line in Task or Details; Task lines are joined into one Markdown checklist title when saved. Category is optional; type an existing category or a new one to create its Markdown heading. Branch is required for a branch task. The Branch field searches local Git branches as you type and shows two matches at a time, with a scroll indicator when more are available. `↑` and `↓` cycle through matches, wrapping at either end; `enter` or `tab` accepts the highlighted branch, and `shift+tab` returns to Task. Typing replaces the prefilled current branch. Saving creates a Markdown branch section if needed, but only for a Git branch that still exists locally. The edit form has the same fields as the add form, and its breadcrumb shows where the task is now, e.g. `General › @auth › Edit task`. Change Category to move a general task to another category (clear it to make the task generic), or pick another local branch to move a branch task; the list follows the task to its new place, and a heading left empty by the move is removed. `c` from a task list still changes a category without opening the form. The app adds the Markdown indentation for details. Pressing `a` at the General root adds a generic task, even if a category row is selected. Open a category first to prefill its category. In the Git Branches list, `a` uses the current Git branch when the form opens, even if another branch is highlighted. Inside a branch’s task list, `a` and `b` use that branch. Elsewhere, `b` opens the branch add form with the current Git branch. Category editing uses a one-line prompt with `enter` to save; clearing it makes the task generic. Spaces are ignored in the category input, including pasted spaces. File TODOs open at the selected line in Vim, Neovim, VS Code, Codium, or Cursor; other editors open the file normally. VS Code-style editors use `--wait` so the dashboard reloads when editing finishes.

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

Tasks before the first heading are generic. The app writes categories as `# Category`; when reading an external file, any non-branch heading defines a category. `# Branches` contains `## branch-name` headings, with tasks directly below each branch. The full-screen list includes all of them. A non-branch task has at most one category; categories can be entered as `auth`, `@auth`, or `#auth`, and the dashboard shows them as `@auth`. New categories can use punctuation and symbols but cannot contain whitespace; `Branches` is reserved. Existing categories in older files remain readable and can be renamed. Indent description paragraphs, lists, or code blocks by two spaces beneath a task; they appear on the detail page. Keep priority directly below the checkbox, before the description. Indented checkboxes are treated as part of the description, not separate tasks. Bare list items such as `- Buy milk` are readable and become `- [ ] Buy milk` when edited. Changing priority edits the task in place; the dashboard sorts tasks by priority when it opens and when you press `r`, keeping rows still between reloads. Categories come only from Markdown headings; an old `- Labels:` line under a task is treated as part of its description. Other Markdown is preserved when tasks are changed.

## TODOs in source files

Run `todo scan [directory]` for a plain list with filename and line number. The dashboard shows the same results automatically, limited to the first 1,000 matches. Scanning starts at the repository root by default. It uses ripgrep (`rg`) when available and falls back to a built-in parallel scanner otherwise.

The search finds case-insensitive `TODO` and `@todo` markers in common code comment forms. It skips hidden, ignored, and binary files. Markdown files are excluded so the task file and documentation do not appear as code TODOs. The built-in scanner uses Git's tracked and unignored file list when inside a repository; elsewhere it skips hidden files and common dependency/build directories.

Use `todo -all-files scan [directory]` to include Markdown and normally ignored or hidden text files. Git's internal `.git` directory stays excluded.
