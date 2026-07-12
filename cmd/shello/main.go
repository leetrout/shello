// Command shello is a Trello-style kanban board for the terminal.
package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/spf13/pflag"

	"github.com/leetrout/shello/internal/board"
	"github.com/leetrout/shello/internal/tui"
)

func main() {
	path := pflag.StringP("file", "f", "shello.json", "path to the board JSON file")
	pflag.Parse()

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
