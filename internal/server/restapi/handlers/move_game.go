package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// MoveGame implements baldaapi.Handler.
func (h *Handlers) MoveGame(ctx context.Context, req *baldaapi.MoveRequest, params baldaapi.MoveGameParams) (baldaapi.MoveGameRes, error) {
	uid, err := h.sess.GetUID(params.XAPISession)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return &baldaapi.MoveGameUnauthorized{
				Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
				Message: baldaapi.NewOptString("session not found"),
				Type:    baldaapi.NewOptString("Unauthorized"),
			}, nil
		}
		slog.Error("move_game: get uid", slog.String("sid", params.XAPISession), slog.Any("error", err))
		return &baldaapi.MoveGameUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("session unavailable"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	nl := req.GetNewLetter()
	newLetter := game.Letter{
		RowID: uint8(nl.GetRow()),
		ColID: uint8(nl.GetCol()),
		Char:  nl.GetChar(),
	}

	wordPath := make([]game.Letter, 0, len(req.GetWordPath()))
	for _, cell := range req.GetWordPath() {
		wordPath = append(wordPath, game.Letter{
			RowID: uint8(cell.GetRow()),
			ColID: uint8(cell.GetCol()),
		})
	}

	rec, moverID, err := h.svc.SubmitMove(ctx, uid, params.ID.String(), newLetter, wordPath)
	if err != nil {
		switch {
		case errors.Is(err, lobby.ErrGameNotFound):
			return &baldaapi.MoveGameNotFound{
				Status:  baldaapi.NewOptInt(http.StatusNotFound),
				Message: baldaapi.NewOptString("game not found"),
				Type:    baldaapi.NewOptString("NotFound"),
			}, nil
		case errors.Is(err, game.ErrNotYourTurn), errors.Is(err, game.ErrWrongState):
			return &baldaapi.MoveGameConflict{
				Status:  baldaapi.NewOptInt(http.StatusConflict),
				Message: baldaapi.NewOptString(err.Error()),
				Type:    baldaapi.NewOptString("Conflict"),
			}, nil
		case errors.Is(err, game.ErrWordHasGaps),
			errors.Is(err, game.ErrNewLetterNotInWord),
			errors.Is(err, game.ErrWordAlreadyUsed),
			errors.Is(err, game.ErrWordIsInitialWord),
			errors.Is(err, game.ErrWordNotInDictionary),
			errors.Is(err, game.ErrWrongLetterPlace),
			errors.Is(err, game.ErrLetterPlaceTaken):
			return &baldaapi.MoveGameBadRequest{
				Status:  baldaapi.NewOptInt(http.StatusBadRequest),
				Message: baldaapi.NewOptString(err.Error()),
				Type:    baldaapi.NewOptString("BadRequest"),
			}, nil
		default:
			slog.Error("move_game: submit move", slog.Any("error", err))
			return &baldaapi.MoveGameBadRequest{
				Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
				Message: baldaapi.NewOptString("failed to submit move"),
				Type:    baldaapi.NewOptString("InternalError"),
			}, nil
		}
	}

	// The game's FSM processes the turn advance asynchronously.
	// Compute the next player deterministically so the response matches the
	// eventual server state even if the FSM hasn't advanced yet.
	nextTurnUID := nextPlayerID(moverID, rec.Game.PlayerScores())

	// Publish updated game state to the game channel.
	gameState := buildGameState(rec, nextTurnUID)
	if err := h.cf.Publish(ctx, centrifugo.ChannelGame(rec.ID), gameState); err != nil {
		slog.Error("move_game: publish game state", slog.Any("error", err))
	}

	nextUID, err := uuid.Parse(nextTurnUID)
	if err != nil {
		slog.Error("move_game: parse next turn uid", slog.Any("error", err))
	}

	return &baldaapi.MoveResponse{
		Board:          boardToSlice(rec.Game.Board().AsStrings()),
		CurrentTurnUID: baldaapi.NewOptUUID(nextUID),
		Players:        playerScoresToAPI(rec.Game.PlayerScores()),
		Status:         baldaapi.NewOptGameStatus(baldaapi.GameStatusInProgress),
		MoveNumber:     baldaapi.NewOptInt(rec.Game.MoveNumber()),
	}, nil
}

func boardToSlice(board [5][5]string) [][]string {
	out := make([][]string, len(board))
	for i, row := range board {
		r := make([]string, len(row))
		copy(r, row[:])
		out[i] = r
	}
	return out
}

func playerScoresToAPI(scores []game.PlayerScore) []baldaapi.PlayerScore {
	out := make([]baldaapi.PlayerScore, 0, len(scores))
	for _, ps := range scores {
		pid, err := uuid.Parse(ps.UID)
		if err != nil {
			continue
		}
		out = append(out, baldaapi.PlayerScore{
			UID:        baldaapi.NewOptUUID(pid),
			Score:      baldaapi.NewOptInt(ps.Score),
			WordsCount: baldaapi.NewOptInt(ps.WordsCount),
		})
	}
	return out
}

func nextPlayerID(moverID string, players []game.PlayerScore) string {
	for i, p := range players {
		if p.UID == moverID {
			return players[(i+1)%len(players)].UID
		}
	}
	return ""
}

func buildGameState(rec *lobby.GameRecord, currentTurnUID string) centrifugo.EvGameState {
	players := make([]centrifugo.PlayerScore, 0, len(rec.Players))
	for _, p := range rec.Players {
		players = append(players, centrifugo.PlayerScore{UID: p.ID, Score: p.Score, WordsCount: len(p.Words)})
	}
	if currentTurnUID == "" {
		currentTurnUID = rec.Game.CurrentPlayerID()
	}
	return centrifugo.EvGameState{
		Type:           "game_state",
		GameID:         rec.ID,
		Board:          rec.Game.Board().AsStrings(),
		CurrentTurnUID: currentTurnUID,
		Players:        players,
		Status:         "in_progress",
		MoveNumber:     rec.Game.MoveNumber(),
	}
}
