package game_test

import (
	"testing"

	game "github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
)

func TestGameCreate(t *testing.T) {
	player := &game.Player{
		UserID: 10,
		Exp:    12,
		Score:  11,
		Words:  nil,
	}

	g := game.NewGame(player)
	assert.NotNil(t, g)
	assert.NotEqual(t, g.UID, "")
	assert.Equal(t, 10, g.Places[10].UserID)
}

func TestGameJoin(t *testing.T) {
	player := &game.Player{
		UserID: 10,
		Exp:    12,
		Score:  11,
		Words:  nil,
	}

	g := game.NewGame(player)
	assert.NotNil(t, g)
	assert.NotEqual(t, g.UID, "")
	assert.Equal(t, 10, g.Places[10].UserID)

	err := g.Join(g.UID, &game.Player{
		UserID: 11,
		Exp:    11,
		Score:  11,
		Words:  nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 11, g.Places[11].UserID)

	err = g.Join(g.UID, &game.Player{
		UserID: 12,
		Exp:    12,
		Score:  12,
		Words:  nil,
	})
	assert.Error(t, err)
}

func TestGameStart(t *testing.T) {
	g := initGame(t)
	err := g.Start("kawabanga")
	assert.Error(t, err, "should return an error because word len is greater then 5")

	err = g.Start("apple")
	assert.NoError(t, err)
	assert.Equal(t, game.StateSTARTED, g.State)
}

func initGame(t *testing.T) *game.Game {
	player := &game.Player{
		UserID: 10,
		Exp:    10,
		Score:  10,
		Words:  nil,
	}

	g := game.NewGame(player)
	assert.NotNil(t, g)
	assert.NotEqual(t, g.UID, "")
	assert.Equal(t, 10, g.Places[10].UserID)

	err := g.Join(g.UID, &game.Player{
		UserID: 11,
		Exp:    11,
		Score:  11,
		Words:  nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 11, g.Places[11].UserID)

	return g
}
