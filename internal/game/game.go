// Package game implement Balda game logic
package game

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

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

func (g *Game) Start(ctx context.Context, w string) error {
	g.StartTS = time.Now()

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

	g.State = StateSTARTED

	g.mainLoop(ctx)

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

func (g *Game) mainLoop(ctx context.Context) {
	g.fsmState = StateWaitTurn
	log.Debug().Msg("game: main loop: start")
Loop:
	for {
		select {
		case <-ctx.Done():
			log.Error().Err(ctx.Err()).Msg("game: main loop")
		default:
		}

		select {
		case <-ctx.Done():
			log.Error().Err(ctx.Err()).Msg("game: main loop")
		default:
			switch g.fsmState {
			case StateWaitTurn:
				g.waitTurn()
			case StateNextTurn:
				g.waitTurn()
			case StatePlaceKick:
				log.Debug().Msgf("game: main loop: placeID: %d was kicked due to inactivity", g.Turn.PlaceID)
				break Loop
			}
		}
	}
	log.Debug().Msg("game: main loop: end")
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
			log.Debug().Msgf("game: main loop: placeID: %d: wait turn", g.Turn.PlaceID)
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

func (g *Game) placeIDByUserID(userID int) (int, error) {
	var placeID int
	place, ok := g.Places[userID]
	if !ok {
		return placeID, fmt.Errorf("there is no such user in the game")
	}
	placeID = place.PlaceID

	return placeID, nil
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

	log.Debug().Msgf("game: main loop: next turn: placeID: %d", g.Turn.PlaceID)
}

func (g *Game) getFSMState() int {
	return g.fsmState
}

func (g *Game) setFSMState(state int) {
	g.fsmState = state
}

func (g *Game) GameTurn(userID int, l *Letter, word []Letter) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	placeID, err := g.placeIDByUserID(userID)
	if err != nil {
		return fmt.Errorf("game turn: check place id: %v", err)
	}

	if err = g.checkGame(placeID); err != nil {
		return fmt.Errorf("game turn: check game: %v", err)
	}

	if len(word) <= 2 {
		return fmt.Errorf("game: word is not selected")
	}

	g.Places[userID].PlaceState = PlaceStateSEND

	// TODO: make a method to check there is no gaps between letters in the word

	w := MakeWord(word)
	if g.Places.IsTakenWord(w) {
		return fmt.Errorf("game: no turn: word already taken")
	}

	if !g.СheckWordExistence(w) {
		return fmt.Errorf("game: no turn: there is no such word in the dictionary")
	}

	if err := g.Words.PutLetterOnTable(l); err != nil {
		return fmt.Errorf("game: no turn: %w", err)
	}
	log.Debug().Msgf("game: main loop: placeID: %d put the word on the table", placeID)
	g.Places[userID].Words = append(g.Places[userID].Words, w)
	g.nextTurn()

	// TODO: send events to clients

	return nil
}

func (g *Game) СheckWordExistence(word string) bool {
	if _, ok := Dict.Definition[word]; !ok {
		return false
	}
	return true
}

func MakeWord(word []Letter) string {
	var w string
	for _, v := range word {
		w += v.Char
	}
	return w
}

// TODO: impl
func GapsBetweenLetters(word []Letter) bool {
	return true
}

// TODO: impl
func (g *Game) GameTurnSkip(userID int) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	placeID, err := g.placeIDByUserID(userID)
	if err != nil {
		return fmt.Errorf("game turn skip: check place id: %v", err)
	}

	if err = g.checkGame(placeID); err != nil {
		return fmt.Errorf("game turn skip: check game: %v", err)
	}

	g.Places[userID].PlaceState = PlaceStateSKIP
	log.Debug().Msgf("game: main loop: skip turn: placeID: %d skipped the turn", placeID)
	g.nextTurn()

	return nil
}

func (g *Game) checkGame(placeID int) error {
	if g.State != StateSTARTED {
		return fmt.Errorf("game not started")
	}

	if g.Turn.PlaceID != placeID {
		return fmt.Errorf("not user's turn")
	}

	return nil
}
