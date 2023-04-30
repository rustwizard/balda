package game

import (
	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewLettersTable(t *testing.T) {
	lt, err := game.NewLettersTable("ффффф")
	assert.NoError(t, err)
	letters := lt.Table[3]
	assert.Equal(t, 5, len(letters))
	assert.Equal(t, "ф", letters[0].Char)
	assert.Equal(t, uint8(3), letters[0].RowID)
}

func TestInitialWord(t *testing.T) {
	lt, err := game.NewLettersTable("ффффф")
	assert.NoError(t, err)
	letters := lt.Table[3]
	assert.Equal(t, 5, len(letters))
	assert.Equal(t, "ф", letters[0].Char)
	assert.Equal(t, uint8(3), letters[0].RowID)
	assert.Equal(t, "ффффф", lt.InitialWord())
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
