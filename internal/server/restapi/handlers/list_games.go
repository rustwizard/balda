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

		playerIDs := make([]uuid.UUID, 0, len(s.PlayerIDs))
		for _, pid := range s.PlayerIDs {
			uid, err := uuid.Parse(pid)
			if err != nil {
				continue
			}
			playerIDs = append(playerIDs, uid)
		}

		games[i] = baldaapi.GameSummary{
			ID:        baldaapi.NewOptUUID(gameID),
			PlayerIds: playerIDs,
			StartedAt: baldaapi.NewOptInt64(s.StartedAt.UnixMilli()),
		}
	}

	return &baldaapi.ListGamesResponse{Games: games}, nil
}
