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
