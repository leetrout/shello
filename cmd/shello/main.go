// Command shello is a Trello-style kanban board for the terminal.
package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/spf13/pflag"

	"github.com/leetrout/shello/internal/board"
	"github.com/leetrout/shello/internal/tui"
)

// Build metadata, injected by GoReleaser via -ldflags (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	path := pflag.StringP("file", "f", "shello.json", "path to the board JSON file")
	showVersion := pflag.BoolP("version", "v", false, "print version and exit")
	pflag.Parse()

	if *showVersion {
		fmt.Printf("shello %s (%s) built %s\n", version, commit, date)
		return
	}

	b, err := board.Load(*path)
	if err != nil {
		log.Fatal("could not load board", "path", *path, "err", err)
	}

	p := tea.NewProgram(
		tui.New(b, *path),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // enables press/drag/release mouse events
	)
	if _, err := p.Run(); err != nil {
		log.Fatal("shello exited with an error", "err", err)
	}
}
