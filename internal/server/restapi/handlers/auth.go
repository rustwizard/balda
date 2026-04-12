package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/centrifugo"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Auth implements baldaapi.Handler.
func (h *Handlers) Auth(ctx context.Context, req *baldaapi.AuthRequest) (baldaapi.AuthRes, error) {
	slog.Info("auth handler called")

	var firstname, lastname string
	var uid int64
	var playerID uuid.UUID
	err := h.svc.DB().Pool().QueryRow(ctx, `
		SELECT u.user_id, u.first_name, u.last_name, ps.player_id
		FROM users u
		JOIN player_state ps ON ps.user_id = u.user_id
		WHERE u.email = $1 AND u.hash_password = crypt($2, u.hash_password)
	`, req.Email, req.Password).
		Scan(&uid, &firstname, &lastname, &playerID)
	if err != nil {
		if uid == 0 {
			slog.Error("auth: wrong email/password", slog.Any("error", err))
		} else {
			slog.Error("auth: fetch user from db", slog.Any("error", err))
		}
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}

	player := baldaapi.Player{
		UID:       baldaapi.NewOptUUID(playerID),
		Firstname: baldaapi.NewOptString(firstname),
		Lastname:  baldaapi.NewOptString(lastname),
	}

	sid, err := h.sess.Get(uid)
	if err == session.ErrNotFound {
		sidStr, err := h.sess.Create(uid)
		if err != nil {
			slog.Error("auth: create sid", slog.Any("error", err))
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString(""),
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Type:    baldaapi.NewOptString("Auth Error"),
			}, nil
		}
		player.Sid = baldaapi.NewOptString(sidStr)
		cfToken, err := centrifugo.GenerateConnectionToken(playerID.String(), h.centrifugoTokenHMACSecret, 24*time.Hour)
		if err != nil {
			slog.Error("auth: generate centrifugo token", slog.Any("error", err))
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString(""),
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Type:    baldaapi.NewOptString("Auth Error"),
			}, nil
		}
		return &baldaapi.AuthResponse{
			Player:          baldaapi.NewOptPlayer(player),
			CentrifugoToken: baldaapi.NewOptString(cfToken),
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
	cfToken, err := centrifugo.GenerateConnectionToken(playerID.String(), h.centrifugoTokenHMACSecret, 24*time.Hour)
	if err != nil {
		slog.Error("auth: generate centrifugo token", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}
	return &baldaapi.AuthResponse{
		Player:          baldaapi.NewOptPlayer(player),
		CentrifugoToken: baldaapi.NewOptString(cfToken),
	}, nil
}
