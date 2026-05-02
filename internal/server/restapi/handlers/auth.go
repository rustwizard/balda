package handlers

import (
	"context"
	"log/slog"
	"net/http"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Auth implements baldaapi.Handler.
func (h *Handlers) Auth(ctx context.Context, req *baldaapi.AuthRequest) (baldaapi.AuthRes, error) {
	slog.Info("auth handler called")

	u, err := h.svc.AuthUser(ctx, req.Email, req.Password)
	if err != nil {
		slog.Error("auth: wrong email/password or db error", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}

	player := baldaapi.Player{
		UID:       baldaapi.NewOptUUID(u.PlayerID),
		Firstname: baldaapi.NewOptString(u.Firstname),
		Lastname:  baldaapi.NewOptString(u.Lastname),
		Exp:       baldaapi.NewOptInt64(u.Exp),
	}

	sid, err := h.sess.Get(u.UID)
	if err == session.ErrNotFound {
		sidStr, err := h.sess.Create(u.UID)
		if err != nil {
			slog.Error("auth: create sid", slog.Any("error", err))
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString(""),
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Type:    baldaapi.NewOptString("Auth Error"),
			}, nil
		}
		player.Sid = baldaapi.NewOptString(sidStr)
		cfToken, lobbyToken, err := h.generateCentrifugoTokens(u.UID)
		if err != nil {
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString(""),
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Type:    baldaapi.NewOptString("Auth Error"),
			}, nil
		}
		return &baldaapi.AuthResponse{
			Player:          baldaapi.NewOptPlayer(player),
			CentrifugoToken: baldaapi.NewOptString(cfToken),
			LobbyToken:      baldaapi.NewOptString(lobbyToken),
		}, nil
	}
	if err != nil {
		slog.Error("auth: get sid", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}

	player.Sid = baldaapi.NewOptString(sid.Sid)
	cfToken, lobbyToken, err := h.generateCentrifugoTokens(u.UID)
	if err != nil {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}
	return &baldaapi.AuthResponse{
		Player:          baldaapi.NewOptPlayer(player),
		CentrifugoToken: baldaapi.NewOptString(cfToken),
		LobbyToken:      baldaapi.NewOptString(lobbyToken),
	}, nil
}
