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

	var uid int64
	var firstname, lastname string
	err := h.db.Pool.QueryRow(ctx, `SELECT user_id, first_name, last_name FROM users WHERE email = $1 AND
					hash_password = crypt($2, hash_password)
								`, req.Email, req.Password).
		Scan(&uid, &firstname, &lastname)
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

	user := baldaapi.User{
		UID:       baldaapi.NewOptInt64(uid),
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
		user.Sid = baldaapi.NewOptString(sidStr)
		return &baldaapi.AuthResponse{User: baldaapi.NewOptUser(user)}, nil
	}
	if err != nil {
		slog.Error("auth: get sid", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}

	user.Sid = baldaapi.NewOptString(sid.Sid)
	return &baldaapi.AuthResponse{User: baldaapi.NewOptUser(user)}, nil
}
