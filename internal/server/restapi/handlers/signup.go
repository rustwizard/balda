package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/flname"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Signup implements baldaapi.Handler.
func (h *Handlers) Signup(ctx context.Context, req *baldaapi.SignupRequest) (baldaapi.SignupRes, error) {
	slog.Info("signup handler called")

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	var uid int64
	var apiKey string
	err = tx.QueryRow(ctx, `INSERT INTO users(first_name, last_name, email, hash_password)
		VALUES($1, $2, $3, crypt($4, gen_salt('bf', 8))) RETURNING user_id, api_key`,
		req.Firstname, req.Lastname, req.Email, req.Password).Scan(&uid, &apiKey)
	if err != nil {
		slog.Error("signup: user", slog.Any("error", err))
		if err = tx.Rollback(ctx); err != nil {
			slog.Error("signup: rollback", slog.Any("error", err))
		}
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("user"),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	_, err = tx.Exec(ctx, `INSERT INTO user_state(user_id, nickname, exp, flags, lives)
		VALUES($1, $2, $3, $4, $5)`, uid, flname.GenNickname(), 0, 0, 5)
	if err != nil {
		slog.Error("signup: user state", slog.Any("error", err))
		if err = tx.Rollback(ctx); err != nil {
			slog.Error("signup: rollback", slog.Any("error", err))
		}
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("user state"),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	if err = tx.Commit(ctx); err != nil {
		if err = tx.Rollback(ctx); err != nil {
			slog.Error("signup: rollback", slog.Any("error", err))
		}
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	sid, err := uuid.NewRandom()
	if err != nil {
		slog.Error("signup: gen session id", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	if err = h.sess.Save(&session.User{Sid: sid.String(), UID: uid}); err != nil {
		slog.Error("signup: save session", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	return &baldaapi.SignupResponse{
		User: baldaapi.NewOptUser(baldaapi.User{
			UID:       baldaapi.NewOptInt64(uid),
			Firstname: baldaapi.NewOptString(req.Firstname),
			Lastname:  baldaapi.NewOptString(req.Lastname),
			Sid:       baldaapi.NewOptString(sid.String()),
			Key:       baldaapi.NewOptString(apiKey),
		}),
	}, nil
}
