package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/service"
)

// ProposeEndGame implements baldaapi.Handler.
func (h *Handlers) ProposeEndGame(ctx context.Context, params baldaapi.ProposeEndGameParams) (baldaapi.ProposeEndGameRes, error) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return &baldaapi.ProposeEndGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	if err := h.svc.ProposeEnd(ctx, uid, params.ID.String()); err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.ProposeEndGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, service.ErrNotParticipant):
			return &baldaapi.ProposeEndGameForbidden{
				Status:  baldaapi.NewOptInt(http.StatusForbidden),
				Message: baldaapi.NewOptString("not a participant of this game"),
				Type:    baldaapi.NewOptString("Forbidden"),
			}, nil
		case errors.Is(err, game.ErrNotYourTurn), errors.Is(err, game.ErrWrongState):
			return &baldaapi.ProposeEndGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString(err.Error()),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		default:
			slog.Error("propose_end_game: propose end", slog.Any("error", err))
			return &baldaapi.ProposeEndGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to propose end"),
				Type:    baldaapi.NewOptString("InternalError"),
			}, nil
		}
	}

	return &baldaapi.ProposeEndGameNoContent{}, nil
}
