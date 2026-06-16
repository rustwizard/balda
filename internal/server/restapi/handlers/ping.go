package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rustwizard/balda/internal/auth"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// Ping implements baldaapi.Handler.
// Updates the player's game presence and returns server time.
// Identity comes from the JWT access token; presence and authentication are separate concerns.
func (h *Handlers) Ping(ctx context.Context, params baldaapi.PingParams) (baldaapi.PingRes, error) {
	claims, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized("unauthorized"), nil
	}

	// Presence is keyed by player_id (the identity the game FSM uses).
	playerID := claims.PlayerID.String()
	presCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := h.pres.Refresh(presCtx, playerID); err != nil {
		slog.Error("ping: refresh presence", slog.String("player_id", playerID), slog.Any("error", err))
	}

	return &baldaapi.PingNoContent{
		XRequestID:  baldaapi.NewOptInt64(params.XRequestID),
		XServerTime: baldaapi.NewOptInt64(time.Now().UnixMilli()),
	}, nil
}

func unauthorized(msg string) *baldaapi.ErrorResponse {
	return &baldaapi.ErrorResponse{
		Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
		Message: baldaapi.NewOptString(msg),
		Type:    baldaapi.NewOptString("Unauthorized"),
	}
}
