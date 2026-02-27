package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rustwizard/balda/internal/flname"

	"github.com/rustwizard/balda/internal/session"

	"github.com/go-openapi/runtime/middleware"
	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/server/models"
	"github.com/rustwizard/balda/internal/server/restapi/operations/signup"
	"github.com/rustwizard/cleargo/db/pg"
)

type SignUp struct {
	db   *pg.DB
	sess *session.Service
}

func NewSignUp(db *pg.DB, sess *session.Service) *SignUp {
	return &SignUp{db: db, sess: sess}
}

func (s *SignUp) Handle(params signup.PostSignupParams) middleware.Responder {
	slog.Info("signup handler called")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	var uid int64
	var apiKey string
	err = tx.QueryRow(ctx, `INSERT INTO users(first_name, last_name, email, hash_password)
		VALUES($1, $2, $3, crypt($4, gen_salt('bf', 8))) RETURNING user_id, api_key`, params.Body.Firstname,
		params.Body.Lastname, params.Body.Email, params.Body.Password).Scan(&uid, &apiKey)
	if err != nil {
		slog.Error("signup: user", slog.Any("error", err))
		if err = tx.Rollback(ctx); err != nil {
			slog.Error("signup: rollback", slog.Any("error", err))
		}
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "user",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	_, err = tx.Exec(ctx, `INSERT INTO user_state(user_id, nickname, exp, flags, lives)
		VALUES($1, $2, $3, $4, $5)`, uid,
		flname.GenNickname(), 0, 0, 5)
	if err != nil {
		slog.Error("signup: user state", slog.Any("error", err))
		if err = tx.Rollback(ctx); err != nil {
			slog.Error("signup: rollback", slog.Any("error", err))
		}
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "user state",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	err = tx.Commit(ctx)
	if err != nil {
		if err = tx.Rollback(ctx); err != nil {
			slog.Error("signup: rollback", slog.Any("error", err))
		}
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	sid, err := uuid.NewRandom()
	if err != nil {
		slog.Error("signup: gen session id", slog.Any("error", err))
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	err = s.sess.Save(&session.User{
		Sid: sid.String(),
		UID: uid,
	})
	if err != nil {
		slog.Error("signup: gen session id", slog.Any("error", err))
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	return signup.NewPostSignupOK().WithPayload(&models.SignupResponse{User: &models.User{
		Firstname: *params.Body.Firstname,
		Key:       apiKey,
		Lastname:  *params.Body.Lastname,
		Sid:       sid.String(),
		UID:       uid,
	}})
}
