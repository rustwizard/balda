package service

import (
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

func (s *Balda) Lobby() *lobby.Lobby {
	return s.lby
}
