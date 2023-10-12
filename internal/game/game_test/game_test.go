package game_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	game "github.com/rustwizard/balda/internal/game"
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
	ctx := context.Background()
	err := g.Start(ctx, "kawabanga")
	assert.Error(t, err, "should return an error because word len is greater then 5")

	err = g.Start(ctx, "apple")
	assert.NoError(t, err)
	assert.Equal(t, game.StateSTARTED, g.State)
	assert.Equal(t, "apple", g.Words.InitialWord())

}

func TestGameCheckWordExistence(t *testing.T) {
	g := initGame(t)
	word := "лето"
	assert.True(t, g.СheckWordExistence(word))

	word = "цуквыфва"
	assert.False(t, g.СheckWordExistence(word))
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

func TestGameTurn(t *testing.T) {
	g := initGame(t)
	ctx := context.Background()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	var err error
	go func(wg *sync.WaitGroup) {
		err = g.Start(ctx, "аббат")
		assert.NoError(t, err)
		assert.Equal(t, game.StateSTARTED, g.State)
		assert.Equal(t, "аббат", g.Words.InitialWord())
		wg.Done()
	}(wg)

	err = g.GameTurn(10, &game.Letter{
		RowID: 0,
		ColID: 0,
		Char:  "",
	}, []game.Letter{{Char: "л"}, {Char: "е"}, {Char: "т"}, {Char: "о"}})
	assert.Error(t, err) // game not started

	// wait 1 sec and do the turn
	time.Sleep(1 * time.Second)
	err = g.GameTurn(10, &game.Letter{
		RowID: 0,
		ColID: 0,
		Char:  "",
	}, []game.Letter{{Char: "л"}, {Char: "е"}, {Char: "т"}, {Char: "о"}})
	assert.Error(t, err) // wrong place for letter

	err = g.GameTurn(11, &game.Letter{
		RowID: 0,
		ColID: 0,
		Char:  "",
	}, []game.Letter{{Char: "л"}, {Char: "е"}, {Char: "т"}, {Char: "о"}})
	assert.Error(t, err) // not users turn

	err = g.GameTurn(14, &game.Letter{
		RowID: 0,
		ColID: 0,
		Char:  "",
	}, []game.Letter{{Char: "л"}, {Char: "е"}, {Char: "т"}, {Char: "о"}})
	assert.Error(t, err) // there is no such user in the game

	err = g.GameTurn(10, &game.Letter{
		RowID: 0,
		ColID: 0,
		Char:  "",
	}, []game.Letter{{Char: "л"}, {Char: "е"}})
	assert.Error(t, err) // word is not selected

	err = g.GameTurn(10, &game.Letter{
		RowID: 0,
		ColID: 1,
		Char:  "р",
	}, []game.Letter{{Char: "р"}, {Char: "р"}, {Char: "p"}})
	assert.Error(t, err) // there is no such word in the dictionary

	err = g.GameTurn(10, &game.Letter{
		RowID: 1,
		ColID: 0,
		Char:  "р",
	}, []game.Letter{{Char: "р"}, {Char: "а"}, {Char: "б"}})
	assert.NoError(t, err)

	assert.True(t, g.Places.IsTakenWord("раб"))

	err = g.GameTurn(11, &game.Letter{
		RowID: 1,
		ColID: 3,
		Char:  "р",
	}, []game.Letter{{Char: "р"}, {Char: "а"}, {Char: "б"}})
	assert.Error(t, err) // word already taken

	wg.Wait()

}

func TestGameSkipTurn(t *testing.T) {
	g := initGame(t)
	ctx := context.Background()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	var err error
	go func(wg *sync.WaitGroup) {
		err = g.Start(ctx, "аббат")
		assert.NoError(t, err)
		assert.Equal(t, game.StateSTARTED, g.State)
		assert.Equal(t, "аббат", g.Words.InitialWord())
		wg.Done()
	}(wg)

	// wait 1 sec and do the turn
	time.Sleep(1 * time.Second)

	err = g.GameTurn(10, &game.Letter{
		RowID: 1,
		ColID: 0,
		Char:  "р",
	}, []game.Letter{{Char: "р"}, {Char: "а"}, {Char: "б"}})
	assert.NoError(t, err)

	// wait 1 sec and do the turn
	time.Sleep(2 * time.Second)
	err = g.GameTurnSkip(11)
	assert.NoError(t, err)

	wg.Wait()
}
