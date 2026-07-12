// Package board holds shello's kanban data model and its JSON persistence.
package board

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Card is a single Trello-style card. Note, when set, is a path to a markdown
// file holding the card's long-form context; it is resolved relative to the
// directory of the board file (see internal/tui). Empty means no attachment.
type Card struct {
	Title string `json:"title"`
	Note  string `json:"note,omitempty"`
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

// groupRefs collapses a set of {column, index} card references into a
// per-column list of valid, de-duplicated indices, dropping any ref that is out
// of range for the current board. It is the shared front-end for the batch
// operations below.
func groupRefs(b *Board, refs [][2]int) map[int][]int {
	seen := make(map[[2]int]bool, len(refs))
	byCol := make(map[int][]int)
	for _, r := range refs {
		if seen[r] {
			continue
		}
		seen[r] = true
		col, idx := r[0], r[1]
		if col < 0 || col >= len(b.Columns) {
			continue
		}
		if idx < 0 || idx >= len(b.Columns[col].Cards) {
			continue
		}
		byCol[col] = append(byCol[col], idx)
	}
	return byCol
}

// DeleteCards removes every card named in refs (each a {column, index} pair).
// Within a column the indices are removed high-to-low so earlier removals don't
// invalidate later ones. Out-of-range and duplicate refs are ignored.
func (b *Board) DeleteCards(refs [][2]int) {
	for col, idxs := range groupRefs(b, refs) {
		sort.Sort(sort.Reverse(sort.IntSlice(idxs)))
		for _, i := range idxs {
			cards := b.Columns[col].Cards
			b.Columns[col].Cards = append(cards[:i], cards[i+1:]...)
		}
	}
}

// MoveCardsByColumn shifts each card named in refs by delta columns (delta is
// normally -1 or +1). Within a source column the selected cards are removed and
// appended — preserving their top-to-bottom order — to the end of the
// destination column (source+delta). Cards whose destination column is out of
// range are left in place. Duplicate and out-of-range refs are ignored.
func (b *Board) MoveCardsByColumn(refs [][2]int, delta int) {
	type move struct {
		dst   int
		cards []Card
	}
	// Capture and detach every source column first, then append, so a column
	// that is both a source and a destination isn't seen mid-mutation.
	var moves []move
	for col, idxs := range groupRefs(b, refs) {
		dst := col + delta
		if dst < 0 || dst >= len(b.Columns) {
			continue // at the edge: leave these cards where they are
		}
		sort.Ints(idxs)
		cards := make([]Card, len(idxs))
		for k, i := range idxs {
			cards[k] = b.Columns[col].Cards[i]
		}
		for k := len(idxs) - 1; k >= 0; k-- {
			i := idxs[k]
			src := b.Columns[col].Cards
			b.Columns[col].Cards = append(src[:i], src[i+1:]...)
		}
		moves = append(moves, move{dst: dst, cards: cards})
	}
	for _, mv := range moves {
		b.Columns[mv.dst].Cards = append(b.Columns[mv.dst].Cards, mv.cards...)
	}
}
