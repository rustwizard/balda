package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// SkipGame implements baldaapi.Handler.
func (h *Handlers) SkipGame(ctx context.Context, params baldaapi.SkipGameParams) (baldaapi.SkipGameRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.SkipGameUnauthorized{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("skip_game: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return &baldaapi.SkipGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	rec, moverID, err := h.svc.SkipTurn(ctx, uid, params.ID.String())
	if err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.SkipGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, game.ErrNotYourTurn), errors.Is(err, game.ErrWrongState):
			return &baldaapi.SkipGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString(err.Error()),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		default:
			slog.Error("skip_game: skip turn", slog.Any("error", err))
			return &baldaapi.SkipGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to skip turn"),
				Type:    baldaapi.NewOptString("InternalError"),
			}, nil
		}
	}

	nextTurnUID := nextPlayerID(moverID, rec.Game.PlayerScores())
	gameState := buildGameState(rec, nextTurnUID)
	if err := h.cf.Publish(ctx, centrifugo.ChannelGame(rec.ID), gameState); err != nil {
		slog.Error("skip_game: publish game state", slog.Any("error", err))
	}

	return &baldaapi.SkipGameNoContent{}, nil
}
