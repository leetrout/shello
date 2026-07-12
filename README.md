# shello

A Trello-style kanban board that lives in your terminal. Columns fill the
screen, cards wrap and scroll, and you move cards around with either the
keyboard or the mouse — drag-and-drop included.

[![asciicast](https://asciinema.org/a/1260654.svg)](https://asciinema.org/a/1260654)

```
 shello ▪ trello in your terminal

 Todo •3               Doing •1              Done •1
 ────────────────     ────────────────     ────────────────
 Welcome to shello    Move cards with      Read the help bar
 👋                   H / L
 Press ? for help
 Drag me to another
 column

 hjkl/↑↓←→ move cursor · space grab & move card · a add · e edit · … · q quit
```

## Install

Requires Go 1.26+.

```bash
go build -o shello .
./shello
```

Or run without building:

```bash
go run .
```

## Usage

```bash
shello [-file path/to/board.json]
```

The board is stored as JSON and **auto-saves on every change**. By default it
lives in `shello.json` in the current directory; point `-file` elsewhere to keep
multiple boards. On first run you get a small starter board.

## Keys

### Navigate
| Key | Action |
| --- | --- |
| `←` `→` / `h` `l` | move the cursor between columns |
| `↑` `↓` / `j` `k` | move the cursor between cards |
| `g` / `G` | jump to the first / last card |

### Move a card
| Key | Action |
| --- | --- |
| `space` | **grab** the selected card, then use the arrows to carry it between columns and reorder it; `space`/`enter`/`esc` to **drop** |
| `H` `L` | move the selected card to the previous / next column |
| `J` `K` | reorder the selected card up / down |
| *mouse* | click a card and **drag** it to any column |

### Edit
| Key | Action |
| --- | --- |
| `a` | add a card to the current column |
| `e` / `enter` | edit the selected card (saving it empty deletes it) |
| `d` / `x` | delete the selected card |
| `n` | new column |
| `r` | rename the current column |
| `<` `>` | move the current column left / right |
| `D` | delete the current column (asks for confirmation first) |

### Other
| Key | Action |
| --- | --- |
| `s` | save now (auto-save is always on) |
| `?` | toggle the full help |
| `q` / `ctrl+c` | quit |

## Mouse

`shello` enables mouse tracking, so you can:

- **Drag and drop** cards between columns — the source card dims, the target
  column header lights up green, and the footer shows where it will land.
- **Scroll** a column with the wheel when it has more cards than fit.

## How it scales

- Columns divide the terminal width evenly and fill it edge to edge; they resize
  live as you resize the terminal.
- Card text **wraps** to as many rows as it needs instead of being truncated.
- Each column **scrolls independently** when its cards overflow — the keyboard
  cursor auto-scrolls to stay visible, the mouse wheel scrolls the column under
  the pointer, and `↑`/`↓` hints in the column header show there's more.

## Board format

```json
{
  "columns": [
    {
      "title": "Todo",
      "cards": [
        { "title": "Welcome to shello 👋" }
      ]
    }
  ]
}
```

It's plain JSON — edit it by hand if you like; `shello` reads it on startup and
writes it back on every change.

## Development

```bash
go test ./...   # unit tests for card moves, wrapping, scrolling, hit-testing
go vet ./...
```

| File | Role |
| --- | --- |
| `board.go` | `Board`/`Column`/`Card` model, JSON load/save, `moveCard` |
| `model.go` | Bubble Tea model: keyboard + mouse handling, scroll & hit-testing geometry |
| `view.go` | Lip Gloss rendering, text wrapping |
| `main.go` | entry point and flags |
