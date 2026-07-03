package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/leaderboard"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetLeaderboard implements baldaapi.Handler.
func (h *Handlers) GetLeaderboard(ctx context.Context, params baldaapi.GetLeaderboardParams) (baldaapi.GetLeaderboardRes, error) {
	sortBy := "rating"
	if params.Sort.IsSet() {
		sortBy = string(params.Sort.Value)
	}

	limit := 100
	if params.Limit.IsSet() {
		limit = params.Limit.Value
	}

	entries, generatedAt, err := h.svc.GetLeaderboard(ctx, leaderboard.Request{
		Period: string(params.Period),
		Sort:   sortBy,
		Limit:  limit,
	})
	if err != nil {
		slog.Error("get leaderboard", slog.Any("error", err))
		return &baldaapi.GetLeaderboardInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to get leaderboard"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	players := make([]baldaapi.LeaderboardEntry, len(entries))
	for i, e := range entries {
		players[i] = baldaapi.LeaderboardEntry{
			Rank:     baldaapi.NewOptInt(i + 1),
			UID:      baldaapi.NewOptUUID(e.PlayerID),
			Nickname: baldaapi.NewOptString(e.Nickname),
			Rating:   baldaapi.NewOptInt64(e.Rating),
			Exp:      baldaapi.NewOptInt64(e.Exp),
		}
	}

	return &baldaapi.LeaderboardResponse{
		Period:      baldaapi.NewOptLeaderboardResponsePeriod(baldaapi.LeaderboardResponsePeriod(params.Period)),
		Sort:        baldaapi.NewOptLeaderboardResponseSort(baldaapi.LeaderboardResponseSort(sortBy)),
		GeneratedAt: baldaapi.NewOptInt64(generatedAt.UnixMilli()),
		Players:     players,
	}, nil
}
