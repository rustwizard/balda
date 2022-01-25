package handlers

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/rs/zerolog/log"
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
	// TODO: create user in db
	// TODO: fill the response
	log.Info().Msg("signup handler called")
	return signup.NewPostSignupOK().WithPayload(&models.SignupResponse{})
}
