package game

import (
	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"testing"
	"unicode/utf8"
)

func TestNewDictionary(t *testing.T) {
	dict, err := game.NewDictionary()
	assert.NoError(t, err)
	assert.NotNil(t, dict)
	assert.Equal(t, 5, utf8.RuneCountInString(dict.FiveLetters[0]))
}
