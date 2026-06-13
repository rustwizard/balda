package bot

import (
	"context"
	"log/slog"
	"math/rand"
	"time"

	"github.com/rustwizard/balda/internal/game"
)

const (
	botMoveTimeout = 5 * time.Second
	thinkingMin    = 500 * time.Millisecond
	thinkingMax    = 1500 * time.Millisecond
)

// Notifier implements game.Notifier and triggers bot moves on the bot's turn.
type Notifier struct {
	engine      *Engine
	g           *game.Game
	botPlayerID string
}

// NewNotifier creates a notifier that drives the given bot player.
func NewNotifier(engine *Engine, botPlayerID string) *Notifier {
	return &Notifier{
		engine:      engine,
		botPlayerID: botPlayerID,
	}
}

// SetGame stores the game reference. Must be called before game.Run starts.
func (n *Notifier) SetGame(g *game.Game) {
	n.g = g
}

// NotifyTurnStart is called by the game FSM when a new turn begins.
// If it is the bot's turn, it computes and submits a move in a goroutine.
func (n *Notifier) NotifyTurnStart(playerID string) {
	if playerID != n.botPlayerID {
		return
	}

	go func() {
		thinkingTime := thinkingMin + time.Duration(rand.Int63n(int64(thinkingMax-thinkingMin)))
		// Ensure the previous event has time to be published before the bot acts.
		select {
		case <-n.g.Done():
			return
		case <-time.After(thinkingTime):
		}

		if n.gameIsDone() {
			return
		}
		board := n.g.BoardSnapshot()
		usedWords := n.g.UsedWords()

		ctx, cancel := context.WithTimeout(context.Background(), botMoveTimeout)
		defer cancel()

		type moveResult struct {
			letter *game.Letter
			path   []game.Letter
			err    error
		}
		done := make(chan moveResult, 1)
		go func() {
			letter, path, err := n.engine.MakeMove(board, usedWords)
			done <- moveResult{letter: letter, path: path, err: err}
		}()

		var letter *game.Letter
		var path []game.Letter
		var err error
		select {
		case <-ctx.Done():
			err = ErrNoMoveFound
		case r := <-done:
			letter, path, err = r.letter, r.path, r.err
		}

		if n.gameIsDone() {
			return
		}

		if err != nil {
			slog.Warn("bot: no move found, skipping", slog.String("playerID", n.botPlayerID), slog.Any("error", err))
			if skipErr := n.g.Skip(n.botPlayerID); skipErr != nil {
				slog.Error("bot: skip turn", slog.String("playerID", n.botPlayerID), slog.Any("error", skipErr))
			}
			return
		}

		if submitErr := n.g.SubmitWord(n.botPlayerID, letter, path); submitErr != nil {
			slog.Error("bot: submit word", slog.String("playerID", n.botPlayerID), slog.Any("error", submitErr))
			if skipErr := n.g.Skip(n.botPlayerID); skipErr != nil {
				slog.Error("bot: skip turn after failed submit", slog.String("playerID", n.botPlayerID), slog.Any("error", skipErr))
			}
		}
	}()
}

func (n *Notifier) gameIsDone() bool {
	select {
	case <-n.g.Done():
		return true
	default:
		return false
	}
}

// NotifyTimeout is a no-op for bots.
func (n *Notifier) NotifyTimeout(_ string, _ int, _ bool) {}

// NotifySkip is a no-op for bots.
func (n *Notifier) NotifySkip(_ string, _ int, _ bool) {}

// NotifyKick is a no-op for bots.
func (n *Notifier) NotifyKick(_ string) {}

// NotifyGameFinished is a no-op for bots.
func (n *Notifier) NotifyGameFinished() {}

// NotifyEndProposed is called when the opponent proposes to end the game.
// The bot auto-accepts in a goroutine because this notification is dispatched
// while the game mutex is held.
func (n *Notifier) NotifyEndProposed(proposerID string) {
	if proposerID == n.botPlayerID {
		return
	}
	go func() {
		if err := n.g.AcceptEnd(n.botPlayerID); err != nil {
			slog.Warn("bot: auto-accept end proposal failed", slog.String("playerID", n.botPlayerID), slog.Any("error", err))
		}
	}()
}

// NotifyEndAccepted is a no-op for bots.
func (n *Notifier) NotifyEndAccepted() {}

// NotifyEndRejected is a no-op for bots.
func (n *Notifier) NotifyEndRejected(_ time.Duration) {}
