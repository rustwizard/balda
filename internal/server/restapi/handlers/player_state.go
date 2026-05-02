package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetPlayerStateUID implements baldaapi.Handler.
func (h *Handlers) GetPlayerStateUID(ctx context.Context, params baldaapi.GetPlayerStateUIDParams) (baldaapi.GetPlayerStateUIDRes, error) {
	ps, err := h.svc.GetPlayerState(ctx, params.UID)
	if err != nil {
		slog.Error("player state: fetch from db", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("PlayerState Error"),
		}, nil
	}

	gs := h.svc.GameSummary(params.UID.String())
	if gs == nil {
		return &baldaapi.PlayerState{
			UID:      baldaapi.NewOptUUID(params.UID),
			Nickname: baldaapi.NewOptString(ps.Nickname),
			Exp:      baldaapi.NewOptInt64(ps.Exp),
			Flags:    baldaapi.NewOptInt64(ps.Flags),
			Lives:    baldaapi.NewOptInt64(ps.Lives),
		}, nil
	}

	gameID, err := uuid.Parse(gs.ID)
	if err != nil {
		slog.Error("player state: parse game id", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("PlayerState Error"),
		}, nil
	}

	return &baldaapi.PlayerState{
		UID:      baldaapi.NewOptUUID(params.UID),
		Nickname: baldaapi.NewOptString(ps.Nickname),
		Exp:      baldaapi.NewOptInt64(ps.Exp),
		Flags:    baldaapi.NewOptInt64(ps.Flags),
		Lives:    baldaapi.NewOptInt64(ps.Lives),
		GameID:   baldaapi.NewOptUUID(gameID),
	}, nil
}
