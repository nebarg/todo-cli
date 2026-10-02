# Example

A task file, this README and some code with TODO comments, to try `todo` and `todo-scan` on. From this directory:

```sh
todo -f todo.md
```

Inside this repository, `todo` uses the `todo.md` at the repository root, so `-f` picks this one.

- **General** has tasks of each priority, open and done, and a `design` category. `▸ README.md` opens the TODOs at the end of this README.
- **Branches** has `main`, which a clone of this repository has, and `doesnt-exist`, which it doesn't, so it shows as missing.
- **Files** lists the TODO comments in [`code`](code): generic ones, todo-system levels from `todo000` to `todo5`, and categories such as `todo@auth`.

The dashboard writes its changes to `todo.md` and this README. `git restore .` in this directory undoes them.

To list the TODO comments without the dashboard:

```sh
todo-scan --list code
```

## TODOs

- [ ] todo0 Fix the broken link in the docs
- Add a screenshot of each tab
- [x] Write this README
