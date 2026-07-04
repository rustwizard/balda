package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/auth"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetPlayerAchievements implements baldaapi.Handler.
func (h *Handlers) GetPlayerAchievements(ctx context.Context) (baldaapi.GetPlayerAchievementsRes, error) {
	claims, ok := auth.FromContext(ctx)
	if !ok {
		return &baldaapi.GetPlayerAchievementsUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	list, err := h.svc.GetPlayerAchievements(ctx, claims.PlayerID)
	if err != nil {
		slog.Error("get player achievements", slog.Any("error", err))
		return &baldaapi.GetPlayerAchievementsInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to get achievements"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	out := make([]baldaapi.Achievement, len(list))
	for i, a := range list {
		out[i] = baldaapi.Achievement{
			ID:          baldaapi.NewOptAchievementID(baldaapi.AchievementID(a.ID)),
			Name:        baldaapi.NewOptString(a.Name),
			Description: baldaapi.NewOptString(a.Description),
			Unlocked:    baldaapi.NewOptBool(a.Unlocked),
		}
	}

	return &baldaapi.PlayerAchievementsResponse{
		Achievements: out,
	}, nil
}
