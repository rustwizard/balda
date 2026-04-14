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

func TestMakeWord_NormalizesYoToE(t *testing.T) {
	letters := []game.Letter{
		{Char: "е"},
		{Char: "л"},
		{Char: "к"},
		{Char: "а"},
	}
	assert.Equal(t, "елка", game.MakeWord(letters))

	lettersYo := []game.Letter{
		{Char: "ё"},
		{Char: "л"},
		{Char: "к"},
		{Char: "а"},
	}
	assert.Equal(t, "елка", game.MakeWord(lettersYo))
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

func TestCheckWordExistence_YoMatchesE(t *testing.T) {
	g, err := game.NewGame(nil, nil)
	require.NoError(t, err)

	// Runtime dictionary insertion (not from JSON) still uses normalized keys
	// because addTestWord normalizes, but here we test the lookup directly.
	game.Dict.Definition["елка"] = "test"
	t.Cleanup(func() { delete(game.Dict.Definition, "елка") })

	assert.True(t, g.СheckWordExistence("ёлка"), "ё should match е in dictionary")
	assert.True(t, g.СheckWordExistence("елка"), "е should match е in dictionary")
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

func TestGame_IsTakenWord_YoMatchesE(t *testing.T) {
	players := []*game.Player{
		{ID: "p1", Words: []string{"елка"}},
	}
	g, err := game.NewGame(players, nil)
	require.NoError(t, err)

	assert.True(t, g.IsTakenWord("ёлка"), "stored word with е should match query with ё")
	assert.True(t, g.IsTakenWord("елка"), "stored word with е should match query with е")
}

func TestGapsBetweenLetters(t *testing.T) {
	tests := []struct {
		name string
		word []game.Letter
		want bool
	}{
		{
			name: "empty word",
			word: []game.Letter{},
			want: true,
		},
		{
			name: "single letter",
			word: []game.Letter{{RowID: 2, ColID: 2, Char: "а"}},
			want: true,
		},
		{
			name: "two adjacent letters horizontally",
			word: []game.Letter{
				{RowID: 2, ColID: 1, Char: "б"},
				{RowID: 2, ColID: 2, Char: "а"},
			},
			want: false,
		},
		{
			name: "two adjacent letters vertically",
			word: []game.Letter{
				{RowID: 1, ColID: 2, Char: "б"},
				{RowID: 2, ColID: 2, Char: "а"},
			},
			want: false,
		},
		{
			name: "horizontal word no gaps",
			word: []game.Letter{
				{RowID: 2, ColID: 0, Char: "с"},
				{RowID: 2, ColID: 1, Char: "л"},
				{RowID: 2, ColID: 2, Char: "о"},
				{RowID: 2, ColID: 3, Char: "в"},
				{RowID: 2, ColID: 4, Char: "о"},
			},
			want: false,
		},
		{
			name: "vertical word no gaps",
			word: []game.Letter{
				{RowID: 0, ColID: 2, Char: "с"},
				{RowID: 1, ColID: 2, Char: "т"},
				{RowID: 2, ColID: 2, Char: "о"},
				{RowID: 3, ColID: 2, Char: "л"},
			},
			want: false,
		},
		{
			name: "L-shaped path no gaps",
			word: []game.Letter{
				{RowID: 0, ColID: 0, Char: "с"},
				{RowID: 1, ColID: 0, Char: "т"},
				{RowID: 2, ColID: 0, Char: "о"},
				{RowID: 2, ColID: 1, Char: "л"},
			},
			want: false,
		},
		{
			name: "gap of two cells",
			word: []game.Letter{
				{RowID: 2, ColID: 0, Char: "с"},
				{RowID: 2, ColID: 2, Char: "о"}, // skips ColID 1
			},
			want: true,
		},
		{
			name: "diagonal jump — not adjacent",
			word: []game.Letter{
				{RowID: 1, ColID: 1, Char: "а"},
				{RowID: 2, ColID: 2, Char: "б"}, // diagonal: rowDiff+colDiff == 2
			},
			want: true,
		},
		{
			name: "same cell repeated — distance 0",
			word: []game.Letter{
				{RowID: 2, ColID: 2, Char: "а"},
				{RowID: 2, ColID: 2, Char: "б"},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := game.GapsBetweenLetters(tc.word)
			if got != tc.want {
				t.Errorf("GapsBetweenLetters(%v) = %v, want %v", tc.word, got, tc.want)
			}
		})
	}
}
