// Package game implement Balda game logic
package game

import (
	"fmt"
	ulid "github.com/oklog/ulid/v2"
)

const (
	PlaceStateIDLE = 0 //
	PlaceStateSEND = 1 // send word to server.
	PlaceStateSKIP = 2 // skip turn
)

const (
	StatePAUSED  = 0
	StateSTARTED = 1
)

const (
	PlacePlayerOne = 1
	PlacePlayerTwo = 2
)

const MaxPlayers = 2

type Player struct {
	UserID int
	Exp    int
	Score  int
	Words  []string
}

type Place struct {
	PlaceID    int
	PlaceState int
	Player
}

type Turn struct {
	PlaceID   int
	TimeTotal int // time in seconds for the player's turn
	TimeRest  int // time in seconds before turn is end by timeout
}

type Game struct {
	UID    string
	Places map[int]Place
	State  int      // StatePAUSED || StateSTARTED
	Delay  int      // seconds when game will start. if == 0 then unknown when game will be started
	Words  [][]rune // words on a table
	Turn   Turn     // current turn configuration
}

func NewGame(player *Player) *Game {
	game := &Game{
		UID:    ulid.Make().String(),
		Places: make(map[int]Place),
		State:  StatePAUSED,
		Delay:  0,
		Words:  [][]rune{},
	}

	game.Places[player.UserID] = Place{
		PlaceID:    PlacePlayerOne,
		PlaceState: PlaceStateIDLE,
		Player:     *player,
	}

	return game
}

func (g *Game) Join(GUID string, player *Player) error {
	if g.UID != GUID {
		return fmt.Errorf("game: join: wrong guid")
	}

	if g.State == StateSTARTED {
		return fmt.Errorf("game: join: game already started")
	}

	if len(g.Places) >= MaxPlayers {
		return fmt.Errorf("game: join: max players reached")
	}

	g.Places[player.UserID] = Place{
		PlaceID:    PlacePlayerTwo,
		PlaceState: PlaceStateIDLE,
		Player:     *player,
	}

	return nil
}
