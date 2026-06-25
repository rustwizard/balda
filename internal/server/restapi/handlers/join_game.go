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
)

// JoinGame implements baldaapi.Handler.
func (h *Handlers) JoinGame(ctx context.Context, params baldaapi.JoinGameParams) (baldaapi.JoinGameRes, error) {
	uid, ok := uidFromContext(ctx)
	if !ok {
		return &baldaapi.JoinGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("unauthorized"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	rec, err := h.svc.JoinGame(ctx, uid, params.ID.String())
	if err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.JoinGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, lobby.ErrPlayerInGame):
			return &baldaapi.JoinGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString("player already in a game"),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		case errors.Is(err, lobby.ErrGameNotWaiting), errors.Is(err, lobby.ErrGameFull):
			return &baldaapi.JoinGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString("game is not available for joining"),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		default:
			slog.Error("join_game: join", slog.Any("error", err))
			return &baldaapi.JoinGameInternalServerError{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to join game"),
				Type:    baldaapi.NewOptString("InternalServerError"),
			}, nil
		}
	}

	gameID, err := uuid.Parse(rec.ID)
	if err != nil {
		slog.Error("join_game: parse game id", slog.Any("error", err))
		return &baldaapi.JoinGameConflict{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
		}, nil
	}

	playerIDs := make([]uuid.UUID, 0, len(rec.Players))
	lobbyPlayers := make([]baldaapi.LobbyPlayer, 0, len(rec.Players))
	for _, p := range rec.Players {
		pid, err := uuid.Parse(p.ID)
		if err != nil {
			slog.Warn("join_game: skip player with invalid id", slog.String("playerID", p.ID), slog.Any("error", err))
			continue
		}
		playerIDs = append(playerIDs, pid)
		lobbyPlayers = append(lobbyPlayers, baldaapi.LobbyPlayer{
			UID:    baldaapi.NewOptUUID(pid),
			Exp:    baldaapi.NewOptInt64(int64(p.Exp)),
			Rating: baldaapi.NewOptInt64(int64(p.Rating)),
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
		slog.Error("join_game: publish to lobby", slog.Any("error", err))
	}
	if err := h.cf.Publish(ctx, centrifugo.ChannelGame(rec.ID), ev); err != nil {
		slog.Error("join_game: publish to game channel", slog.Any("error", err))
	}
	h.publishLobbyUpdate(ctx)

	// The creator (index 0) always moves first.
	firstPlayerID := ""
	if len(rec.Players) > 0 {
		firstPlayerID = rec.Players[0].ID
	}

	gameToken, err := centrifugo.GenerateSubscriptionToken(
		strconv.FormatInt(uid, 10), centrifugo.ChannelGame(rec.ID), h.centrifugoTokenHMACSecret, 24*time.Hour,
	)
	if err != nil {
		slog.Error("join_game: generate game token", slog.Any("error", err))
		return &baldaapi.JoinGameConflict{
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Message: baldaapi.NewOptString("internal error"),
		}, nil
	}

	// Build the board as a slice-of-slices for the HTTP response so the joining
	// player can render the initial word immediately without racing Centrifugo.
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
