// Package gamecoord wires a running game to Centrifugo real-time events.
// One Coordinator is created per game; it implements game.Notifier and:
//   - publishes turn_change to the game channel on every turn start
//   - publishes game_state (full board snapshot) alongside turn_change
//   - auto-acknowledges non-kick timeouts so the turn advances automatically
//   - publishes game_over when a player is kicked
package gamecoord

import (
	"context"
	"log/slog"
	"time"

	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
)

// Coordinator implements game.Notifier and bridges game events to Centrifugo.
//
// nextReason and firstTurn are only written/read from the game's Run goroutine
// (NotifyTimeout and NotifyTurnStart are both called under g.mu from Run), so
// no additional synchronisation is needed.
type Coordinator struct {
	gameID     string
	g          *game.Game
	players    []*game.Player
	cf         *centrifugo.Client
	nextReason string // reason for the upcoming turn_change; "" means "move"
	firstTurn  bool   // true until the first NotifyTurnStart
}

// New creates a Coordinator for the given game. Call SetGame immediately after
// constructing the *game.Game so notifier callbacks can read game state.
func New(gameID string, players []*game.Player, cf *centrifugo.Client) *Coordinator {
	return &Coordinator{
		gameID:    gameID,
		players:   players,
		cf:        cf,
		firstTurn: true,
	}
}

// SetGame stores the game reference. Must be called before game.Run starts.
func (c *Coordinator) SetGame(g *game.Game) {
	c.g = g
}

// NotifyTurnStart is called by the game FSM at the beginning of each turn.
// It publishes a turn_change event (general turn change notification) and a
// game_state snapshot so both clients know whose turn it is and see the board.
func (c *Coordinator) NotifyTurnStart(playerID string) {
	var reason string
	if c.firstTurn {
		reason = "game_start"
		c.firstTurn = false
	} else if c.nextReason != "" {
		reason = c.nextReason
		c.nextReason = ""
	} else {
		reason = "move"
	}
	go c.publishTurnChange(playerID, reason)
	go c.publishGameState()
}

// NotifyTimeout is called when the current player's timer expires.
// For non-kick timeouts it records the reason and immediately acknowledges so
// the turn advances. For kick-threshold timeouts the game auto-queues EventKick;
// NotifyKick follows.
func (c *Coordinator) NotifyTimeout(_ string, _ int, willKick bool) {
	if !willKick {
		c.nextReason = "timeout"
		go c.g.AckTimeout()
	}
}

// NotifyKick is called when a player is kicked (3 consecutive timeouts).
// It publishes game_over and the lobby will remove the game when Run exits.
func (c *Coordinator) NotifyKick(kickedPlayerID string) {
	go c.publishGameOver(kickedPlayerID)
}

func (c *Coordinator) publishTurnChange(playerID, reason string) {
	ev := centrifugo.EvTurnChange{
		Type:           "turn_change",
		GameID:         c.gameID,
		CurrentTurnUID: playerID,
		Reason:         reason,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.cf.Publish(ctx, centrifugo.ChannelGame(c.gameID), ev); err != nil {
		slog.Error("gamecoord: publish turn_change", slog.String("gameID", c.gameID), slog.Any("error", err))
	}
}

func (c *Coordinator) publishGameState() {
	board := c.g.Board().AsStrings()
	scores := c.g.PlayerScores()
	currentTurn := c.g.CurrentPlayerID()
	moveNum := c.g.MoveNumber()

	players := make([]centrifugo.PlayerScore, len(scores))
	for i, s := range scores {
		players[i] = centrifugo.PlayerScore{UID: s.UID, Score: s.Score, WordsCount: s.WordsCount}
	}

	ev := centrifugo.EvGameState{
		Type:           "game_state",
		GameID:         c.gameID,
		Board:          board,
		CurrentTurnUID: currentTurn,
		Players:        players,
		Status:         "in_progress",
		MoveNumber:     moveNum,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.cf.Publish(ctx, centrifugo.ChannelGame(c.gameID), ev); err != nil {
		slog.Error("gamecoord: publish game_state", slog.String("gameID", c.gameID), slog.Any("error", err))
	}
}

func (c *Coordinator) publishGameOver(kickedPlayerID string) {
	scores := c.g.PlayerScores()

	winnerUID := ""
	players := make([]centrifugo.PlayerScore, len(scores))
	for i, s := range scores {
		players[i] = centrifugo.PlayerScore{UID: s.UID, Score: s.Score, WordsCount: s.WordsCount}
		if s.UID != kickedPlayerID {
			winnerUID = s.UID
		}
	}

	ev := centrifugo.EvGameOver{
		Type:      "game_over",
		GameID:    c.gameID,
		WinnerUID: winnerUID,
		Players:   players,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.cf.Publish(ctx, centrifugo.ChannelGame(c.gameID), ev); err != nil {
		slog.Error("gamecoord: publish game_over", slog.String("gameID", c.gameID), slog.Any("error", err))
	}
}
