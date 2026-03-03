// Package game implement Balda game logic
package game

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	TurnDuration           = 60 * time.Second
	MaxConsecutiveTimeouts = 3
)

type Turn struct {
	PlayerID  string
	StartedAt time.Time
	timer     *time.Timer
}

type Notifier interface {
	NotifyTimeout(playerID string, consecutive int, willKick bool)
	NotifyKick(playerID string)
	NotifyTurnStart(playerID string)
}

type Game struct {
	mu       sync.Mutex
	state    GameState
	players  []*Player
	board    *LettersTable
	current  int
	turn     *Turn
	eventCh  chan TurnEvent
	done     chan struct{}
	notifier Notifier
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

// GapsBetweenLetters reports whether there are gaps between consecutive letters
// in the word path. Each consecutive pair must occupy board-adjacent cells
// (sharing an edge horizontally or vertically; Manhattan distance == 1).
// Returns true when the path is discontinuous (invalid word placement).
func GapsBetweenLetters(word []Letter) bool {
	if len(word) < 2 {
		return true
	}
	for i := 1; i < len(word); i++ {
		prev, curr := word[i-1], word[i]
		rowDiff := int(curr.RowID) - int(prev.RowID)
		colDiff := int(curr.ColID) - int(prev.ColID)
		if rowDiff < 0 {
			rowDiff = -rowDiff
		}
		if colDiff < 0 {
			colDiff = -colDiff
		}
		if rowDiff+colDiff != 1 {
			return true
		}
	}
	return false
}

func NewGame(players []*Player, n Notifier) (*Game, error) {
	board, err := NewLettersTable(Dict.RandomFiveLetterWord())
	if err != nil {
		return nil, err
	}
	return &Game{
		players:  players,
		eventCh:  make(chan TurnEvent, 4), // buffered: timer + auto-kick can queue simultaneously
		done:     make(chan struct{}),
		board:    board,
		notifier: n,
	}, nil
}

func (g *Game) Run(ctx context.Context) {
	g.startTurn()
	for {
		select {
		case <-ctx.Done():
			g.shutdown()
			return
		case ev, ok := <-g.eventCh:
			if !ok {
				return
			}
			g.mu.Lock()
			g.dispatch(ev)
			terminal := g.state == StateGameOver
			g.mu.Unlock()

			if terminal {
				g.shutdown()
				return
			}
		}
	}
}

func (g *Game) dispatch(ev TurnEvent) {
	t, ok := fsmTable[g.state][ev]
	if !ok {
		log.Printf("ignored event %v in state %v", ev, g.state)
		return
	}
	t.action(g)      // action runs first; may queue follow-up events
	g.state = t.next // then commit the state
}

// --- WaitingForMove actions ---

func (g *Game) onMoveAccepted() {
	g.currentPlayer().ConsecutiveTimeouts = 0
	g.cancelTimer()
	g.advanceTurn()
}

func (g *Game) onSkip() {
	g.currentPlayer().ConsecutiveTimeouts = 0
	g.cancelTimer()
	g.advanceTurn()
}

func (g *Game) onTurnTimeout() {
	p := g.currentPlayer()
	p.ConsecutiveTimeouts++

	willKick := p.ConsecutiveTimeouts >= MaxConsecutiveTimeouts

	// Phase 1: notify. Non-blocking — notifier must not call back into Game synchronously.
	g.notifier.NotifyTimeout(p.ID, p.ConsecutiveTimeouts, willKick)

	if willKick {
		// Auto-queue the kick; coordinator can also send it explicitly.
		// Sending into buffered channel from within the run loop is safe.
		g.eventCh <- EventKick
	}
	// If not kicking, we wait for an external EventAckTimeout before advancing.
}

// --- PlayerTimedOut actions ---

func (g *Game) onTimeoutAck() {
	// Player acknowledged the timeout notification; resume play.
	g.advanceTurn()
}

func (g *Game) onKick() {
	p := g.currentPlayer()
	p.Kicked = true
	g.notifier.NotifyKick(p.ID)
	// StateGameOver committed by dispatch; shutdown follows in Run.
}

func (g *Game) currentPlayer() *Player { return g.players[g.current] }

func (g *Game) advanceTurn() {
	g.current = (g.current + 1) % len(g.players)
	g.startTurn()
}

func (g *Game) startTurn() {
	p := g.currentPlayer()
	g.turn = &Turn{
		PlayerID:  p.ID,
		StartedAt: time.Now(),
		timer: time.AfterFunc(TurnDuration, func() {
			select {
			case g.eventCh <- EventTurnTimeout:
			case <-g.done:
			}
		}),
	}
	g.notifier.NotifyTurnStart(p.ID)
}

func (g *Game) cancelTimer() {
	if g.turn != nil && g.turn.timer != nil {
		g.turn.timer.Stop()
	}
}

func (g *Game) shutdown() {
	g.cancelTimer()
	// Safe to call multiple times: sync.Once or closed-channel idiom
	select {
	case <-g.done:
	default:
		close(g.done)
	}
}

func (g *Game) AddWordToCurrentPlayer(word string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.currentPlayer().Words = append(g.currentPlayer().Words, word)
}

func (g *Game) IsTakenWord(word string) bool {
	for _, player := range g.players {
		for _, pword := range player.Words {
			if pword == word {
				return true
			}
		}
	}
	return false
}
