package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rustwizard/balda/internal/auth"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// RefreshToken implements baldaapi.Handler. It validates the presented refresh
// token, rotates it (revoking the old one), and returns a fresh token pair.
func (h *Handlers) RefreshToken(ctx context.Context, req *baldaapi.RefreshRequest) (baldaapi.RefreshTokenRes, error) {
	hash := auth.HashRefreshToken(h.cfg.JWTSecret, req.RefreshToken)

	rt, err := h.svc.GetRefreshToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return refreshUnauthorized("token_unknown"), nil
	}
	if err != nil {
		slog.Error("refresh: get token", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	// A revoked token presented again signals a replay: revoke the whole family.
	if rt.Revoked {
		if err := h.svc.RevokeAllUserTokens(ctx, rt.UserID); err != nil {
			slog.Error("refresh: revoke all on replay", slog.Int64("uid", rt.UserID), slog.Any("error", err))
		}
		return refreshUnauthorized("token_revoked"), nil
	}

	if time.Now().After(rt.ExpiresAt) {
		return refreshUnauthorized("token_expired"), nil
	}

	// Rotate: invalidate the presented token before issuing a new pair.
	if err := h.svc.RevokeRefreshToken(ctx, hash); err != nil {
		slog.Error("refresh: revoke old", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	u, err := h.svc.GetUserForToken(ctx, rt.UserID)
	if err != nil {
		slog.Error("refresh: get user", slog.Int64("uid", rt.UserID), slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	access, refresh, err := h.issueTokens(ctx, rt.UserID, u.PlayerID, u.Role, tokenMeta{UserAgent: rt.UserAgent, IPAddr: rt.IPAddr})
	if err != nil {
		slog.Error("refresh: issue tokens", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	return &baldaapi.RefreshResponse{
		AccessToken:  baldaapi.NewOptString(access),
		RefreshToken: baldaapi.NewOptString(refresh),
		TokenType:    baldaapi.NewOptString("Bearer"),
		ExpiresIn:    baldaapi.NewOptInt(int(auth.AccessTokenTTL.Seconds())),
	}, nil
}

func refreshUnauthorized(code string) *baldaapi.ErrorResponse {
	return &baldaapi.ErrorResponse{
		Message: baldaapi.NewOptString(code),
		Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
		Type:    baldaapi.NewOptString("Unauthorized"),
	}
}
