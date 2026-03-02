package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetUsersStateUID implements baldaapi.Handler.
func (h *Handlers) GetUsersStateUID(ctx context.Context, params baldaapi.GetUsersStateUIDParams) (baldaapi.GetUsersStateUIDRes, error) {
	var nickname string
	var exp, flags, lives int64
	var gameID int32

	if err := pgx.BeginTxFunc(ctx, h.db.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT nickname, exp, flags, lives FROM user_state WHERE user_id = $1`, params.UID).
			Scan(&nickname, &exp, &flags, &lives)
	}); err != nil {
		slog.Error("user state: fetch user state from db", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("UserState Error"),
		}, nil
	}

	// TODO: check if user in the game and set gameID
	_ = gameID

	return &baldaapi.UserState{
		UID:      baldaapi.NewOptInt64(params.UID),
		Nickname: baldaapi.NewOptString(nickname),
		Exp:      baldaapi.NewOptInt64(exp),
		Flags:    baldaapi.NewOptInt64(flags),
		Lives:    baldaapi.NewOptInt64(lives),
	}, nil
}
