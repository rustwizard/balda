package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/runtime/middleware"
	"github.com/rs/zerolog/log"
	"github.com/rustwizard/balda/internal/server/models"
	"github.com/rustwizard/balda/internal/server/restapi/operations/auth"
	"github.com/rustwizard/balda/internal/session"
	"github.com/rustwizard/cleargo/db/pg"
)

type Auth struct {
	db   *pg.DB
	sess *session.Service
}

func (a Auth) Handle(params auth.PostAuthParams, i interface{}) middleware.Responder {
	log.Info().Msg("auth handler called")

	// TODO: create session

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := &models.User{}
	err := a.db.Pool.QueryRow(ctx, `SELECT user_id, first_name, last_name FROM users WHERE email = $1 AND 
						hash_password = crypt($2, hash_password)
									`, params.Body.Email, params.Body.Password).
		Scan(&user.UID, &user.Firstname, &user.Lastname)
	if err != nil {
		log.Error().Err(err).Msg("auth: fetch user form db")
		return auth.NewPostAuthUnauthorized().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusUnauthorized,
			Type:    "Auth Error",
		})
	}
	if user.UID == 0 {
		log.Error().Err(err).Msg("auth: wrong email/password")
		return auth.NewPostAuthUnauthorized().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusUnauthorized,
			Type:    "Auth Error",
		})
	}

	return auth.NewPostAuthOK().WithPayload(&models.AuthResponse{User: user})
}

func NewAuth(db *pg.DB, sess *session.Service) *Auth {
	return &Auth{db: db, sess: sess}
}
