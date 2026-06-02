package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// RejectEndGame implements baldaapi.Handler.
func (h *Handlers) RejectEndGame(ctx context.Context, params baldaapi.RejectEndGameParams) (baldaapi.RejectEndGameRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.RejectEndGameUnauthorized{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("reject_end_game: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return &baldaapi.RejectEndGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	if err := h.svc.RejectEnd(ctx, uid, params.ID.String()); err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.RejectEndGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, game.ErrWrongState), errors.Is(err, game.ErrNotOpponent):
			return &baldaapi.RejectEndGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString(err.Error()),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		default:
			slog.Error("reject_end_game: reject end", slog.Any("error", err))
			return &baldaapi.RejectEndGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to reject end"),
				Type:    baldaapi.NewOptString("InternalError"),
			}, nil
		}
	}

	return &baldaapi.RejectEndGameNoContent{}, nil
}
