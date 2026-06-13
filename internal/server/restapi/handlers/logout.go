package handlers

import (
	"context"
	"log/slog"

	"github.com/rustwizard/balda/internal/auth"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// Logout implements baldaapi.Handler. Requires a valid access token (BearerAuth)
// and revokes the refresh token for this device when one is supplied.
//
// Note: the access token itself remains valid until it expires (≤1h); a jti
// blacklist for immediate access-token revocation is a future enhancement.
func (h *Handlers) Logout(ctx context.Context, req baldaapi.OptLogoutRequest) (baldaapi.LogoutRes, error) {
	if req.Set && req.Value.RefreshToken.Set && req.Value.RefreshToken.Value != "" {
		hash := auth.HashRefreshToken(h.jwtSecret, req.Value.RefreshToken.Value)
		if err := h.svc.RevokeRefreshToken(ctx, hash); err != nil {
			slog.Error("logout: revoke refresh token", slog.Any("error", err))
		}
	}
	return &baldaapi.LogoutNoContent{}, nil
}
