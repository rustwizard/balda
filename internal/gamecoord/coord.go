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
	"github.com/rustwizard/balda/internal/storage"
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
	onGameOver func(storage.GameResult)
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

// SetOnGameOver registers a callback invoked (in a goroutine) on every game-over path.
func (c *Coordinator) SetOnGameOver(fn func(storage.GameResult)) {
	c.onGameOver = fn
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

// NotifySkip is called each time the current player skips a turn.
// When willEnd is true, EventKick has already been queued; game_over follows.
func (c *Coordinator) NotifySkip(playerID string, consecutive int, willEnd bool) {
	skipsLeft := game.MaxConsecutiveSkips - consecutive
	go c.publishSkipWarn(playerID, consecutive, skipsLeft)
	if !willEnd {
		c.nextReason = "skip"
	}
}

// NotifyKick is called when a player is kicked (3 consecutive timeouts).
// It publishes game_over and the lobby will remove the game when Run exits.
func (c *Coordinator) NotifyKick(kickedPlayerID string) {
	go c.publishGameOver(kickedPlayerID)
}

// NotifyBoardFull is called when the last empty cell on the board is filled.
// The winner is the player with the higher score; equal scores mean a draw.
func (c *Coordinator) NotifyBoardFull() {
	go c.publishBoardFullGameOver()
}

// NotifyEndProposed is called when the current player proposes to end the game.
func (c *Coordinator) NotifyEndProposed(proposerID string) {
	go c.publishEndProposal(proposerID)
}

func (c *Coordinator) publishEndProposal(proposerID string) {
	ev := centrifugo.EvEndProposal{
		Type:        "end_proposal",
		GameID:      c.gameID,
		ProposerUID: proposerID,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.cf.Publish(ctx, centrifugo.ChannelGame(c.gameID), ev); err != nil {
		slog.Error("gamecoord: publish end_proposal", slog.String("gameID", c.gameID), slog.Any("error", err))
	}
}

// NotifyEndAccepted is called when the opponent accepts the end proposal.
func (c *Coordinator) NotifyEndAccepted() {
	go c.publishEndProposalResult(true, 0)
}

// NotifyEndRejected is called when the opponent rejects the end proposal.
func (c *Coordinator) NotifyEndRejected(remainingTurn time.Duration) {
	go c.publishEndProposalResult(false, remainingTurn.Milliseconds())
}

// findWinnerByScore returns the UID of the player with the highest score.
// If excludeUID is non-empty, that player is ignored.
// If multiple players share the highest score, it returns "" (draw).
func findWinnerByScore(scores []game.PlayerState, excludeUID string) string {
	var maxScore int
	var candidates []string
	for _, s := range scores {
		if s.UID == excludeUID {
			continue
		}
		if s.Score > maxScore {
			maxScore = s.Score
			candidates = []string{s.UID}
		} else if s.Score == maxScore {
			candidates = append(candidates, s.UID)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

func (c *Coordinator) publishEndProposalResult(accepted bool, remainingMs int64) {
	ev := centrifugo.EvEndProposalResult{
		Type:        "end_proposal_result",
		GameID:      c.gameID,
		Accepted:    accepted,
		RemainingMs: remainingMs,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.cf.Publish(ctx, centrifugo.ChannelGame(c.gameID), ev); err != nil {
		slog.Error("gamecoord: publish end_proposal_result", slog.String("gameID", c.gameID), slog.Any("error", err))
	}

	if accepted {
		scores := c.g.PlayerScores()
		winnerUID := findWinnerByScore(scores, "")
		c.dispatchGameResult(winnerUID, storage.FinishReasonAcceptEnd, scores)
	}
}

func (c *Coordinator) dispatchGameResult(winnerUID string, reason storage.FinishReason, scores []game.PlayerState) {
	if c.onGameOver == nil {
		return
	}
	isDraw := winnerUID == ""
	players := make([]storage.PlayerResult, len(scores))
	for i, s := range scores {
		isWinner := s.UID == winnerUID
		players[i] = storage.PlayerResult{
			PlayerID:   s.UID,
			Score:      s.Score,
			WordsCount: s.WordsCount,
			ExpGained:  storage.ExpGained(s.Score, isWinner, isDraw),
		}
	}
	result := storage.GameResult{
		GameID:       c.gameID,
		WinnerID:     winnerUID,
		FinishReason: reason,
		FinishedAt:   time.Now(),
		Players:      players,
	}
	c.onGameOver(result)
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

	players := make([]centrifugo.PlayerState, len(scores))
	for i, s := range scores {
		players[i] = centrifugo.PlayerState{UID: s.UID, Exp: s.Exp, Score: s.Score, WordsCount: s.WordsCount, Words: s.Words}
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

func (c *Coordinator) publishSkipWarn(playerID string, skipsUsed, skipsLeft int) {
	ev := centrifugo.EvSkipWarn{
		Type:      "skip_warn",
		GameID:    c.gameID,
		PlayerUID: playerID,
		SkipsUsed: skipsUsed,
		SkipsLeft: skipsLeft,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.cf.Publish(ctx, centrifugo.ChannelGame(c.gameID), ev); err != nil {
		slog.Error("gamecoord: publish skip_warn", slog.String("gameID", c.gameID), slog.Any("error", err))
	}
}

func (c *Coordinator) publishBoardFullGameOver() {
	scores := c.g.PlayerScores()
	winnerUID := findWinnerByScore(scores, "")

	// Persist the result before notifying clients so the database is the
	// source of truth by the time they see game_over.
	c.dispatchGameResult(winnerUID, storage.FinishReasonBoardFull, scores)

	players := make([]centrifugo.PlayerState, len(scores))
	for i, s := range scores {
		players[i] = centrifugo.PlayerState{UID: s.UID, Exp: s.Exp, Score: s.Score, WordsCount: s.WordsCount, Words: s.Words}
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
		slog.Error("gamecoord: publish game_over (board full)", slog.String("gameID", c.gameID), slog.Any("error", err))
	}
}

func (c *Coordinator) publishGameOver(kickedPlayerID string) {
	scores := c.g.PlayerScores()
	winnerUID := findWinnerByScore(scores, kickedPlayerID)

	// Persist the result before notifying clients so the database is the
	// source of truth by the time they see game_over.
	c.dispatchGameResult(winnerUID, storage.FinishReasonKick, scores)

	players := make([]centrifugo.PlayerState, len(scores))
	for i, s := range scores {
		players[i] = centrifugo.PlayerState{UID: s.UID, Exp: s.Exp, Score: s.Score, WordsCount: s.WordsCount, Words: s.Words}
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
