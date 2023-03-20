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
	assert.NotEqual(t, g.GameUID, "")
	assert.Equal(t, 10, g.Places[10].UserID)
}
