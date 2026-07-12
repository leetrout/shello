package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/leetrout/shello/internal/board"
)

// ---- helpers ----

func newModel(t *testing.T, b board.Board, w, h int) Model {
	t.Helper()
	m := New(b, filepath.Join(t.TempDir(), "board.json"))
	nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return nm.(Model)
}

func send(m Model, msg tea.Msg) Model {
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func sample() board.Board {
	return board.Board{Columns: []board.Column{
		{Title: "A", Cards: []board.Card{{Title: "a0"}, {Title: "a1"}, {Title: "a2"}}},
		{Title: "B", Cards: []board.Card{{Title: "b0"}}},
	}}
}

func colTitles(m Model) []string {
	out := make([]string, len(m.board.Columns))
	for i, c := range m.board.Columns {
		out[i] = c.Title
	}
	return out
}

// ---- wrapping ----

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"fits", "hello", 10, []string{"hello"}},
		{"word wrap", "hello world foo", 11, []string{"hello world", "foo"}},
		{"hard break long word", "abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"empty", "", 8, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, wrapText(tt.in, tt.width))
		})
	}
}

// ---- hit-testing ----

func TestHitColumn(t *testing.T) {
	m := newModel(t, sample(), 100, 40) // 2 cols → colWidth 49, colOuter 51
	assert.Equal(t, 0, m.hitColumn(0))
	assert.Equal(t, -1, m.hitColumn(m.colWidth()+1), "gap between columns")
	assert.Equal(t, 1, m.hitColumn(m.colOuterWidth()))
	assert.Equal(t, -1, m.hitColumn(10_000), "past the last column")
}

func TestHitCardAndDropIndex(t *testing.T) {
	m := newModel(t, sample(), 100, 40)
	top := cardsTopAbs()

	// single-line cards: 1 content row + 1 spacer = 2 rows each
	assert.Equal(t, 0, m.hitCard(0, top), "first card")
	assert.Equal(t, -1, m.hitCard(0, top+1), "spacer row")
	assert.Equal(t, 1, m.hitCard(0, top+2), "second card")

	assert.Equal(t, 0, m.dropIndex(0, top), "insert before first")
	assert.Equal(t, 2, m.dropIndex(0, top+4), "insert before third")
	assert.Equal(t, 3, m.dropIndex(0, top+1000), "clamp to end")
}

func TestMultiLineCardShiftsGeometry(t *testing.T) {
	long := "this is a fairly long card title that will wrap across several rows"
	b := board.Board{Columns: []board.Column{{Title: "A", Cards: []board.Card{
		{Title: long}, {Title: "short"},
	}}}}
	m := newModel(t, b, 40, 40) // narrow → the long card wraps

	tops, heights := m.columnCardLayout(0)
	require.GreaterOrEqual(t, heights[0], 2, "long card wraps to multiple rows")
	assert.Equal(t, heights[0]+cardGap, tops[1], "second card starts below the first")
	assert.Equal(t, 0, m.hitCard(0, cardsTopAbs()+1), "row inside wrapped card resolves to card 0")
}

// ---- scrolling ----

func manyCards(n int) board.Board {
	cards := make([]board.Card, n)
	for i := range cards {
		cards[i] = board.Card{Title: "card-" + itoa(i)}
	}
	return board.Board{Columns: []board.Column{{Title: "Big", Cards: cards}}}
}

func TestScrollAutoRevealsCursorAndWheel(t *testing.T) {
	m := newModel(t, manyCards(40), 60, 20)
	require.Positive(t, m.maxScroll(0), "column overflows")

	// G jumps to the last card and must auto-scroll it into view
	m = send(m, key("G"))
	assert.Positive(t, m.scrollFor(0), "auto-scrolled to reveal last card")

	// wheel back to the top
	for range 40 {
		m = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, X: 1, Y: 5})
	}
	assert.Equal(t, 0, m.scrollFor(0), "wheel-up reaches the top")

	// hit-testing accounts for the scroll offset
	m.setScroll(0, 6) // 6 rows = 3 single-line cards
	assert.Equal(t, 3, m.hitCard(0, cardsTopAbs()), "scrolled hit-test at viewport top")
}

// ---- keyboard navigation ----

func TestCursorNavigation(t *testing.T) {
	m := newModel(t, board.Default(), 100, 30)
	m = send(m, key("l"))
	assert.Equal(t, 1, m.curCol, "l moves right")
	m = send(m, key("h"))
	assert.Equal(t, 0, m.curCol, "h moves left")
	m = send(m, key("j"))
	assert.Equal(t, 1, m.curCard, "j moves down")
}

// ---- grab & move card ----

func TestGrabMoveCard(t *testing.T) {
	m := newModel(t, board.Default(), 100, 30)
	before := m.board.Columns[0].Cards[0].Title

	m = send(m, key(" ")) // grab
	require.True(t, m.grabbed, "space grabs the card")

	m = send(m, key("l")) // carry into next column
	assert.Equal(t, 1, m.curCol, "card carried to column 1")
	assert.Equal(t, before, m.board.Columns[1].Cards[m.curCard].Title, "content carried")
	assert.Len(t, m.board.Columns[0].Cards, 2, "removed from source column")

	m = send(m, key(" ")) // drop
	assert.False(t, m.grabbed, "space drops the card")
}

// ---- move columns ----

func TestMoveColumn(t *testing.T) {
	m := newModel(t, board.Default(), 100, 30)

	m = send(m, key(">"))
	assert.Equal(t, []string{"Doing", "Todo", "Done"}, colTitles(m), "> moves column right")
	assert.Equal(t, 1, m.curCol, "selection follows the moved column")

	m = send(m, key("<"))
	assert.Equal(t, []string{"Todo", "Doing", "Done"}, colTitles(m), "< moves column left")

	before := colTitles(m)
	m = send(m, key("<")) // at the left edge: no-op
	assert.Equal(t, before, colTitles(m), "moving past the edge is a no-op")
}

// ---- delete column confirmation ----

func TestDeleteColumnConfirmation(t *testing.T) {
	m := newModel(t, board.Default(), 100, 30)

	m = send(m, key("D"))
	require.Equal(t, modeConfirm, m.mode, "D asks for confirmation")
	assert.Len(t, m.board.Columns, 3, "nothing deleted yet")

	m = send(m, key("n"))
	assert.Equal(t, modeNormal, m.mode, "n cancels")
	assert.Len(t, m.board.Columns, 3, "still nothing deleted")

	m = send(m, key("D"))
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"Doing", "Done"}, colTitles(m), "enter confirms the delete")
}

// ---- rendering ----

func TestViewReflectsCursor(t *testing.T) {
	lipgloss.SetColorProfile(lipgloss.ColorProfile()) // ensure a color profile is initialised
	lipgloss.SetColorProfile(0)                       // 0 = TrueColor, so selection colors render
	m := newModel(t, board.Default(), 100, 30)

	base := m.View()

	shifted := m
	shifted.curCol = 1
	assert.NotEqual(t, base, shifted.View(), "moving the column cursor changes the frame")

	down := m
	down.curCard = 1
	assert.NotEqual(t, base, down.View(), "moving the card cursor changes the frame")

	grabbed := m
	grabbed.grabbed = true
	assert.NotEqual(t, base, grabbed.View(), "grabbing a card changes the frame")
}
