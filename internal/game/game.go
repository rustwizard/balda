// Package game implement Balda game logic
package game

import ulid "github.com/oklog/ulid/v2"

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
	PlacePlayerOne = 0
	PlacePlayerTwo = 1
)

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
	GameUID string
	Places  map[int]Place
	State   int      // GameStatePAUSED || GameStateSTARTED
	Delay   int      // seconds when game will start. if == 0 then unknown when game will be started
	Words   [][]rune // words on a table
	Turn    Turn     // current turn configuration
}

func NewGame(player *Player) *Game {
	game := &Game{
		GameUID: ulid.Make().String(),
		Places:  make(map[int]Place),
		State:   StatePAUSED,
		Delay:   0,
		Words:   [][]rune{},
	}

	game.Places[player.UserID] = Place{
		PlaceID:    PlacePlayerOne,
		PlaceState: PlaceStateIDLE,
		Player:     *player,
	}

	return game
}
