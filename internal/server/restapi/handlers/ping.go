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
// Refreshes the session TTL and returns server time.
func (h *Handlers) Ping(_ context.Context, params baldaapi.PingParams) (baldaapi.PingRes, error) {
	if err := h.sess.Refresh(params.XAPISession); err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return unauthorized("session not found"), nil
		}
		slog.Error("ping: refresh session", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return unauthorized("session unavailable"), nil
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
