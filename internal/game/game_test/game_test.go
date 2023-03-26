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
