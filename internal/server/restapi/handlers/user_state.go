package handlers

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/rustwizard/balda/internal/server/restapi/operations"
	"github.com/rustwizard/balda/internal/session"
	"github.com/rustwizard/cleargo/db/pg"
)

type UserState struct {
	db   *pg.DB
	sess *session.Service
}

func (u UserState) Handle(params operations.GetUsersStateUIDParams) middleware.Responder {
	//TODO implement me
	panic("implement me")
}

func NewUserState(db *pg.DB, sess *session.Service) *UserState {
	return &UserState{db: db, sess: sess}
}
