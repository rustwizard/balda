package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"strconv"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Auth implements baldaapi.Handler.
func (h *Handlers) Auth(ctx context.Context, req *baldaapi.AuthRequest) (baldaapi.AuthRes, error) {
	slog.Info("auth handler called")

	u, err := h.svc.AuthUser(ctx, req.Email, req.Password)
	if err != nil {
		slog.Error("auth: wrong email/password or db error", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}

	player := baldaapi.Player{
		UID:       baldaapi.NewOptUUID(u.PlayerID),
		Firstname: baldaapi.NewOptString(u.Firstname),
		Lastname:  baldaapi.NewOptString(u.Lastname),
		Exp:       baldaapi.NewOptInt64(u.Exp),
	}

	sid, err := h.sess.Get(u.UID)
	if errors.Is(err, session.ErrNotFound) {
		sidStr, err := h.sess.Create(u.UID)
		if err != nil {
			slog.Error("auth: create sid", slog.Any("error", err))
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString(""),
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Type:    baldaapi.NewOptString("Auth Error"),
			}, nil
		}
		player.Sid = baldaapi.NewOptString(sidStr)
		cfToken, lobbyToken, err := h.generateCentrifugoTokens(u.UID)
		if err != nil {
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString(""),
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Type:    baldaapi.NewOptString("Auth Error"),
			}, nil
		}
		resp := &baldaapi.AuthResponse{
			Player:          baldaapi.NewOptPlayer(player),
			CentrifugoToken: baldaapi.NewOptString(cfToken),
			LobbyToken:      baldaapi.NewOptString(lobbyToken),
		}
		if ag := h.buildActiveGame(u.UID, u.PlayerID.String()); ag != nil {
			resp.ActiveGame = baldaapi.NewOptActiveGame(*ag)
		}
		return resp, nil
	}
	if err != nil {
		slog.Error("auth: get sid", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}

	player.Sid = baldaapi.NewOptString(sid.Sid)
	cfToken, lobbyToken, err := h.generateCentrifugoTokens(u.UID)
	if err != nil {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("Auth Error"),
		}, nil
	}
	resp := &baldaapi.AuthResponse{
		Player:          baldaapi.NewOptPlayer(player),
		CentrifugoToken: baldaapi.NewOptString(cfToken),
		LobbyToken:      baldaapi.NewOptString(lobbyToken),
	}
	if ag := h.buildActiveGame(u.UID, u.PlayerID.String()); ag != nil {
		resp.ActiveGame = baldaapi.NewOptActiveGame(*ag)
	}
	return resp, nil
}

func (h *Handlers) buildActiveGame(uid int64, playerID string) *baldaapi.ActiveGame {
	rec := h.svc.ActiveGameRecord(playerID)
	if rec == nil || rec.Status != lobby.GameStatusInProgress {
		return nil
	}

	gameToken, err := centrifugo.GenerateSubscriptionToken(
		strconv.FormatInt(uid, 10), centrifugo.ChannelGame(rec.ID), h.centrifugoTokenHMACSecret, 24*time.Hour,
	)
	if err != nil {
		slog.Error("auth: generate game token for reconnect", slog.Any("error", err))
		return nil
	}

	gameID, err := uuid.Parse(rec.ID)
	if err != nil {
		return nil
	}

	scores := rec.Game.PlayerScores()
	players := make([]baldaapi.PlayerGameState, 0, len(scores))
	for _, s := range scores {
		pid, err := uuid.Parse(s.UID)
		if err != nil {
			continue
		}
		players = append(players, baldaapi.PlayerGameState{
			UID:        baldaapi.NewOptUUID(pid),
			Exp:        baldaapi.NewOptInt64(int64(s.Exp)),
			Score:      baldaapi.NewOptInt(s.Score),
			WordsCount: baldaapi.NewOptInt(s.WordsCount),
			Words:      s.Words,
		})
	}

	currentTurnUID, _ := uuid.Parse(rec.Game.CurrentPlayerID())

	return &baldaapi.ActiveGame{
		GameID:         baldaapi.NewOptUUID(gameID),
		GameToken:      baldaapi.NewOptString(gameToken),
		Board:          boardToSlice(rec.Game.BoardSnapshot()),
		CurrentTurnUID: baldaapi.NewOptUUID(currentTurnUID),
		MoveNumber:     baldaapi.NewOptInt(rec.Game.MoveNumber()),
		Status:         baldaapi.NewOptGameStatus(baldaapi.GameStatusInProgress),
		Players:        players,
	}
}
