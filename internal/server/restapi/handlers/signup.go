package handlers

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/go-openapi/runtime/middleware"
	"github.com/rustwizard/balda/internal/server/models"
	"github.com/rustwizard/balda/internal/server/restapi/operations/signup"
	"github.com/rustwizard/cleargo/db/pg"
)

type SignUp struct {
	db *pg.DB
}

func NewSignUp(db *pg.DB) *SignUp {
	return &SignUp{db: db}
}

func (s *SignUp) Handle(params signup.PostSignupParams) middleware.Responder {
	log.Info().Msg("signup handler called")
	// TODO: use context in proper way
	ctx := context.Background()
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
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	err = tx.Commit(ctx)
	if err != nil {
		if err = tx.Rollback(ctx); err != nil {
			log.Error().Err(err).Msg("signup: rollback")
		}
		return signup.NewPostSignupBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "SignUp Error",
		})
	}

	// TODO: generate session_id and put sid to the session storage
	sid := "test_sid"
	return signup.NewPostSignupOK().WithPayload(&models.SignupResponse{User: &models.User{
		Firstname: *params.Body.Firstname,
		Key:       apiKey,
		Lastname:  *params.Body.Lastname,
		Sid:       sid,
		UID:       uid,
	}})
}
