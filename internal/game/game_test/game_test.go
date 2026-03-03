package game

import (
	"testing"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	g, err := game.NewGame(nil, nil)
	require.NoError(t, err)
	assert.True(t, g.СheckWordExistence(word))
}

func TestCheckWordExistence_WordNotInDictionary(t *testing.T) {
	g, err := game.NewGame(nil, nil)
	require.NoError(t, err)
	assert.False(t, g.СheckWordExistence("zzzzzznotaword"))
}

// ─── Game.AddWordToCurrentPlayer ────────────────────────────────────────────

func TestGame_AddWordToCurrentPlayer_AddsToFirstPlayer(t *testing.T) {
	players := []*game.Player{{ID: "p1"}, {ID: "p2"}}
	g, err := game.NewGame(players, nil)
	require.NoError(t, err)

	g.AddWordToCurrentPlayer("кот")

	assert.Equal(t, []string{"кот"}, players[0].Words)
	assert.Empty(t, players[1].Words)
}

func TestGame_AddWordToCurrentPlayer_MultipleWords(t *testing.T) {
	players := []*game.Player{{ID: "p1"}}
	g, err := game.NewGame(players, nil)
	require.NoError(t, err)

	g.AddWordToCurrentPlayer("кот")
	g.AddWordToCurrentPlayer("дом")

	assert.Equal(t, []string{"кот", "дом"}, players[0].Words)
}

// ─── Game.IsTakenWord ────────────────────────────────────────────────────────

func TestGame_IsTakenWord_NoPlayers(t *testing.T) {
	g, err := game.NewGame(nil, nil)
	require.NoError(t, err)
	assert.False(t, g.IsTakenWord("кот"))
}

func TestGame_IsTakenWord_WordFound(t *testing.T) {
	players := []*game.Player{{ID: "p1", Words: []string{"кот", "дом"}}}
	g, err := game.NewGame(players, nil)
	require.NoError(t, err)

	assert.True(t, g.IsTakenWord("кот"))
	assert.True(t, g.IsTakenWord("дом"))
}

func TestGame_IsTakenWord_WordNotPresent(t *testing.T) {
	players := []*game.Player{{ID: "p1", Words: []string{"кот"}}}
	g, err := game.NewGame(players, nil)
	require.NoError(t, err)

	assert.False(t, g.IsTakenWord("лес"))
}

func TestGame_IsTakenWord_AcrossMultiplePlayers(t *testing.T) {
	players := []*game.Player{
		{ID: "p1", Words: []string{"кот"}},
		{ID: "p2", Words: []string{"дом"}},
	}
	g, err := game.NewGame(players, nil)
	require.NoError(t, err)

	assert.True(t, g.IsTakenWord("кот"))
	assert.True(t, g.IsTakenWord("дом"))
	assert.False(t, g.IsTakenWord("лес"))
}
