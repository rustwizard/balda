package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// CreateGame implements baldaapi.Handler.
func (h *Handlers) CreateGame(ctx context.Context, params baldaapi.CreateGameParams) (baldaapi.CreateGameRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return unauthorized("session not found"), nil
		}
		slog.Error("create_game: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return unauthorized("session unavailable"), nil
	}

	rec, err := h.svc.CreateGame(ctx, uid)
	if err != nil {
		if errors.Is(err, lobby.ErrPlayerInGame) {
			return &baldaapi.ErrorResponse{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString("player already in a game"),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		}
		slog.Error("create_game: create", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to create game"),
		}, nil
	}

	gameID, err := uuid.Parse(rec.ID)
	if err != nil {
		slog.Error("create_game: parse game id", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
		}, nil
	}

	playerIDs := make([]uuid.UUID, 0, len(rec.Players))
	for _, p := range rec.Players {
		pid, err := uuid.Parse(p.ID)
		if err != nil {
			continue
		}
		playerIDs = append(playerIDs, pid)
	}

	ev := centrifugo.EvGameCreated{
		Type:    "game_created",
		GameID:  rec.ID,
		Status:  "waiting",
		Players: make([]string, 0, len(rec.Players)),
	}
	for _, p := range rec.Players {
		ev.Players = append(ev.Players, p.ID)
	}
	if err := h.cf.Publish(ctx, centrifugo.ChannelLobby, ev); err != nil {
		slog.Error("create_game: publish game_created", slog.Any("error", err))
	}

	h.publishLobbyUpdate(ctx)

	gameToken, err := centrifugo.GenerateSubscriptionToken(
		strconv.FormatInt(uid, 10), centrifugo.ChannelGame(rec.ID), h.centrifugoTokenHMACSecret, 24*time.Hour,
	)
	if err != nil {
		slog.Error("create_game: generate game token", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
		}, nil
	}

	return &baldaapi.CreateGameResponse{
		Game: baldaapi.NewOptGameSummary(baldaapi.GameSummary{
			ID:        baldaapi.NewOptUUID(gameID),
			PlayerIds: playerIDs,
			Status:    baldaapi.NewOptGameStatus(baldaapi.GameStatusWaiting),
			StartedAt: baldaapi.NewOptInt64(rec.StartedAt.UnixMilli()),
		}),
		GameToken: baldaapi.NewOptString(gameToken),
	}, nil
}
