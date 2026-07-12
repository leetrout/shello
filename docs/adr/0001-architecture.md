# 1. Architecture: Bubble Tea TUI with cmd/ + internal/ layout

Date: 2026-07-11

## Status

Accepted

## Context

shello is an interactive, full-screen kanban board for the terminal with keyboard
navigation and mouse drag-and-drop. It needs live rendering, input handling, and a
small persistent data model.

## Decision

- Build on the **charmbracelet** stack — **bubbletea** (event loop / model-update-view),
  **lipgloss** (styling/layout), **bubbles/textinput** (inline editing) — per the
  "interactive TUI" branch of the Go preferences. A cobra-style CLI framework is not used;
  a single `--file` flag is parsed with **pflag**.
- Use the standard Go layout: `cmd/shello/main.go` as the entrypoint, all logic under
  `internal/`.
- Split logic into two packages so geometry/rendering never leaks into the data model:
  - `internal/board` — `Board`/`Column`/`Card`, JSON load/save, and `MoveCard` (the one
    non-trivial data operation), with no UI dependencies.
  - `internal/tui` — the Bubble Tea `Model`: update logic, keyboard + mouse handling, and
    the lipgloss renderer.
- Persist the board as indented JSON, auto-saving on every mutation.

## Consequences

- `internal/board` is trivially unit-testable in isolation; `internal/tui` tests drive the
  `Model` through `Update` and assert on state and rendered frames.
- Card-wrapping geometry is computed once and shared between the renderer and mouse
  hit-testing, so drag-and-drop stays aligned with what's on screen.
- Adding a non-JSON backend later means implementing load/save behind the `board` package
  without touching the UI.
