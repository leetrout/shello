// Package board holds shello's kanban data model and its JSON persistence.
package board

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Card is a single Trello-style card.
type Card struct {
	Title string `json:"title"`
}

// Column is a Trello-style list holding an ordered set of cards.
type Column struct {
	Title string `json:"title"`
	Cards []Card `json:"cards"`
}

// Board is the whole kanban board: an ordered set of columns.
type Board struct {
	Columns []Column `json:"columns"`
}

// Clone returns a deep copy of the board that shares no slice backing arrays
// with the original, so one can be mutated while the other is kept (e.g. on an
// undo stack).
func (b Board) Clone() Board {
	cols := make([]Column, len(b.Columns))
	for i, c := range b.Columns {
		cards := make([]Card, len(c.Cards))
		copy(cards, c.Cards) // Card is value-only, so a shallow copy suffices
		cols[i] = Column{Title: c.Title, Cards: cards}
	}
	return Board{Columns: cols}
}

// Default returns a starter board so a first run isn't empty.
func Default() Board {
	return Board{Columns: []Column{
		{Title: "Todo", Cards: []Card{
			{Title: "Welcome to shello 👋"},
			{Title: "Press ? for help"},
			{Title: "Drag me to another column"},
		}},
		{Title: "Doing", Cards: []Card{
			{Title: "Move cards with H / L"},
		}},
		{Title: "Done", Cards: []Card{
			{Title: "Read the help bar"},
		}},
	}}
}

// Load reads a board from path, returning a default board if the file does not
// exist. Any other error is returned to the caller.
func Load(path string) (Board, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Board{}, err
	}
	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return Board{}, err
	}
	if b.Columns == nil {
		b.Columns = []Column{}
	}
	return b, nil
}

// Save writes the board to path as indented JSON, creating parent dirs.
func (b Board) Save(path string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// MoveCard removes the card at (fromCol, fromIdx) and inserts it into toCol at
// toIdx, clamping indices so it is always safe to call. It returns the resting
// (column, index) of the moved card.
func (b *Board) MoveCard(fromCol, fromIdx, toCol, toIdx int) (int, int) {
	if fromCol < 0 || fromCol >= len(b.Columns) {
		return fromCol, fromIdx
	}
	src := &b.Columns[fromCol]
	if fromIdx < 0 || fromIdx >= len(src.Cards) {
		return fromCol, fromIdx
	}
	card := src.Cards[fromIdx]
	// remove from source
	src.Cards = append(src.Cards[:fromIdx], src.Cards[fromIdx+1:]...)

	if toCol < 0 {
		toCol = 0
	}
	if toCol >= len(b.Columns) {
		toCol = len(b.Columns) - 1
	}
	// If moving within the same column past the removal point, no adjustment is
	// needed because we already removed the element; clamp to new length.
	dst := &b.Columns[toCol]
	if toIdx < 0 {
		toIdx = 0
	}
	if toIdx > len(dst.Cards) {
		toIdx = len(dst.Cards)
	}
	dst.Cards = append(dst.Cards, Card{})
	copy(dst.Cards[toIdx+1:], dst.Cards[toIdx:])
	dst.Cards[toIdx] = card
	return toCol, toIdx
}
