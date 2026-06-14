package game

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDictionary_ExcludesProperNouns asserts the dictionary holds only common
// nouns. Proper nouns (leading-uppercase keys like "Диана", "Аллах") must not be
// loaded: Balda forbids them, and a capitalized word picked as the initial board
// word would also break lowercase dictionary lookups for words reusing its cells.
func TestNewDictionary_ExcludesProperNouns(t *testing.T) {
	dict, err := game.NewDictionary()
	require.NoError(t, err)

	var properInDefinition []string
	for w := range dict.Definition {
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsUpper(r) {
			properInDefinition = append(properInDefinition, w)
		}
	}
	assert.Empty(t, properInDefinition, "Definition must contain only common nouns (no leading-uppercase keys)")

	for _, w := range dict.FiveLetters {
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsUpper(r) {
			t.Errorf("FiveLetters must not contain a proper noun: %q", w)
		}
	}
}

func TestNewDictionary(t *testing.T) {
	dict, err := game.NewDictionary()
	assert.NoError(t, err)
	assert.NotNil(t, dict)
	assert.Equal(t, 5, utf8.RuneCountInString(dict.FiveLetters[0]))
}

func TestDictionary_RandomFiveLetterWord(t *testing.T) {
	dict, err := game.NewDictionary()
	assert.NoError(t, err)
	assert.NotNil(t, dict)
	assert.Equal(t, 5, utf8.RuneCountInString(dict.RandomFiveLetterWord()))
}
