package handlers

import (
	"net/http"

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

	ss, err := a.sess.Get(params.HTTPRequest)
	if err == session.ErrNotFound {
		// TODO: create session
	}
	if err != nil {
		return auth.NewPostAuthUnauthorized().WithPayload(&models.ErrorResponse{
			Message: err.Error(),
			Status:  http.StatusUnauthorized,
			Type:    "SignUp Error",
		})
	}
	_ = ss
	return auth.NewPostAuthOK()
}

func NewAuth(db *pg.DB, sess *session.Service) *Auth {
	return &Auth{db: db, sess: sess}
}
