package game

import (
	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsTakenWord(t *testing.T) {
	places := make(game.Places)
	places[10] = &game.Place{
		PlaceID:    game.PlacePlayerOne,
		PlaceState: game.PlaceStateIDLE,
		Player: game.Player{
			UserID:            10,
			Exp:               10,
			Score:             2,
			Words:             []string{"zzz", "ttt"},
			TimeoutTurnsCount: 0,
		},
	}
	places[11] = &game.Place{
		PlaceID:    game.PlacePlayerTwo,
		PlaceState: game.PlaceStateSEND,
		Player: game.Player{
			UserID:            11,
			Exp:               11,
			Score:             3,
			Words:             []string{"mmm", "qqq", "asdf"},
			TimeoutTurnsCount: 0,
		},
	}
	word := []game.Letter{{
		RowID: 0,
		ColID: 0,
		Char:  "m",
	}, {
		RowID: 1,
		ColID: 0,
		Char:  "m",
	}, {
		RowID: 2,
		ColID: 0,
		Char:  "m",
	}}

	assert.True(t, places.IsTakenWord(game.MakeWord(word)))

	word = []game.Letter{{
		RowID: 0,
		ColID: 0,
		Char:  "w",
	}, {
		RowID: 1,
		ColID: 0,
		Char:  "w",
	}, {
		RowID: 2,
		ColID: 0,
		Char:  "w",
	}}

	assert.False(t, places.IsTakenWord(game.MakeWord(word)))
}
