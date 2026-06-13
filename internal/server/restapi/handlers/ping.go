package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// Ping implements baldaapi.Handler.
// Updates the player's game presence and returns server time.
// Identity comes from the JWT access token; presence and authentication are separate concerns.
func (h *Handlers) Ping(ctx context.Context, params baldaapi.PingParams) (baldaapi.PingRes, error) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return unauthorized("unauthorized"), nil
	}

	presCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := h.pres.Refresh(presCtx, uid); err != nil {
		slog.Error("ping: refresh presence", slog.Int64("uid", uid), slog.Any("error", err))
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
