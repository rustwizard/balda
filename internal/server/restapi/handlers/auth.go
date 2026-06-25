package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"strconv"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/auth"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/storage"
)

// Auth implements baldaapi.Handler.
func (h *Handlers) Auth(ctx context.Context, req *baldaapi.AuthRequest) (baldaapi.AuthRes, error) {
	u, err := h.svc.AuthUser(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidCredentials) {
			return &baldaapi.ErrorResponse{
				Message: baldaapi.NewOptString("invalid email or password"),
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("auth: db error", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	access, refresh, err := h.issueTokens(ctx, u.UID, u.PlayerID, u.Role, "", "")
	if err != nil {
		slog.Error("auth: issue tokens", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	cfToken, lobbyToken, err := h.generateCentrifugoTokens(u.UID)
	if err != nil {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	resp := &baldaapi.AuthResponse{
		Player: baldaapi.NewOptPlayer(baldaapi.Player{
			UID:       baldaapi.NewOptUUID(u.PlayerID),
			Firstname: baldaapi.NewOptString(u.Firstname),
			Lastname:  baldaapi.NewOptString(u.Lastname),
			Exp:       baldaapi.NewOptInt64(u.Exp),
			Rating:    baldaapi.NewOptInt64(u.Rating),
		}),
		AccessToken:     baldaapi.NewOptString(access),
		RefreshToken:    baldaapi.NewOptString(refresh),
		TokenType:       baldaapi.NewOptString("Bearer"),
		ExpiresIn:       baldaapi.NewOptInt(int(auth.AccessTokenTTL.Seconds())),
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
			Rating:     baldaapi.NewOptInt64(int64(s.Rating)),
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
