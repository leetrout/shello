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

### With mise (recommended)

`shello` ships prebuilt binaries with every release, so [mise](https://mise.jdx.dev)
can install and pin it directly from GitHub — no toolchain required:

```bash
mise use -g github:leetrout/shello        # latest, on your PATH
mise use -g github:leetrout/shello@0.1.0  # or pin a version
```

Prefer a one-off? `mise x github:leetrout/shello -- shello`.

### From a release archive

Grab the archive for your OS/arch from the [releases page](https://github.com/leetrout/shello/releases),
verify it against `checksums.txt`, extract, and put `shello` on your PATH.

### From source

Requires Go 1.26+. Tooling is managed with mise:

```bash
mise install        # installs the pinned Go toolchain + dev tools
mise run build      # -> ./bin/shello
./bin/shello
```

Or run without building:

```bash
mise run run        # or: go run ./cmd/shello
```

## Usage

```bash
shello [-f path/to/board.json]
```

The board is stored as JSON and **auto-saves on every change**. By default it
lives in `shello.json` in the current directory; point `--file`/`-f` elsewhere to
keep multiple boards. On first run you get a small starter board.

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

Tooling (Go, gofumpt, goimports, golines, golangci-lint, gotestsum, govulncheck) is
pinned in `.mise.toml`. The same mise tasks run locally and in CI:

```bash
mise run fmt        # gofumpt + goimports + golines
mise run lint       # golangci-lint (default set + revive, 120-col)
mise run test       # gotestsum
mise run vulncheck  # govulncheck
mise run check      # fmt-check + vet + lint + test (what CI runs)
```

Git hooks are managed with [prek](https://github.com/j178/prek): `prek install`.

### Releasing

Releases are cut by [GoReleaser](https://goreleaser.com) — it cross-compiles the
binaries, builds archives with checksums and an SBOM, and creates the GitHub
release with a generated changelog. Tag and push to trigger it:

```bash
git tag v0.1.0
git push origin v0.1.0   # .github/workflows/release.yml runs `mise run release`
```

Dry-run the whole thing locally first (writes to `./dist`, publishes nothing):

```bash
mise run release-check     # validate .goreleaser.yaml
mise run release-snapshot  # cross-compile + archive locally
```

### Layout

| Path | Role |
| --- | --- |
| `cmd/shello/` | entry point and flags (`pflag`, `charmbracelet/log`) |
| `internal/board/` | `Board`/`Column`/`Card` model, JSON load/save, `MoveCard` |
| `internal/tui/model.go` | Bubble Tea model: keyboard + mouse handling, scroll & hit-testing geometry |
| `internal/tui/view.go` | Lip Gloss rendering, text wrapping |

See [`docs/adr/`](docs/adr/) for architecture decisions.
