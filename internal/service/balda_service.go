package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/rustwizard/balda/internal/storage"
)

type Balda struct {
	lby      *lobby.Lobby
	mm       *matchmaking.Queue
	s        *storage.Balda
	notifier game.Notifier
}

func New(lby *lobby.Lobby, mm *matchmaking.Queue, s *storage.Balda, n game.Notifier) *Balda {
	return &Balda{lby: lby, mm: mm, s: s, notifier: n}
}

func (s *Balda) DB() *storage.Balda {
	return s.s
}

func (s *Balda) GameSummary(playerID string) *lobby.GameSummary {
	gs, err := s.lby.FindByPlayer(playerID)
	if err == lobby.ErrGameNotFound {
		return nil
	}
	return &gs
}

func (s *Balda) ListGames() []lobby.GameSummary {
	return s.lby.List()
}

func (s *Balda) Lobby() *lobby.Lobby {
	return s.lby
}

// CreateGame creates a new game in waiting status for the given user.
// It fetches the player UUID from the database by uid, then registers the game in the lobby.
func (s *Balda) CreateGame(ctx context.Context, uid int64) (*lobby.GameRecord, error) {
	var playerID uuid.UUID
	err := s.s.Pool().QueryRow(ctx,
		`SELECT player_id FROM player_state WHERE user_id = $1`, uid,
	).Scan(&playerID)
	if err != nil {
		return nil, fmt.Errorf("create game: fetch player: %w", err)
	}
	return s.lby.Create(playerID.String())
}

// JoinGame adds the user identified by uid to the waiting game with the given gameID.
// When quorum (2 players) is reached the game starts and the creator moves first.
func (s *Balda) JoinGame(ctx context.Context, uid int64, gameID string) (*lobby.GameRecord, error) {
	var playerID uuid.UUID
	err := s.s.Pool().QueryRow(ctx,
		`SELECT player_id FROM player_state WHERE user_id = $1`, uid,
	).Scan(&playerID)
	if err != nil {
		return nil, fmt.Errorf("join game: fetch player: %w", err)
	}
	return s.lby.Join(ctx, gameID, playerID.String(), s.notifier)
}

func (s *Balda) playerIDByUID(ctx context.Context, uid int64) (string, error) {
	var playerID uuid.UUID
	err := s.s.Pool().QueryRow(ctx,
		`SELECT player_id FROM player_state WHERE user_id = $1`, uid,
	).Scan(&playerID)
	if err != nil {
		return "", fmt.Errorf("fetch player: %w", err)
	}
	return playerID.String(), nil
}

func (s *Balda) isPlayerInGame(rec *lobby.GameRecord, playerID string) bool {
	for _, p := range rec.Players {
		if p.ID == playerID {
			return true
		}
	}
	return false
}

// SubmitMove validates and applies a player's move.
// Returns the game record and the player ID who made the move.
func (s *Balda) SubmitMove(ctx context.Context, uid int64, gameID string, newLetter game.Letter, wordPath []game.Letter) (*lobby.GameRecord, string, error) {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return nil, "", err
	}

	rec, err := s.lby.Get(gameID)
	if err != nil {
		return nil, "", err
	}

	if !s.isPlayerInGame(rec, playerID) {
		return nil, "", fmt.Errorf("player is not in this game")
	}

	// Resolve characters for the word path from the current board state.
	board := rec.Game.Board().AsStrings()
	for i := range wordPath {
		if wordPath[i].RowID == newLetter.RowID && wordPath[i].ColID == newLetter.ColID {
			wordPath[i].Char = newLetter.Char
		} else {
			wordPath[i].Char = board[wordPath[i].RowID][wordPath[i].ColID]
		}
	}

	if err := rec.Game.SubmitWord(playerID, &newLetter, wordPath); err != nil {
		return nil, "", err
	}

	return rec, playerID, nil
}

// ProposeEnd proposes to end the game. Only the current player may call this.
func (s *Balda) ProposeEnd(ctx context.Context, uid int64, gameID string) error {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	rec, err := s.lby.Get(gameID)
	if err != nil {
		return err
	}
	if !s.isPlayerInGame(rec, playerID) {
		return fmt.Errorf("player is not in this game")
	}
	return rec.Game.ProposeEnd(playerID)
}

// AcceptEnd accepts the opponent's end-game proposal.
func (s *Balda) AcceptEnd(ctx context.Context, uid int64, gameID string) error {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	rec, err := s.lby.Get(gameID)
	if err != nil {
		return err
	}
	if !s.isPlayerInGame(rec, playerID) {
		return fmt.Errorf("player is not in this game")
	}
	return rec.Game.AcceptEnd(playerID)
}

// RejectEnd rejects the opponent's end-game proposal.
func (s *Balda) RejectEnd(ctx context.Context, uid int64, gameID string) error {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	rec, err := s.lby.Get(gameID)
	if err != nil {
		return err
	}
	if !s.isPlayerInGame(rec, playerID) {
		return fmt.Errorf("player is not in this game")
	}
	return rec.Game.RejectEnd(playerID)
}

// SkipTurn ends the current turn without a move.
// Returns the game record and the player ID who skipped.
func (s *Balda) SkipTurn(ctx context.Context, uid int64, gameID string) (*lobby.GameRecord, string, error) {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return nil, "", err
	}

	rec, err := s.lby.Get(gameID)
	if err != nil {
		return nil, "", err
	}

	if !s.isPlayerInGame(rec, playerID) {
		return nil, "", fmt.Errorf("player is not in this game")
	}

	if err := rec.Game.Skip(playerID); err != nil {
		return nil, "", err
	}

	return rec, playerID, nil
}
