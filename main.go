package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	path := flag.String("file", "shello.json", "path to the board JSON file")
	flag.Parse()

	board, err := LoadBoard(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading board:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		NewModel(board, *path),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // enables press/drag/release mouse events
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
