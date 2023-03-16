package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/runtime/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"github.com/rustwizard/balda/internal/server/models"
	"github.com/rustwizard/balda/internal/server/restapi/operations"
	"github.com/rustwizard/balda/internal/session"
	"github.com/rustwizard/cleargo/db/pg"
)

type UserState struct {
	db   *pg.DB
	sess *session.Service
}

func (u UserState) Handle(params operations.GetUsersStateUIDParams) middleware.Responder {
	userState := models.UserState{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pgx.BeginTxFunc(ctx, u.db.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT nickname, exp, flags, lives  FROM user_state WHERE user_id = $1`, params.UID).
			Scan(&userState.Nickname, &userState.Exp, &userState.Flags, &userState.Lives); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error().Err(err).Msg("user state: fetch user state from db")
		return operations.NewGetUsersStateUIDBadRequest().WithPayload(&models.ErrorResponse{
			Message: "",
			Status:  http.StatusBadRequest,
			Type:    "UserState Error",
		})
	}

	userState.UID = params.UID
	// TODO: check if user in the game and set gameID

	return operations.NewGetUsersStateUIDOK().WithPayload(&userState)
}

func NewUserState(db *pg.DB, sess *session.Service) *UserState {
	return &UserState{db: db, sess: sess}
}
