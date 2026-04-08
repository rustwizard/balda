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
	_, err := h.sess.Get(params.XAPIUser)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.ErrorResponse{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("ping: get session", slog.Int64("uid", params.XAPIUser), slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	return &baldaapi.PingNoContent{
		XRequestID:  baldaapi.NewOptInt64(params.XRequestID),
		XServerTime: baldaapi.NewOptInt64(time.Now().UnixMilli()),
	}, nil
}
