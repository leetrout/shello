package board

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func titles(c Column) []string {
	out := make([]string, len(c.Cards))
	for i, x := range c.Cards {
		out[i] = x.Title
	}
	return out
}

func sample() Board {
	return Board{Columns: []Column{
		{Title: "A", Cards: []Card{{"a0"}, {"a1"}, {"a2"}}},
		{Title: "B", Cards: []Card{{"b0"}}},
	}}
}

func TestMoveCard(t *testing.T) {
	tests := []struct {
		name                           string
		fromCol, fromIdx, toCol, toIdx int
		wantCol, wantIdx               int
		wantA, wantB                   []string
	}{
		{
			name:    "across columns to front",
			fromCol: 0, fromIdx: 1, toCol: 1, toIdx: 0,
			wantCol: 1, wantIdx: 0,
			wantA: []string{"a0", "a2"}, wantB: []string{"a1", "b0"},
		},
		{
			name:    "within column to end",
			fromCol: 0, fromIdx: 0, toCol: 0, toIdx: 2,
			wantCol: 0, wantIdx: 2,
			wantA: []string{"a1", "a2", "a0"}, wantB: []string{"b0"},
		},
		{
			name:    "clamps oversized toIdx",
			fromCol: 1, fromIdx: 0, toCol: 0, toIdx: 99,
			wantCol: 0, wantIdx: 3,
			wantA: []string{"a0", "a1", "a2", "b0"}, wantB: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := sample()
			gotCol, gotIdx := b.MoveCard(tt.fromCol, tt.fromIdx, tt.toCol, tt.toIdx)
			assert.Equal(t, tt.wantCol, gotCol, "resting column")
			assert.Equal(t, tt.wantIdx, gotIdx, "resting index")
			assert.Equal(t, tt.wantA, titles(b.Columns[0]), "column A")
			assert.Equal(t, tt.wantB, titles(b.Columns[1]), "column B")
		})
	}
}

func TestMoveCardOutOfRangeIsNoOp(t *testing.T) {
	b := sample()
	col, idx := b.MoveCard(0, 99, 1, 0) // bad source index
	assert.Equal(t, 0, col)
	assert.Equal(t, 99, idx)
	assert.Len(t, b.Columns[1].Cards, 1, "destination untouched")
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	b, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.NoError(t, err)
	assert.Equal(t, Default(), b)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "board.json")
	want := sample()
	require.NoError(t, want.Save(path), "save creates parent dirs")

	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
