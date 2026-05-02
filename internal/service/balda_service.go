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

// AuthUser verifies credentials and returns the user's identity.
func (s *Balda) AuthUser(ctx context.Context, email, password string) (storage.UserAuth, error) {
	return s.s.AuthUser(ctx, email, password)
}

// CreateUser registers a new user with their player profile in one transaction.
func (s *Balda) CreateUser(ctx context.Context, firstname, lastname, email, password, nickname string) (storage.UserCreated, error) {
	return s.s.CreateUser(ctx, firstname, lastname, email, password, nickname)
}

// GetPlayerState returns the profile fields for the given player UUID.
func (s *Balda) GetPlayerState(ctx context.Context, playerID uuid.UUID) (storage.PlayerState, error) {
	return s.s.GetPlayerState(ctx, playerID)
}

// CreateGame creates a new game in waiting status for the given user.
func (s *Balda) CreateGame(ctx context.Context, uid int64) (*lobby.GameRecord, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	return s.lby.Create(&game.Player{ID: p.PlayerID.String(), Exp: p.Exp})
}

// JoinGame adds the user identified by uid to the waiting game with the given gameID.
func (s *Balda) JoinGame(ctx context.Context, uid int64, gameID string) (*lobby.GameRecord, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("join game: %w", err)
	}
	return s.lby.Join(ctx, gameID, &game.Player{ID: p.PlayerID.String(), Exp: p.Exp}, s.notifier)
}

func (s *Balda) playerIDByUID(ctx context.Context, uid int64) (string, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return "", fmt.Errorf("fetch player: %w", err)
	}
	return p.PlayerID.String(), nil
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
