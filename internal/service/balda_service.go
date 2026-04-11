package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/rustwizard/balda/internal/storage"
)

type Balda struct {
	lby *lobby.Lobby
	mm  *matchmaking.Queue
	s   *storage.Balda
}

func New(lby *lobby.Lobby, mm *matchmaking.Queue, s *storage.Balda) *Balda {
	return &Balda{lby: lby, mm: mm, s: s}
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
