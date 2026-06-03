package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Ping implements baldaapi.Handler.
// Updates the player's game presence and returns server time.
// Auth session TTL is not affected — presence and authentication are separate concerns.
func (h *Handlers) Ping(_ context.Context, params baldaapi.PingParams) (baldaapi.PingRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return unauthorized("session not found"), nil
		}
		slog.Error("ping: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return unauthorized("session unavailable"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.pres.Refresh(ctx, uid); err != nil {
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
