package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
	"github.com/google/uuid"
)

// JoinGame implements baldaapi.Handler.
func (h *Handlers) JoinGame(ctx context.Context, params baldaapi.JoinGameParams) (baldaapi.JoinGameRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.JoinGameUnauthorized{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("join_game: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return &baldaapi.JoinGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	rec, err := h.svc.JoinGame(ctx, uid, params.ID.String())
	if err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.JoinGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, lobby.ErrPlayerInGame):
			return &baldaapi.JoinGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString("player already in a game"),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		case errors.Is(err, lobby.ErrGameNotWaiting), errors.Is(err, lobby.ErrGameFull):
			return &baldaapi.JoinGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString("game is not available for joining"),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		default:
			slog.Error("join_game: join", slog.Any("error", err))
			return &baldaapi.JoinGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to join game"),
			}, nil
		}
	}

	gameID, err := uuid.Parse(rec.ID)
	if err != nil {
		slog.Error("join_game: parse game id", slog.Any("error", err))
		return &baldaapi.JoinGameConflict{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
		}, nil
	}

	playerIDs := make([]uuid.UUID, 0, len(rec.Players))
	for _, p := range rec.Players {
		pid, err := uuid.Parse(p.ID)
		if err != nil {
			continue
		}
		playerIDs = append(playerIDs, pid)
	}

	return &baldaapi.JoinGameResponse{
		Game: baldaapi.NewOptGameSummary(baldaapi.GameSummary{
			ID:        baldaapi.NewOptUUID(gameID),
			PlayerIds: playerIDs,
			Status:    baldaapi.NewOptGameStatus(baldaapi.GameStatusInProgress),
			StartedAt: baldaapi.NewOptInt64(rec.StartedAt.UnixMilli()),
		}),
	}, nil
}
