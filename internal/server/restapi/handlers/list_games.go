package handlers

import (
	"context"

	"github.com/google/uuid"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// ListGames implements baldaapi.Handler.
func (h *Handlers) ListGames(ctx context.Context, _ baldaapi.ListGamesParams) (baldaapi.ListGamesRes, error) {
	summaries := h.svc.ListGames()

	games := make([]baldaapi.GameSummary, len(summaries))
	for i, s := range summaries {
		gameID, err := uuid.Parse(s.ID)
		if err != nil {
			continue
		}

		playerIDs := make([]uuid.UUID, 0, len(s.Players))
		lobbyPlayers := make([]baldaapi.LobbyPlayer, 0, len(s.Players))
		for _, p := range s.Players {
			pid, err := uuid.Parse(p.ID)
			if err != nil {
				continue
			}
			playerIDs = append(playerIDs, pid)
			lobbyPlayers = append(lobbyPlayers, baldaapi.LobbyPlayer{
				UID: baldaapi.NewOptUUID(pid),
				Exp: baldaapi.NewOptInt64(int64(p.Exp)),
			})
		}

		games[i] = baldaapi.GameSummary{
			ID:        baldaapi.NewOptUUID(gameID),
			PlayerIds: playerIDs,
			Players:   lobbyPlayers,
			Status:    baldaapi.NewOptGameStatus(baldaapi.GameStatus(s.Status)),
			StartedAt: baldaapi.NewOptInt64(s.StartedAt.UnixMilli()),
		}
	}

	return &baldaapi.ListGamesResponse{Games: games}, nil
}
