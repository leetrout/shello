package main

import "testing"

func titles(c Column) []string {
	out := make([]string, len(c.Cards))
	for i, x := range c.Cards {
		out[i] = x.Title
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sample() Board {
	return Board{Columns: []Column{
		{Title: "A", Cards: []Card{{"a0"}, {"a1"}, {"a2"}}},
		{Title: "B", Cards: []Card{{"b0"}}},
	}}
}

func TestMoveCardAcrossColumns(t *testing.T) {
	b := sample()
	col, idx := b.moveCard(0, 1, 1, 0) // a1 -> front of B
	if col != 1 || idx != 0 {
		t.Fatalf("resting pos = (%d,%d), want (1,0)", col, idx)
	}
	if !eq(titles(b.Columns[0]), []string{"a0", "a2"}) {
		t.Fatalf("col A = %v", titles(b.Columns[0]))
	}
	if !eq(titles(b.Columns[1]), []string{"a1", "b0"}) {
		t.Fatalf("col B = %v", titles(b.Columns[1]))
	}
}

func TestMoveCardWithinColumnForward(t *testing.T) {
	// Simulates the drag-release shift: move a0 to end. Raw drop idx = 3,
	// same-column-forward shift makes it 2 after removal.
	b := sample()
	b.moveCard(0, 0, 0, 2)
	if !eq(titles(b.Columns[0]), []string{"a1", "a2", "a0"}) {
		t.Fatalf("col A = %v", titles(b.Columns[0]))
	}
}

func TestMoveCardClampsOutOfRange(t *testing.T) {
	b := sample()
	col, idx := b.moveCard(0, 99, 1, 0) // bad source index is a no-op
	if col != 0 || idx != 99 {
		t.Fatalf("expected no-op, got (%d,%d)", col, idx)
	}
	if len(b.Columns[1].Cards) != 1 {
		t.Fatalf("column B mutated: %v", titles(b.Columns[1]))
	}
}

func TestDropIndexRounding(t *testing.T) {
	m := NewModel(sample(), "x")
	m.width = 100
	top := cardsTopAbs()
	// single-line cards: slot = 1 row content + 1 spacer = 2 rows each
	if got := m.dropIndex(0, top); got != 0 {
		t.Fatalf("dropIndex at top = %d, want 0", got)
	}
	if got := m.dropIndex(0, top+4); got != 2 {
		t.Fatalf("dropIndex at 3rd slot = %d, want 2", got)
	}
	// far below clamps to len
	if got := m.dropIndex(0, top+1000); got != 3 {
		t.Fatalf("dropIndex far = %d, want 3", got)
	}
}

func TestDropIndexMultiLineCard(t *testing.T) {
	// A tall wrapped card must shift the geometry of the cards below it.
	long := "this is a fairly long card title that will wrap across several rows"
	b := Board{Columns: []Column{{Title: "A", Cards: []Card{
		{Title: long}, {Title: "short"},
	}}}}
	m := NewModel(b, "x")
	m.width = 40 // colWidth 40 → inner 38 → the long card wraps to >1 row
	tops, heights := m.columnCardLayout(0)
	if heights[0] < 2 {
		t.Fatalf("long card should wrap to multiple rows, got height %d", heights[0])
	}
	// the second card must begin below the full height of the first + gap
	if tops[1] != heights[0]+cardGap {
		t.Fatalf("card 1 top = %d, want %d", tops[1], heights[0]+cardGap)
	}
	// hit-testing a row inside the wrapped first card returns card 0
	if idx := m.hitCard(0, cardsTopAbs()+1); idx != 0 {
		t.Fatalf("hitCard inside wrapped card = %d, want 0", idx)
	}
}

func TestHitCardAndColumn(t *testing.T) {
	m := NewModel(sample(), "x")
	m.width = 100 // 2 cols → colWidth 49, colOuter 51
	if col := m.hitColumn(0); col != 0 {
		t.Fatalf("hitColumn(0)=%d want 0", col)
	}
	if col := m.hitColumn(m.colWidth() + 1); col != -1 {
		t.Fatalf("hitColumn in gap should be -1, got %d", col)
	}
	if col := m.hitColumn(m.colOuterWidth()); col != 1 {
		t.Fatalf("hitColumn(colOuter)=%d want 1", col)
	}
	top := cardsTopAbs()
	if idx := m.hitCard(0, top); idx != 0 {
		t.Fatalf("hitCard first =%d want 0", idx)
	}
	if idx := m.hitCard(0, top+1); idx != -1 {
		t.Fatalf("hitCard on spacer row should be -1, got %d", idx)
	}
	if idx := m.hitCard(0, top+2); idx != 1 {
		t.Fatalf("hitCard second =%d want 1", idx)
	}
}
