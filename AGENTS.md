# Agent instructions

## Preferences

This project follows Lee's standing dev stack & tooling preferences. Consult them before
making tooling, library, or architecture decisions:

@~/code/github.com/leetrout/dev-tool-notes/AGENTS.md

Precedence: the project's own conventions win, then these preferences, then generic priors.

## This project

`shello` is a Trello-style kanban TUI built on the **charmbracelet** stack (bubbletea +
lipgloss + bubbles), per the "interactive TUI" branch of `stack/go.md`.

- **Layout:** `cmd/shello/` entrypoint · `internal/board` (data model + JSON persistence)
  · `internal/tui` (Bubble Tea model, update, view).
- **Toolchain:** managed by **mise** (`.mise.toml`). Run everything through mise tasks so
  local and CI match: `mise run fmt` · `lint` · `test` · `vet` · `vulncheck` · `check` · `build`.
- **Quality:** gofumpt + goimports + golines formatting; golangci-lint (default set + revive,
  120-col); testify + gotestsum for tests. Git hooks via **prek** (`.pre-commit-config.yaml`).
- **Before committing:** `mise run check` must pass.

## Using shello as a task tool (for agents)

An agent can read and manage a shello board by operating directly on its JSON file — no
need to drive the TUI.

- **The board is one JSON file.** Default `shello.json` in the cwd; override with `-f
  <path>`. Shape is `{"columns":[{"title","cards":[{"title","note"}]}]}`. A card has a `title`
  and an optional `note` — a path to a markdown file (resolved relative to the board file's
  dir) holding its long-form context; in the TUI `o` opens it in `$EDITOR` and cards with a
  note show a 📎. See [issue #5](https://github.com/leetrout/shello/issues/5) for a fuller
  markdown mode. It's plain JSON — safe to read, hand-edit, and write back.
- **To add/edit/move/delete a todo,** edit the JSON: append to a column's `cards`, change a
  `title`, or move a card object between `columns[].cards`. Preserve the array order — it is
  the on-screen order. Keep it valid JSON (UTF-8, the same 2-space indent shello writes).
- **To attach context to a todo,** write your markdown to a file (e.g. `notes/<slug>.md`) and
  set that path as the card's `"note"`. The user opens it in shello with `o`. Do this while
  the TUI is closed (see the concurrency caveat) — otherwise set it via the TUI's `o` prompt.
- **Concurrency caveat — the big one.** The TUI loads the file **once at startup** and
  **rewrites the whole file on every change**; there is no file watching. So:
  - Don't edit the JSON while a human has the board open — the TUI won't see your edit and
    will clobber it on its next save.
  - Edit the file when the TUI is **not** running, then the user opens (or reopens) shello to
    see your changes. Coordinate before touching a live board.
- **Inspecting without the TUI:** `cat shello.json` / parse it. `shello -v` prints version.
- **Don't** shell out to run the interactive TUI expecting to script it — it's an alt-screen
  Bubble Tea program meant for a human. Manipulate the JSON instead.
