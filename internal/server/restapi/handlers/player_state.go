package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// GetPlayerStateUID implements baldaapi.Handler.
func (h *Handlers) GetPlayerStateUID(ctx context.Context, params baldaapi.GetPlayerStateUIDParams) (baldaapi.GetPlayerStateUIDRes, error) {
	var nickname string
	var exp, flags, lives int64

	if err := pgx.BeginTxFunc(ctx, h.svc.DB().Pool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT nickname, exp, flags, lives FROM player_state WHERE player_id = $1`, params.UID).
			Scan(&nickname, &exp, &flags, &lives)
	}); err != nil {
		slog.Error("player state: fetch user state from db", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("PlayerState Error"),
		}, nil
	}

	gs := h.svc.GameSummary(params.UID.String())
	if gs == nil {
		return &baldaapi.PlayerState{
			UID:      baldaapi.NewOptUUID(params.UID),
			Nickname: baldaapi.NewOptString(nickname),
			Exp:      baldaapi.NewOptInt64(exp),
			Flags:    baldaapi.NewOptInt64(flags),
			Lives:    baldaapi.NewOptInt64(lives),
		}, nil
	}

	gameID, err := uuid.Parse(gs.ID)
	if err != nil {
		slog.Error("player state: parse game id", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("PlayerState Error"),
		}, nil
	}

	return &baldaapi.PlayerState{
		UID:      baldaapi.NewOptUUID(params.UID),
		Nickname: baldaapi.NewOptString(nickname),
		Exp:      baldaapi.NewOptInt64(exp),
		Flags:    baldaapi.NewOptInt64(flags),
		Lives:    baldaapi.NewOptInt64(lives),
		GameID:   baldaapi.NewOptUUID(gameID),
	}, nil
}
