package game

import (
	"testing"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
)

// ─── MakeWord ───────────────────────────────────────────────────────────────

func TestMakeWord_Empty(t *testing.T) {
	assert.Equal(t, "", game.MakeWord(nil))
}

func TestMakeWord_SingleLetter(t *testing.T) {
	letters := []game.Letter{{Char: "а"}}
	assert.Equal(t, "а", game.MakeWord(letters))
}

func TestMakeWord_MultipleLetters(t *testing.T) {
	letters := []game.Letter{
		{Char: "к"},
		{Char: "о"},
		{Char: "т"},
	}
	assert.Equal(t, "кот", game.MakeWord(letters))
}

// ─── GapsBetweenLetters ─────────────────────────────────────────────────────

func TestGapsBetweenLetters_AlwaysTrue(t *testing.T) {
	assert.True(t, game.GapsBetweenLetters(nil))
	assert.True(t, game.GapsBetweenLetters([]game.Letter{{Char: "а"}}))
}

// ─── CheckWordExistence ─────────────────────────────────────────────────────

func TestCheckWordExistence_WordInDictionary(t *testing.T) {
	// Dict is populated via init(); grab any known 5-letter word.
	word := game.Dict.RandomFiveLetterWord()
	g := game.NewGame(nil, nil)
	assert.True(t, g.СheckWordExistence(word))
}

func TestCheckWordExistence_WordNotInDictionary(t *testing.T) {
	g := game.NewGame(nil, nil)
	assert.False(t, g.СheckWordExistence("zzzzzznotaword"))
}

// ─── Places.IsTakenWord ─────────────────────────────────────────────────────

func TestPlaces_IsTakenWord_Empty(t *testing.T) {
	places := game.Places{}
	assert.False(t, places.IsTakenWord("кот"))
}

func TestPlaces_IsTakenWord_WordExists(t *testing.T) {
	places := game.Places{
		1: &game.Place{Player: game.Player{Words: []string{"кот", "дом"}}},
	}
	assert.True(t, places.IsTakenWord("кот"))
	assert.True(t, places.IsTakenWord("дом"))
}

func TestPlaces_IsTakenWord_WordNotPresent(t *testing.T) {
	places := game.Places{
		1: &game.Place{Player: game.Player{Words: []string{"кот"}}},
	}
	assert.False(t, places.IsTakenWord("лес"))
}

func TestPlaces_IsTakenWord_MultiplePlayersFirstHasWord(t *testing.T) {
	places := game.Places{
		1: &game.Place{Player: game.Player{Words: []string{"кот"}}},
		2: &game.Place{Player: game.Player{Words: []string{"дом"}}},
	}
	assert.True(t, places.IsTakenWord("кот"))
	assert.True(t, places.IsTakenWord("дом"))
	assert.False(t, places.IsTakenWord("лес"))
}
