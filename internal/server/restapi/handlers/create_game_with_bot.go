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

// CreateGameWithBot implements baldaapi.Handler.
func (h *Handlers) CreateGameWithBot(ctx context.Context, params baldaapi.CreateGameWithBotParams) (baldaapi.CreateGameWithBotRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.CreateGameWithBotUnauthorized{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("create_game_with_bot: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return &baldaapi.CreateGameWithBotUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	rec, err := h.svc.CreateGameWithBot(ctx, uid)
	if err != nil {
		if errors.Is(err, lobby.ErrPlayerInGame) {
			return &baldaapi.CreateGameWithBotConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString("player already in a game"),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		}
		slog.Error("create_game_with_bot: create", slog.Any("error", err))
		return &baldaapi.CreateGameWithBotInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("failed to create game with bot"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	gameID, err := uuid.Parse(rec.ID)
	if err != nil {
		slog.Error("create_game_with_bot: parse game id", slog.Any("error", err))
		return &baldaapi.CreateGameWithBotInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	playerIDs := make([]uuid.UUID, 0, len(rec.Players))
	lobbyPlayers := make([]baldaapi.LobbyPlayer, 0, len(rec.Players))
	for _, p := range rec.Players {
		pid, err := uuid.Parse(p.ID)
		if err != nil {
			continue
		}
		playerIDs = append(playerIDs, pid)
		lobbyPlayers = append(lobbyPlayers, baldaapi.LobbyPlayer{
			UID: baldaapi.NewOptUUID(pid),
			Exp: baldaapi.NewOptInt64(int64(p.Exp)),
		})
	}

	ev := centrifugo.EvGameStarted{
		Type:      "game_started",
		GameID:    rec.ID,
		Status:    centrifugo.GameStatusInProgress,
		StartedAt: rec.StartedAt.UnixMilli(),
		PlayerIDs: make([]string, 0, len(rec.Players)),
	}
	for _, p := range rec.Players {
		ev.PlayerIDs = append(ev.PlayerIDs, p.ID)
	}
	if err := h.cf.Publish(ctx, centrifugo.ChannelLobby, ev); err != nil {
		slog.Error("create_game_with_bot: publish to lobby", slog.Any("error", err))
	}
	if err := h.cf.Publish(ctx, centrifugo.ChannelGame(rec.ID), ev); err != nil {
		slog.Error("create_game_with_bot: publish to game channel", slog.Any("error", err))
	}
	h.publishLobbyUpdate(ctx)

	// The human player (index 0) always moves first.
	firstPlayerID := ""
	if len(rec.Players) > 0 {
		firstPlayerID = rec.Players[0].ID
	}

	gameToken, err := centrifugo.GenerateSubscriptionToken(
		strconv.FormatInt(uid, 10), centrifugo.ChannelGame(rec.ID), h.centrifugoTokenHMACSecret, 24*time.Hour,
	)
	if err != nil {
		slog.Error("create_game_with_bot: generate game token", slog.Any("error", err))
		return &baldaapi.CreateGameWithBotInternalServerError{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	rawBoard := rec.Game.BoardSnapshot()
	boardSlice := make([][]string, len(rawBoard))
	for i, row := range rawBoard {
		r := make([]string, len(row))
		copy(r, row[:])
		boardSlice[i] = r
	}

	return &baldaapi.JoinGameResponse{
		Game: baldaapi.NewOptGameSummary(baldaapi.GameSummary{
			ID:        baldaapi.NewOptUUID(gameID),
			PlayerIds: playerIDs,
			Players:   lobbyPlayers,
			Status:    baldaapi.NewOptGameStatus(baldaapi.GameStatusInProgress),
			StartedAt: baldaapi.NewOptInt64(rec.StartedAt.UnixMilli()),
		}),
		GameToken:      baldaapi.NewOptString(gameToken),
		Board:          boardSlice,
		CurrentTurnUID: baldaapi.NewOptString(firstPlayerID),
	}, nil
}
