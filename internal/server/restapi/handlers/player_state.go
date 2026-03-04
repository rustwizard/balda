package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetPlayerStateUID implements baldaapi.Handler.
func (h *Handlers) GetPlayerStateUID(ctx context.Context, params baldaapi.GetPlayerStateUIDParams) (baldaapi.GetPlayerStateUIDRes, error) {
	var nickname string
	var exp, flags, lives int64
	var gameID int32

	if err := pgx.BeginTxFunc(ctx, h.svc.DB().Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT nickname, exp, flags, lives FROM player_state WHERE player_id = $1`, params.UID).
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

	return &baldaapi.PlayerState{
		UID:      baldaapi.NewOptUUID(params.UID),
		Nickname: baldaapi.NewOptString(nickname),
		Exp:      baldaapi.NewOptInt64(exp),
		Flags:    baldaapi.NewOptInt64(flags),
		Lives:    baldaapi.NewOptInt64(lives),
	}, nil
}
