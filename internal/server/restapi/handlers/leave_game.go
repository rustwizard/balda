package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// LeaveGame implements baldaapi.Handler.
func (h *Handlers) LeaveGame(ctx context.Context, params baldaapi.LeaveGameParams) (baldaapi.LeaveGameRes, error) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return &baldaapi.LeaveGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	err := h.svc.LeaveGame(ctx, uid, params.ID.String())
	switch {
	case err == nil:
		h.publishLobbyUpdate(ctx)
		return &baldaapi.LeaveGameNoContent{}, nil
	case errors.Is(err, lobby.ErrGameNotFound):
		return &baldaapi.LeaveGameNotFound{
			Status:  baldaapi.NewOptInt(http.StatusNotFound),
			Message: baldaapi.NewOptString("game not found"),
			Type:    baldaapi.NewOptString("NotFound"),
		}, nil
	case errors.Is(err, lobby.ErrNotParticipant):
		return &baldaapi.LeaveGameForbidden{
			Status:  baldaapi.NewOptInt(http.StatusForbidden),
			Message: baldaapi.NewOptString("not a participant of this game"),
			Type:    baldaapi.NewOptString("Forbidden"),
		}, nil
	case errors.Is(err, lobby.ErrGameNotWaiting):
		return &baldaapi.LeaveGameConflict{
			Status:  baldaapi.NewOptInt(http.StatusConflict),
			Message: baldaapi.NewOptString("game is not in waiting status"),
			Type:    baldaapi.NewOptString("Conflict"),
		}, nil
	default:
		slog.Error("leave_game: leave", slog.Any("error", err))
		return nil, err
	}
}
