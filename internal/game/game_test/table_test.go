package game

import (
	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewLettersTable(t *testing.T) {
	lt, err := game.NewLettersTable("ффффф")
	assert.NoError(t, err)
	letters := lt.Table[2]
	assert.Equal(t, 5, len(letters))
	assert.Equal(t, "ф", letters[0].Char)
	assert.Equal(t, uint8(2), letters[0].RowID)
}

func TestInitialWord(t *testing.T) {
	lt, err := game.NewLettersTable("ффффф")
	assert.NoError(t, err)
	letters := lt.Table[2]
	assert.Equal(t, 5, len(letters))
	assert.Equal(t, "ф", letters[0].Char)
	assert.Equal(t, uint8(2), letters[0].RowID)
	assert.Equal(t, "ффффф", lt.InitialWord())
}

func TestLettersTable_IsFull(t *testing.T) {
	lt, err := game.NewLettersTable("zzzzz")
	require.NoError(t, err)
	assert.False(t, lt.IsFull())

	// Fill every cell.
	for r := range lt.Table {
		for c := range lt.Table[r] {
			if lt.Table[r][c] == nil {
				lt.Table[r][c] = &game.Letter{RowID: uint8(r), ColID: uint8(c), Char: "a"}
			}
		}
	}
	assert.True(t, lt.IsFull())
}

func TestPutLetterOnTable(t *testing.T) {
	lt, err := game.NewLettersTable("zzzzz")
	assert.NoError(t, err)

	err = lt.PutLetterOnTable(&game.Letter{
		RowID: 3,
		ColID: 0,
		Char:  "t",
	})
	assert.NoError(t, err)

	err = lt.PutLetterOnTable(&game.Letter{
		RowID: 0,
		ColID: 0,
		Char:  "t",
	})
	assert.Error(t, err)
	assert.Equal(t, err, game.ErrWrongLetterPlace)

	err = lt.PutLetterOnTable(&game.Letter{
		RowID: 4,
		ColID: 1,
		Char:  "t",
	})
	assert.Error(t, err)
	assert.Equal(t, err, game.ErrWrongLetterPlace)

	err = lt.PutLetterOnTable(&game.Letter{
		RowID: 3,
		ColID: 0,
		Char:  "t",
	})
	assert.Error(t, err)
	assert.Equal(t, err, game.ErrLetterPlaceTaken)

	err = lt.PutLetterOnTable(&game.Letter{
		RowID: 5,
		ColID: 5,
		Char:  "t",
	})
	assert.Error(t, err)
	assert.Equal(t, err, game.ErrWrongLetterPlace)

	err = lt.PutLetterOnTable(&game.Letter{
		RowID: 1,
		ColID: 0,
		Char:  "t",
	})
	assert.NoError(t, err)

}

func TestPutLetterOnTable_HorizontalAdjacency(t *testing.T) {
	lt, err := game.NewLettersTable("zzzzz")
	require.NoError(t, err)

	// Fill row 3, col 0 and 1
	require.NoError(t, lt.PutLetterOnTable(&game.Letter{RowID: 3, ColID: 0, Char: "a"}))
	require.NoError(t, lt.PutLetterOnTable(&game.Letter{RowID: 3, ColID: 1, Char: "b"}))

	// Row 4, col 1 is OK because row 3, col 1 is above it (vertical adjacency).
	assert.NoError(t, lt.PutLetterOnTable(&game.Letter{RowID: 4, ColID: 1, Char: "c"}))

	// Row 4, col 2 was previously forbidden because row 3, col 2 is empty.
	// Now it is allowed because row 4, col 1 is a horizontal neighbour.
	assert.NoError(t, lt.PutLetterOnTable(&game.Letter{RowID: 4, ColID: 2, Char: "d"}))

	// Row 4, col 4 has no neighbours — still forbidden.
	assert.ErrorIs(t, lt.PutLetterOnTable(&game.Letter{RowID: 4, ColID: 4, Char: "e"}), game.ErrWrongLetterPlace)
}
