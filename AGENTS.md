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
