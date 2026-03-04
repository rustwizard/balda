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
