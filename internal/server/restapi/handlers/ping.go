package handlers

import (
	"context"
	"time"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// Ping implements baldaapi.Handler.
func (h *Handlers) Ping(_ context.Context, params baldaapi.PingParams) (baldaapi.PingRes, error) {
	return &baldaapi.PingNoContent{
		XRequestID:  baldaapi.NewOptInt64(params.XRequestID),
		XServerTime: baldaapi.NewOptInt64(time.Now().UnixMilli()),
	}, nil
}
