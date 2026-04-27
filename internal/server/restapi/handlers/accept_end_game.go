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

// AcceptEndGame implements baldaapi.Handler.
func (h *Handlers) AcceptEndGame(ctx context.Context, params baldaapi.AcceptEndGameParams) (baldaapi.AcceptEndGameRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.AcceptEndGameUnauthorized{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("accept_end_game: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return &baldaapi.AcceptEndGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	if err := h.svc.AcceptEnd(ctx, uid, params.ID.String()); err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.AcceptEndGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, game.ErrWrongState), errors.Is(err, game.ErrNotOpponent):
			return &baldaapi.AcceptEndGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString(err.Error()),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		default:
			slog.Error("accept_end_game: accept end", slog.Any("error", err))
			return &baldaapi.AcceptEndGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to accept end"),
				Type:    baldaapi.NewOptString("InternalError"),
			}, nil
		}
	}

	return &baldaapi.AcceptEndGameNoContent{}, nil
}
