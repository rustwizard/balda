package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/auth"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetPlayerStats implements baldaapi.Handler.
func (h *Handlers) GetPlayerStats(ctx context.Context) (baldaapi.GetPlayerStatsRes, error) {
	claims, ok := auth.FromContext(ctx)
	if !ok {
		return &baldaapi.GetPlayerStatsUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	stats, err := h.svc.GetPlayerStats(ctx, claims.PlayerID)
	if err != nil {
		slog.Error("get player stats", slog.Any("error", err))
		return &baldaapi.GetPlayerStatsInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to get player stats"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	return &baldaapi.PlayerStatsResponse{
		GamesPlayed:    baldaapi.NewOptInt64(stats.GamesPlayed),
		Wins:           baldaapi.NewOptInt64(stats.Wins),
		Losses:         baldaapi.NewOptInt64(stats.Losses),
		Draws:          baldaapi.NewOptInt64(stats.Draws),
		WinRate:        baldaapi.NewOptFloat64(stats.WinRate),
		AvgWordLength:  baldaapi.NewOptFloat64(stats.AvgWordLength),
		BestWord:       baldaapi.NewOptString(stats.BestWord),
		FavoriteLetter: baldaapi.NewOptString(stats.FavoriteLetter),
	}, nil
}
