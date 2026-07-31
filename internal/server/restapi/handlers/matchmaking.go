package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/matchmaking"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/service"
)

// MatchmakingJoin implements baldaapi.Handler.
func (h *Handlers) MatchmakingJoin(ctx context.Context) (baldaapi.MatchmakingJoinRes, error) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return &baldaapi.MatchmakingJoinUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	err := h.svc.QuickMatchJoin(ctx, uid)
	switch {
	case err == nil:
		return &baldaapi.MatchmakingJoinResponse{
			Status: baldaapi.NewOptMatchmakingJoinResponseStatus(baldaapi.MatchmakingJoinResponseStatusQueued),
		}, nil
	case errors.Is(err, service.ErrPlayerInGame), errors.Is(err, matchmaking.ErrAlreadyQueued):
		return &baldaapi.MatchmakingJoinConflict{
			Status:  baldaapi.NewOptInt(http.StatusConflict),
			Message: baldaapi.NewOptString("player is already in a game or in the queue"),
			Type:    baldaapi.NewOptString("Conflict"),
		}, nil
	default:
		slog.Error("matchmaking join", slog.Any("error", err))
		return &baldaapi.MatchmakingJoinInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to join matchmaking"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}
}

// MatchmakingLeave implements baldaapi.Handler.
func (h *Handlers) MatchmakingLeave(ctx context.Context) (baldaapi.MatchmakingLeaveRes, error) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return &baldaapi.MatchmakingLeaveUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	if err := h.svc.QuickMatchLeave(ctx, uid); err != nil {
		slog.Error("matchmaking leave", slog.Any("error", err))
		return &baldaapi.MatchmakingLeaveInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to leave matchmaking"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}
	return &baldaapi.MatchmakingLeaveNoContent{}, nil
}
