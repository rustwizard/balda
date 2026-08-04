package handlers

import (
	"context"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetConfig implements baldaapi.Handler. It is public (no security) so the
// client can adapt its UI before authentication.
func (h *Handlers) GetConfig(_ context.Context) (*baldaapi.ConfigResponse, error) {
	return &baldaapi.ConfigResponse{
		EmailSignupEnabled: baldaapi.NewOptBool(h.emailSignupEnabled),
		TelegramAppURL:     baldaapi.NewOptString(h.telegramAppURL),
	}, nil
}
