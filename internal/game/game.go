// Package game implement Balda game logic
package game

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	PlaceStateIDLE = 0 //
	PlaceStateSEND = 1 // send word to server.
	PlaceStateSKIP = 2 // skip turn
)

const (
	StatePAUSED    = 0
	StateSTARTED   = 1
	StateWaitTurn  = 2
	StateNextTurn  = 3
	StatePlaceKick = 4
)

const (
	PlacePlayerOne = 1
	PlacePlayerTwo = 2
)

const MaxPlayers = 2

const timeTotal = 60 * time.Second // total time in seconds for player turn

const maxTimeoutTurns = 3

type Player struct {
	UserID            int
	Exp               int
	Score             int
	Words             []string
	TimeoutTurnsCount int
}

type Place struct {
	PlaceID    int
	PlaceState int
	Player
}

type Turn struct {
	PlaceID   int
	TimeTotal time.Duration // time in seconds for the player's turn
	TimeRest  time.Duration // time in seconds before turn is end by timeout
}

type Game struct {
	UID      string
	Places   map[int]*Place
	State    int           // StatePAUSED || StateSTARTED
	fsmState int           // StateWaitTurn
	Delay    int           // seconds when game will start. if == 0 then unknown when game will be started
	Words    *LettersTable // words on a table
	Turn     Turn          // current turn configuration
	StartTS  time.Time     // timestamp when game was started
}

func NewGame(player *Player) *Game {
	game := &Game{
		UID:    ulid.Make().String(),
		Places: make(map[int]*Place),
		State:  StatePAUSED,
		Delay:  0,
		// Words:  init when game started,
	}

	game.Places[player.UserID] = &Place{
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

	g.Places[player.UserID] = &Place{
		PlaceID:    PlacePlayerTwo,
		PlaceState: PlaceStateIDLE,
		Player:     *player,
	}

	return nil
}

func (g *Game) Start(w string) error {
	g.StartTS = time.Now()
	g.State = StateSTARTED

	lt, err := NewLettersTable(w)
	if err != nil {
		return fmt.Errorf("game: start: init word")
	}
	g.Words = lt
	g.Turn = Turn{
		PlaceID:   g.firstTurnPlaceID(),
		TimeTotal: timeTotal,
		TimeRest:  timeTotal,
	}

	g.mainLoop()

	return nil
}

func (g *Game) firstTurnPlaceID() int {
	for _, v := range g.Places {
		if v.PlaceID == PlacePlayerOne {
			return v.PlaceID
		}
	}
	return 0
}

func (g *Game) mainLoop() {
	g.fsmState = StateWaitTurn
	log.Debug().Msg("game: main loop started")
Loop:
	for {
		switch g.fsmState {
		case StateWaitTurn:
			g.waitTurn()
		case StateNextTurn:
			g.waitTurn()
		case StatePlaceKick:
			log.Debug().Msg("kick the place")
			break Loop
		}
	}
	log.Debug().Msg("game: main loop ended")
}

func (g *Game) waitTurn() {
	log.Debug().Msgf("game: main loop: waiting when player with placeID: %d do the turn", g.Turn.PlaceID)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
Loop:
	for {
		select {
		case <-timer.C:
			log.Debug().Msgf("game: main loop: timeout: player with placeID: %d", g.Turn.PlaceID)
			userID := g.userIDByPlaceID(g.Turn.PlaceID)
			g.Places[userID].TimeoutTurnsCount++
			if g.Places[userID].TimeoutTurnsCount >= maxTimeoutTurns {
				g.fsmState = StatePlaceKick
			} else {
				g.nextTurn()
			}
			break Loop
		default:
			log.Debug().Msgf("placeID: %d: wait for turn", g.Turn.PlaceID)
			time.Sleep(1 * time.Second)
		}
	}
}

func (g *Game) userIDByPlaceID(placeID int) int {
	for _, v := range g.Places {
		if v.PlaceID == placeID {
			return v.UserID
		}
	}
	return 0
}

func (g *Game) nextTurn() {
	if g.fsmState == StateWaitTurn {
		g.fsmState = StateNextTurn
	} else {
		g.fsmState = StateWaitTurn
	}

	if g.Turn.PlaceID == PlacePlayerOne {
		g.Turn.PlaceID = PlacePlayerTwo
	} else {
		g.Turn.PlaceID = PlacePlayerOne
	}
}
