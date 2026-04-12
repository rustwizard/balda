package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/flname"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Signup implements baldaapi.Handler.
func (h *Handlers) Signup(ctx context.Context, req *baldaapi.SignupRequest) (baldaapi.SignupRes, error) {
	slog.Info("signup handler called")

	tx, err := h.svc.DB().Pool().Begin(ctx)
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

	var playerID string
	err = tx.QueryRow(ctx, `INSERT INTO player_state(user_id, nickname, exp, flags, lives)
		VALUES($1, $2, $3, $4, $5) RETURNING player_id`, uid, flname.GenNickname(), 0, 0, 5).Scan(&playerID)
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

	pid, err := uuid.Parse(playerID)
	if err != nil {
		slog.Error("signup: parse player_id", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	cfToken, err := centrifugo.GenerateConnectionToken(pid.String(), h.centrifugoTokenHMACSecret, 24*time.Hour)
	if err != nil {
		slog.Error("signup: generate centrifugo token", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	return &baldaapi.SignupResponse{
		User: baldaapi.NewOptPlayer(baldaapi.Player{
			UID:       baldaapi.NewOptUUID(pid),
			Firstname: baldaapi.NewOptString(req.Firstname),
			Lastname:  baldaapi.NewOptString(req.Lastname),
			Sid:       baldaapi.NewOptString(sid.String()),
			Key:       baldaapi.NewOptString(apiKey),
		}),
		CentrifugoToken: baldaapi.NewOptString(cfToken),
	}, nil
}
