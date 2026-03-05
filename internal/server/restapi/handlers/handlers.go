package handlers

import (
	"context"
	"errors"
	"log/slog"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/service"
	"github.com/rustwizard/balda/internal/session"
)

// Handlers implements baldaapi.Handler and baldaapi.SecurityHandler.
type Handlers struct {
	svc       *service.Balda
	sess      *session.Service
	xAPIToken string
}

func New(svc *service.Balda, sess *session.Service, xAPIToken string) *Handlers {
	return &Handlers{svc: svc, sess: sess, xAPIToken: xAPIToken}
}

// HandleAPIKeyHeader implements baldaapi.SecurityHandler.
func (h *Handlers) HandleAPIKeyHeader(ctx context.Context, _ baldaapi.OperationName, t baldaapi.APIKeyHeader) (context.Context, error) {
	slog.Info("KeyAuth header handler called")
	if t.APIKey == h.xAPIToken {
		return ctx, nil
	}
	slog.Error("access attempt with incorrect api key header", slog.String("token", t.APIKey))
	return nil, errors.New("api key header: token error")
}

// HandleAPIKeyQueryParam implements baldaapi.SecurityHandler.
func (h *Handlers) HandleAPIKeyQueryParam(ctx context.Context, _ baldaapi.OperationName, t baldaapi.APIKeyQueryParam) (context.Context, error) {
	slog.Info("KeyAuth query param handler called")
	if t.APIKey == h.xAPIToken {
		return ctx, nil
	}
	slog.Error("access attempt with incorrect api key query param", slog.String("token", t.APIKey))
	return nil, errors.New("api key param: token error")
}
