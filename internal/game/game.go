// Package game implement Balda game logic
package game

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"sync"
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

type Turn struct {
	PlaceID   int
	TimeTotal time.Duration // time in seconds for the player's turn
	TimeRest  time.Duration // time in seconds before turn is end by timeout
}

type Game struct {
	mu       sync.RWMutex
	UID      string
	Places   Places
	State    int           // StatePAUSED || StateSTARTED
	fsmState int           // StateWaitTurn
	Delay    int           // seconds when game will start. if == 0 then unknown when game will be started
	Words    *LettersTable // words on a table
	Turn     Turn          // current turn configuration
	StartTS  time.Time     // timestamp when game was started
}

func NewGame(player *Player) *Game {
	game := &Game{
		mu:     sync.RWMutex{},
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
				g.setFSMState(StatePlaceKick)
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
	if g.getFSMState() == StateWaitTurn {
		g.setFSMState(StateNextTurn)
	} else {
		g.setFSMState(StateWaitTurn)
	}

	if g.Turn.PlaceID == PlacePlayerOne {
		g.Turn.PlaceID = PlacePlayerTwo
	} else {
		g.Turn.PlaceID = PlacePlayerOne
	}
}

func (g *Game) getFSMState() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.fsmState
}

func (g *Game) setFSMState(state int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.fsmState = state
}

func (g *Game) GameTurn(userID int, l *Letter, word []Letter) error {
	place, ok := g.Places[userID]
	if !ok {
		return fmt.Errorf("game: there is no such user in the game")
	}

	if g.State != StateSTARTED {
		return fmt.Errorf("game: not started")
	}

	if g.Turn.PlaceID != place.PlaceID {
		return fmt.Errorf("game: not user's turn")
	}

	if len(word) <= 2 {
		return fmt.Errorf("game: word is not selected")
	}

	g.Places[userID].PlaceState = PlaceStateSEND
	// TODO: send event to client to change place state

	if g.Places.IsTakenWord(word) {
		return fmt.Errorf("game: no turn: word already taken")
	}

	if !g.СheckWordExistence(word) {
		return fmt.Errorf("game: no turn: there is no such word in the dictionary")
	}

	if err := g.Words.PutLetterOnTable(l); err != nil {
		return fmt.Errorf("game: no turn: %w", err)
	}

	// TODO: add word to the player and change turn to another player

	return nil
}

func (g *Game) СheckWordExistence(word []Letter) bool {
	w := makeWord(word)
	if _, ok := Dict.Definition[w]; !ok {
		return false
	}
	return true
}

func makeWord(word []Letter) string {
	var w string
	for _, v := range word {
		w += v.Char
	}
	return w
}
